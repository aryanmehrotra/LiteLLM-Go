package config

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewWatcher(t *testing.T) {
	called := false
	w := NewWatcher("/tmp/test.yaml", 1*time.Second, func(cfg *GatewayConfig) error {
		called = true
		return nil
	})

	if w == nil {
		t.Fatal("expected non-nil watcher")
	}

	if w.path != "/tmp/test.yaml" {
		t.Errorf("expected path '/tmp/test.yaml', got %q", w.path)
	}

	if w.interval != 1*time.Second {
		t.Errorf("expected interval 1s, got %v", w.interval)
	}

	// callback not called yet
	if called {
		t.Error("expected callback not to be called during construction")
	}
}

func TestWatcher_fileHash_NonExistent(t *testing.T) {
	w := NewWatcher("/nonexistent/file.yaml", 1*time.Second, nil)

	hash := w.fileHash()
	zero := [32]byte{}

	if hash != zero {
		t.Error("expected zero hash for nonexistent file")
	}
}

func TestWatcher_fileHash_Deterministic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	if err := os.WriteFile(path, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	w := NewWatcher(path, 1*time.Second, nil)

	h1 := w.fileHash()
	h2 := w.fileHash()

	if h1 != h2 {
		t.Error("expected deterministic hash")
	}

	// Verify the hash matches expected SHA-256
	expected := sha256.Sum256([]byte("test content"))
	if h1 != expected {
		t.Error("hash doesn't match expected SHA-256")
	}
}

func TestWatcher_fileHash_ChangesOnModification(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	if err := os.WriteFile(path, []byte("content v1"), 0644); err != nil {
		t.Fatal(err)
	}

	w := NewWatcher(path, 1*time.Second, nil)
	h1 := w.fileHash()

	if err := os.WriteFile(path, []byte("content v2"), 0644); err != nil {
		t.Fatal(err)
	}

	h2 := w.fileHash()

	if h1 == h2 {
		t.Error("expected different hashes after file modification")
	}
}

func TestWatcher_checkAndReload_NoChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `
model_list:
  - model_name: test
    litellm_params:
      model: openai/gpt-4o
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	called := false
	w := NewWatcher(path, 1*time.Second, func(cfg *GatewayConfig) error {
		called = true
		return nil
	})

	// Set the initial hash
	w.lastHash = w.fileHash()

	// Check without modification — should NOT call the callback
	w.checkAndReload()

	if called {
		t.Error("callback should not be called when file hasn't changed")
	}
}

func TestWatcher_checkAndReload_WithChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `
model_list:
  - model_name: test
    litellm_params:
      model: openai/gpt-4o
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	called := false
	var reloadedConfig *GatewayConfig

	w := NewWatcher(path, 1*time.Second, func(cfg *GatewayConfig) error {
		called = true
		reloadedConfig = cfg
		return nil
	})

	// Set initial hash
	w.lastHash = w.fileHash()

	// Modify the file
	newContent := `
model_list:
  - model_name: updated-model
    litellm_params:
      model: anthropic/claude-sonnet-4-20250514
`
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Now check — should detect the change and call the callback
	w.checkAndReload()

	if !called {
		t.Error("callback should be called when file changes")
	}

	if reloadedConfig == nil {
		t.Fatal("expected non-nil reloaded config")
	}

	if reloadedConfig.ModelList[0].ModelName != "updated-model" {
		t.Errorf("expected updated model name, got %q", reloadedConfig.ModelList[0].ModelName)
	}
}
