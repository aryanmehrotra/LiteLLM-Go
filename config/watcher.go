package config

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Watcher polls a config file for changes and calls onReload when changes are detected.
// Also listens for SIGHUP to trigger reloads.
type Watcher struct {
	path     string
	interval time.Duration
	lastHash [32]byte
	onReload func(*GatewayConfig) error
}

// NewWatcher creates a config file watcher.
func NewWatcher(path string, interval time.Duration, onReload func(*GatewayConfig) error) *Watcher {
	return &Watcher{
		path:     path,
		interval: interval,
		onReload: onReload,
	}
}

// Start begins watching for config changes in a goroutine.
func (w *Watcher) Start() {
	// Set initial hash
	w.lastHash = w.fileHash()

	// Listen for SIGHUP
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)

	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				w.checkAndReload()
			case <-sighup:
				w.checkAndReload()
			}
		}
	}()
}

func (w *Watcher) checkAndReload() {
	hash := w.fileHash()
	if hash == w.lastHash {
		return
	}

	w.lastHash = hash

	cfg, err := Load(w.path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config reload error: %v\n", err)
		return
	}

	if err := w.onReload(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "config apply error: %v\n", err)
	}
}

func (w *Watcher) fileHash() [32]byte {
	data, err := os.ReadFile(w.path)
	if err != nil {
		return [32]byte{}
	}

	return sha256.Sum256(data)
}
