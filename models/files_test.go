package models

import (
	"encoding/json"
	"testing"
)

func TestFileObject_JSON(t *testing.T) {
	f := FileObject{
		ID:        "file-abc123",
		Object:    "file",
		Bytes:     1024,
		CreatedAt: 1700000000,
		Filename:  "training.jsonl",
		Purpose:   "fine-tune",
		Status:    "processed",
	}

	b, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal file object: %v", err)
	}

	var got FileObject
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal file object: %v", err)
	}

	if got.ID != f.ID {
		t.Errorf("ID mismatch: want %q, got %q", f.ID, got.ID)
	}

	if got.Bytes != f.Bytes {
		t.Errorf("Bytes mismatch: want %d, got %d", f.Bytes, got.Bytes)
	}

	if got.Purpose != f.Purpose {
		t.Errorf("Purpose mismatch: want %q, got %q", f.Purpose, got.Purpose)
	}
}

func TestFileListResponse_JSON(t *testing.T) {
	resp := FileListResponse{
		Object: "list",
		Data: []FileObject{
			{ID: "file-1", Object: "file", Bytes: 100, Filename: "a.txt", Purpose: "assistants", CreatedAt: 1700000000},
			{ID: "file-2", Object: "file", Bytes: 200, Filename: "b.jsonl", Purpose: "fine-tune", CreatedAt: 1700000001},
		},
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got FileListResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Object != "list" {
		t.Errorf("Object: want %q, got %q", "list", got.Object)
	}

	if len(got.Data) != 2 {
		t.Errorf("len(Data): want 2, got %d", len(got.Data))
	}
}

func TestFileDeleteResponse_JSON(t *testing.T) {
	resp := FileDeleteResponse{
		ID:      "file-abc",
		Object:  "file",
		Deleted: true,
	}

	b, _ := json.Marshal(resp)

	var got FileDeleteResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !got.Deleted {
		t.Error("expected Deleted to be true")
	}
}
