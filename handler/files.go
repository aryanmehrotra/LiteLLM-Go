package handler

import (
	"database/sql"
	"encoding/base64"
	"io"
	"mime/multipart"
	"strings"
	"time"

	"github.com/google/uuid"
	"gofr.dev/pkg/gofr"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"

	"aryanmehrotra/litellm-go/models"
)

// uploadFileForm is the form struct for multipart file upload.
type uploadFileForm struct {
	Purpose string                `form:"purpose"`
	File    *multipart.FileHeader `file:"file"`
}

// UploadFile handles POST /v1/files — uploads a file and stores it in the database.
func (h *APIHandler) UploadFile() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		var form uploadFileForm
		if err := ctx.Bind(&form); err != nil {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"multipart form"}}
		}

		if form.Purpose == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"purpose"}}
		}

		if form.File == nil {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"file"}}
		}

		f, err := form.File.Open()
		if err != nil {
			return nil, ErrInternal("failed to open uploaded file")
		}
		defer f.Close()

		content, err := io.ReadAll(f)
		if err != nil {
			return nil, ErrInternal("failed to read file content")
		}

		fileID := "file-" + uuid.New().String()
		encoded := base64.StdEncoding.EncodeToString(content)
		now := time.Now().Unix()

		_, err = ctx.SQL.ExecContext(ctx,
			`INSERT INTO gateway_files (id, filename, purpose, bytes, content_b64, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			fileID, form.File.Filename, form.Purpose, int64(len(content)), encoded, now)
		if err != nil {
			ctx.Errorf("upload file: %v", err)
			return nil, ErrInternal("failed to store file")
		}

		return response.Raw{Data: models.FileObject{
			ID:        fileID,
			Object:    "file",
			Bytes:     int64(len(content)),
			CreatedAt: now,
			Filename:  form.File.Filename,
			Purpose:   form.Purpose,
			Status:    "processed",
		}}, nil
	}
}

// ListFiles handles GET /v1/files — lists uploaded files with optional purpose filter.
func (h *APIHandler) ListFiles() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		purpose := ctx.Param("purpose")

		var rows *sql.Rows
		var err error

		if purpose != "" {
			rows, err = ctx.SQL.QueryContext(ctx,
				`SELECT id, filename, purpose, bytes, created_at FROM gateway_files WHERE purpose = $1 ORDER BY created_at DESC`,
				purpose)
		} else {
			rows, err = ctx.SQL.QueryContext(ctx,
				`SELECT id, filename, purpose, bytes, created_at FROM gateway_files ORDER BY created_at DESC`)
		}

		if err != nil {
			ctx.Errorf("list files: %v", err)
			return nil, ErrInternal("failed to list files")
		}
		defer rows.Close()

		var files []models.FileObject

		for rows.Next() {
			var f models.FileObject
			if err := rows.Scan(&f.ID, &f.Filename, &f.Purpose, &f.Bytes, &f.CreatedAt); err != nil {
				continue
			}

			f.Object = "file"
			f.Status = "processed"
			files = append(files, f)
		}

		if err := rows.Err(); err != nil {
			ctx.Errorf("iterate files: %v", err)
			return nil, ErrInternal("failed to list files")
		}

		if files == nil {
			files = []models.FileObject{}
		}

		return response.Raw{Data: models.FileListResponse{
			Object: "list",
			Data:   files,
		}}, nil
	}
}

// GetFile handles GET /v1/files/{id} — returns file metadata.
func (h *APIHandler) GetFile() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		id := ctx.PathParam("id")
		if id == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"id"}}
		}

		var f models.FileObject

		err := ctx.SQL.QueryRowContext(ctx,
			`SELECT id, filename, purpose, bytes, created_at FROM gateway_files WHERE id = $1`, id,
		).Scan(&f.ID, &f.Filename, &f.Purpose, &f.Bytes, &f.CreatedAt)
		if err == sql.ErrNoRows {
			return nil, ErrNotFound("file")
		}

		if err != nil {
			ctx.Errorf("get file: %v", err)
			return nil, ErrInternal("failed to retrieve file")
		}

		f.Object = "file"
		f.Status = "processed"

		return response.Raw{Data: f}, nil
	}
}

// DeleteFile handles DELETE /v1/files/{id} — deletes an uploaded file.
func (h *APIHandler) DeleteFile() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		id := ctx.PathParam("id")
		if id == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"id"}}
		}

		result, err := ctx.SQL.ExecContext(ctx, `DELETE FROM gateway_files WHERE id = $1`, id)
		if err != nil {
			ctx.Errorf("delete file: %v", err)
			return nil, ErrInternal("failed to delete file")
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			return nil, ErrNotFound("file")
		}

		return response.Raw{Data: models.FileDeleteResponse{
			ID:      id,
			Object:  "file",
			Deleted: true,
		}}, nil
	}
}

// GetFileContent handles GET /v1/files/{id}/content — downloads file content.
func (h *APIHandler) GetFileContent() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		id := ctx.PathParam("id")
		if id == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"id"}}
		}

		var encoded string
		var filename string
		var sizeBytes int64

		err := ctx.SQL.QueryRowContext(ctx,
			`SELECT content_b64, filename, bytes FROM gateway_files WHERE id = $1`, id,
		).Scan(&encoded, &filename, &sizeBytes)
		if err == sql.ErrNoRows {
			return nil, ErrNotFound("file")
		}

		if err != nil {
			ctx.Errorf("get file content: %v", err)
			return nil, ErrInternal("failed to retrieve file content")
		}

		content, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, ErrInternal("failed to decode file content")
		}

		// Determine content type from filename extension
		contentType := "application/octet-stream"
		switch {
		case strings.HasSuffix(filename, ".json"):
			contentType = "application/json"
		case strings.HasSuffix(filename, ".jsonl"):
			contentType = "text/plain"
		case strings.HasSuffix(filename, ".txt"):
			contentType = "text/plain"
		case strings.HasSuffix(filename, ".csv"):
			contentType = "text/csv"
		}

		return response.File{
			Content:     content,
			ContentType: contentType,
		}, nil
	}
}

