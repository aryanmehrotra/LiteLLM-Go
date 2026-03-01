package models

import (
	"encoding/json"
	"testing"
)

func TestFineTuningJob_JSON(t *testing.T) {
	job := FineTuningJob{
		ID:           "ftjob-abc123",
		Object:       "fine_tuning.job",
		CreatedAt:    1700000000,
		Model:        "gpt-4o-mini",
		Status:       "queued",
		TrainingFile: "file-train123",
	}

	b, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got FineTuningJob
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != job.ID {
		t.Errorf("ID: want %q, got %q", job.ID, got.ID)
	}

	if got.Status != job.Status {
		t.Errorf("Status: want %q, got %q", job.Status, got.Status)
	}
}

func TestFineTuningJob_OptionalFields(t *testing.T) {
	finishedAt := int64(1700001000)
	fineTunedModel := "ft:gpt-4o-mini:org:suffix:id"
	suffix := "my-suffix"

	job := FineTuningJob{
		ID:             "ftjob-1",
		Object:         "fine_tuning.job",
		CreatedAt:      1700000000,
		FinishedAt:     &finishedAt,
		Model:          "gpt-4o-mini",
		FineTunedModel: &fineTunedModel,
		Status:         "succeeded",
		TrainingFile:   "file-1",
		Suffix:         &suffix,
		Hyperparameters: &Hyperparameters{
			NEpochs: "auto",
		},
	}

	b, _ := json.Marshal(job)

	var got FineTuningJob
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.FinishedAt == nil || *got.FinishedAt != finishedAt {
		t.Error("FinishedAt not preserved")
	}

	if got.FineTunedModel == nil || *got.FineTunedModel != fineTunedModel {
		t.Error("FineTunedModel not preserved")
	}
}

func TestFineTuningJobListResponse(t *testing.T) {
	resp := FineTuningJobListResponse{
		Object:  "list",
		Data:    []FineTuningJob{{ID: "ftjob-1", Object: "fine_tuning.job"}},
		HasMore: false,
	}

	b, _ := json.Marshal(resp)

	var got FineTuningJobListResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(got.Data) != 1 {
		t.Errorf("len(Data): want 1, got %d", len(got.Data))
	}
}

func TestFineTuningEvent_JSON(t *testing.T) {
	event := FineTuningEvent{
		ID:        "event-1",
		Object:    "fine_tuning.job.event",
		CreatedAt: 1700000000,
		Level:     "info",
		Message:   "Step 1 completed",
		Type:      "message",
	}

	b, _ := json.Marshal(event)

	var got FineTuningEvent
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Message != event.Message {
		t.Errorf("Message: want %q, got %q", event.Message, got.Message)
	}
}
