package routing_test

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"aryanmehrotra/litellm-go/models"
	"aryanmehrotra/litellm-go/routing"
	"aryanmehrotra/litellm-go/testutil"
)

// buildRouter creates a test Router with zero retries and a disabled cooldown.
func buildRouter() *routing.Router {
	return routing.NewRouter(
		routing.DefaultRetryPolicy(0, 0),
		routing.NewCooldownTracker(999, time.Millisecond),
		&routing.SimpleStrategy{},
	)
}

// --------------------------------------------------------------------------
// Router.ChatCompletion integration tests with MockProvider
// --------------------------------------------------------------------------

func TestRouter_ChatCompletion_Success(t *testing.T) {
	mock := testutil.NewMockProvider("test-provider",
		testutil.TextResponse("hello from mock"),
	)
	router := buildRouter()
	ctx := testutil.NewGofrCtx()

	req := models.ChatCompletionRequest{
		Model: "test-model",
		Messages: []models.Message{
			{Role: "user", Content: "Say hello"},
		},
	}

	resp, err := router.ChatCompletion(ctx, mock, "test-model", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected non-nil response")
	}

	if len(resp.Choices) == 0 {
		t.Fatal("expected at least 1 choice")
	}

	if resp.Choices[0].Message.Content != "hello from mock" {
		t.Errorf("content: want 'hello from mock', got %q", resp.Choices[0].Message.Content)
	}
}

func TestRouter_ChatCompletion_ProviderError_NoRetry(t *testing.T) {
	responses := []*models.ChatCompletionResponse{nil}
	errs := []error{errors.New("provider failed")}

	mock := testutil.NewMockProviderWithErrors("test-provider", responses, errs)
	router := buildRouter() // 0 retries
	ctx := testutil.NewGofrCtx()

	req := models.ChatCompletionRequest{
		Model: "test-model",
		Messages: []models.Message{{Role: "user", Content: "hi"}},
	}

	_, err := router.ChatCompletion(ctx, mock, "test-model", req)
	if err == nil {
		t.Error("expected error, got nil")
	}

	if mock.CallCount() != 1 {
		t.Errorf("CallCount: want 1 (no retries), got %d", mock.CallCount())
	}
}

func TestRouter_ChatCompletion_WithRetries(t *testing.T) {
	// Provider fails twice then succeeds — retry policy allows 3 retries.
	responses := []*models.ChatCompletionResponse{
		nil,
		nil,
		testutil.TextResponse("finally worked"),
	}
	errs := []error{
		&routing.ProviderError{StatusCode: 500, Body: "internal error", Err: errors.New("500")},
		&routing.ProviderError{StatusCode: 500, Body: "internal error", Err: errors.New("500")},
		nil,
	}

	mock := testutil.NewMockProviderWithErrors("test-provider", responses, errs)

	// 3 retries, very short delay
	router := routing.NewRouter(
		routing.DefaultRetryPolicy(3, time.Millisecond),
		routing.NewCooldownTracker(999, time.Millisecond),
		&routing.SimpleStrategy{},
	)
	ctx := testutil.NewGofrCtx()

	req := models.ChatCompletionRequest{
		Model: "test-model",
		Messages: []models.Message{{Role: "user", Content: "try again"}},
	}

	resp, err := router.ChatCompletion(ctx, mock, "test-model", req)
	if err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}

	if resp.Choices[0].Message.Content != "finally worked" {
		t.Errorf("content: want 'finally worked', got %q", resp.Choices[0].Message.Content)
	}

	// Should have been called 3 times (2 failures + 1 success)
	if mock.CallCount() != 3 {
		t.Errorf("CallCount: want 3, got %d", mock.CallCount())
	}
}

func TestRouter_ChatCompletion_PreservesModelName(t *testing.T) {
	mock := testutil.NewMockProvider("my-llm",
		testutil.TextResponse("response"),
	)
	router := buildRouter()
	ctx := testutil.NewGofrCtx()

	req := models.ChatCompletionRequest{
		Model: "old-model-name",
		Messages: []models.Message{{Role: "user", Content: "hi"}},
	}

	_, err := router.ChatCompletion(ctx, mock, "clean-model-name", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The router should have overridden req.Model with the clean modelName
	call0 := mock.CallAt(0)
	if call0.Model != "clean-model-name" {
		t.Errorf("req.Model in provider call: want 'clean-model-name', got %q", call0.Model)
	}
}

func TestRouter_ChatCompletion_CooldownBlocksProvider(t *testing.T) {
	mock := testutil.NewMockProvider("blocked-provider",
		testutil.TextResponse("should not reach here"),
	)

	// Very low threshold + long cooldown: 1 failure triggers instant cooldown
	cooldown := routing.NewCooldownTracker(1, 10*time.Second)

	// Manually trigger a failure to put provider in cooldown
	cooldown.RecordFailure("blocked-provider")

	router := routing.NewRouter(
		routing.DefaultRetryPolicy(0, 0),
		cooldown,
		&routing.SimpleStrategy{},
	)
	ctx := testutil.NewGofrCtx()

	req := models.ChatCompletionRequest{
		Model: "test-model",
		Messages: []models.Message{{Role: "user", Content: "hi"}},
	}

	_, err := router.ChatCompletion(ctx, mock, "test-model", req)
	if err == nil {
		t.Error("expected cooldown error, got nil")
	}

	// Provider should not have been called
	if mock.CallCount() != 0 {
		t.Errorf("CallCount: want 0 (blocked by cooldown), got %d", mock.CallCount())
	}
}

func TestRouter_ChatCompletion_MultipleSequentialCalls(t *testing.T) {
	mock := testutil.NewMockProvider("test-provider",
		testutil.TextResponse("response-1"),
		testutil.TextResponse("response-2"),
		testutil.TextResponse("response-3"),
	)
	router := buildRouter()
	ctx := testutil.NewGofrCtx()

	for i := 1; i <= 3; i++ {
		req := models.ChatCompletionRequest{
			Model: "test-model",
			Messages: []models.Message{
				{Role: "user", Content: "message " + strconv.Itoa(i)},
			},
		}

		resp, err := router.ChatCompletion(ctx, mock, "test-model", req)
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}

		want := "response-" + strconv.Itoa(i)
		if resp.Choices[0].Message.Content != want {
			t.Errorf("call %d content: want %q, got %q", i, want, resp.Choices[0].Message.Content)
		}
	}

	if mock.CallCount() != 3 {
		t.Errorf("CallCount: want 3, got %d", mock.CallCount())
	}
}

func TestRouter_ChatCompletion_UsageTracking(t *testing.T) {
	mock := testutil.NewMockProvider("test-provider",
		testutil.TextResponse("test"),
	)
	usageTracker := routing.NewUsageTracker(time.Minute)

	router := routing.NewRouter(
		routing.DefaultRetryPolicy(0, 0),
		routing.NewCooldownTracker(999, time.Millisecond),
		&routing.SimpleStrategy{},
	)
	router.Usage = usageTracker

	ctx := testutil.NewGofrCtx()

	req := models.ChatCompletionRequest{
		Model:    "test-model",
		Messages: []models.Message{{Role: "user", Content: "hi"}},
	}

	resp, err := router.ChatCompletion(ctx, mock, "test-model", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// textResponse has TotalTokens=30
	_ = resp

	total := usageTracker.Usage("test-provider")
	if total != 30 {
		t.Errorf("UsageTracker.TotalTokens: want 30, got %d", total)
	}
}
