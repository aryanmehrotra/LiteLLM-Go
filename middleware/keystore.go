package middleware

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"sync"

	"gofr.dev/pkg/gofr"
)

// KeyStore is an in-memory cache of active virtual key hashes.
// It uses a write-through pattern: updates go to DB first, then the map.
// On startup, LoadFromDB populates the map from the database.
type KeyStore struct {
	mu   sync.RWMutex
	keys map[string]bool // key_hash → active
}

// NewKeyStore creates an empty KeyStore.
func NewKeyStore() *KeyStore {
	return &KeyStore{keys: make(map[string]bool)}
}

// LoadFromDB loads all active virtual key hashes from the database.
// Call this at startup to warm the in-memory map.
func (ks *KeyStore) LoadFromDB(ctx *gofr.Context) error {
	rows, err := ctx.SQL.QueryContext(ctx, "SELECT key_hash FROM virtual_keys WHERE is_active = TRUE")
	if err != nil {
		// Table may not exist yet (migrations haven't run), that's fine
		if isMissingTableErr(err) {
			return nil
		}

		return err
	}
	defer rows.Close()

	ks.mu.Lock()
	defer ks.mu.Unlock()

	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			continue
		}

		ks.keys[hash] = true
	}

	return nil
}

// Add adds a key hash to the in-memory map.
func (ks *KeyStore) Add(keyHash string) {
	ks.mu.Lock()
	ks.keys[keyHash] = true
	ks.mu.Unlock()
}

// Remove removes a key hash from the in-memory map.
func (ks *KeyStore) Remove(keyHash string) {
	ks.mu.Lock()
	delete(ks.keys, keyHash)
	ks.mu.Unlock()
}

// IsValid checks if a raw API key is valid by hashing and looking up in memory.
func (ks *KeyStore) IsValid(rawKey string) bool {
	hash := HashKey(rawKey)

	ks.mu.RLock()
	defer ks.mu.RUnlock()

	return ks.keys[hash]
}

// HashKey returns the hex-encoded SHA-256 hash of a key.
func HashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// isMissingTableErr does a simple check for common "table does not exist" errors.
func isMissingTableErr(err error) bool {
	if err == nil || err == sql.ErrNoRows {
		return true
	}

	msg := err.Error()
	// PostgreSQL: "relation ... does not exist"
	// SQLite: "no such table"
	return contains(msg, "does not exist") || contains(msg, "no such table")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
