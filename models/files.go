package models

// FileObject represents an uploaded file (OpenAI-compatible).
type FileObject struct {
	ID        string `json:"id"`
	Object    string `json:"object"` // "file"
	Bytes     int64  `json:"bytes"`
	CreatedAt int64  `json:"created_at"`
	Filename  string `json:"filename"`
	Purpose   string `json:"purpose"` // "fine-tune", "assistants", "batch", etc.
	Status    string `json:"status,omitempty"`
}

// FileListResponse is the response for GET /v1/files.
type FileListResponse struct {
	Object string       `json:"object"` // "list"
	Data   []FileObject `json:"data"`
}

// FileDeleteResponse is the response for DELETE /v1/files/{id}.
type FileDeleteResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"` // "file"
	Deleted bool   `json:"deleted"`
}
