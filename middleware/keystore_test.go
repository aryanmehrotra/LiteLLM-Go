package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestNewKeyStore(t *testing.T) {
	ks := NewKeyStore()
	if ks == nil {
		t.Fatal("expected non-nil KeyStore")
	}

	if ks.keys == nil {
		t.Fatal("expected non-nil keys map")
	}

	if len(ks.keys) != 0 {
		t.Errorf("expected empty keys map, got %d entries", len(ks.keys))
	}
}

func TestKeyStore_AddAndIsValid(t *testing.T) {
	ks := NewKeyStore()
	rawKey := "sk-test-key-123"
	hash := HashKey(rawKey)

	// Before adding, key should be invalid
	if ks.IsValid(rawKey) {
		t.Error("expected key to be invalid before adding")
	}

	// Add the hash
	ks.Add(hash)

	// Now key should be valid
	if !ks.IsValid(rawKey) {
		t.Error("expected key to be valid after adding its hash")
	}
}

func TestKeyStore_Remove(t *testing.T) {
	ks := NewKeyStore()
	rawKey := "sk-remove-me"
	hash := HashKey(rawKey)

	ks.Add(hash)

	if !ks.IsValid(rawKey) {
		t.Fatal("expected key to be valid after adding")
	}

	ks.Remove(hash)

	if ks.IsValid(rawKey) {
		t.Error("expected key to be invalid after removing")
	}
}

func TestKeyStore_IsValid_WrongKey(t *testing.T) {
	ks := NewKeyStore()
	ks.Add(HashKey("sk-correct-key"))

	if ks.IsValid("sk-wrong-key") {
		t.Error("expected wrong key to be invalid")
	}
}

func TestKeyStore_MultipleKeys(t *testing.T) {
	ks := NewKeyStore()
	keys := []string{"sk-key-a", "sk-key-b", "sk-key-c"}

	for _, k := range keys {
		ks.Add(HashKey(k))
	}

	for _, k := range keys {
		if !ks.IsValid(k) {
			t.Errorf("expected key %q to be valid", k)
		}
	}

	// Remove one and verify others still work
	ks.Remove(HashKey("sk-key-b"))

	if ks.IsValid("sk-key-b") {
		t.Error("expected sk-key-b to be invalid after removal")
	}

	if !ks.IsValid("sk-key-a") {
		t.Error("expected sk-key-a to still be valid")
	}

	if !ks.IsValid("sk-key-c") {
		t.Error("expected sk-key-c to still be valid")
	}
}

func TestKeyStore_RemoveNonExistentKey(t *testing.T) {
	ks := NewKeyStore()

	// Removing a key that was never added should not panic
	ks.Remove(HashKey("sk-never-added"))

	if len(ks.keys) != 0 {
		t.Errorf("expected empty keys map after removing non-existent key, got %d entries", len(ks.keys))
	}
}

func TestHashKey_Consistency(t *testing.T) {
	key := "sk-consistent-test"
	hash1 := HashKey(key)
	hash2 := HashKey(key)

	if hash1 != hash2 {
		t.Errorf("HashKey produced inconsistent results: %q vs %q", hash1, hash2)
	}
}

func TestHashKey_DifferentInputs(t *testing.T) {
	hash1 := HashKey("sk-key-1")
	hash2 := HashKey("sk-key-2")

	if hash1 == hash2 {
		t.Error("expected different hashes for different inputs")
	}
}

func TestHashKey_CorrectSHA256(t *testing.T) {
	key := "sk-verify-hash"
	result := HashKey(key)

	// Manually compute the expected SHA-256
	h := sha256.Sum256([]byte(key))
	expected := hex.EncodeToString(h[:])

	if result != expected {
		t.Errorf("expected hash %q, got %q", expected, result)
	}
}

func TestHashKey_EmptyString(t *testing.T) {
	hash := HashKey("")

	// SHA-256 of empty string is well-known
	expected := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if hash != expected {
		t.Errorf("expected SHA-256 of empty string %q, got %q", expected, hash)
	}
}

func TestIsMissingTableErr(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: true,
		},
		{
			name:     "postgres style",
			err:      &testError{msg: `relation "virtual_keys" does not exist`},
			expected: true,
		},
		{
			name:     "sqlite style",
			err:      &testError{msg: "no such table: virtual_keys"},
			expected: true,
		},
		{
			name:     "unrelated error",
			err:      &testError{msg: "connection refused"},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isMissingTableErr(tc.err)
			if result != tc.expected {
				t.Errorf("isMissingTableErr(%v) = %v, want %v", tc.err, result, tc.expected)
			}
		})
	}
}

// testError is a simple error implementation for testing.
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
