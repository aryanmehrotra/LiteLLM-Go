package handler

import (
	"gofr.dev/pkg/gofr"

	"examples/llm-gateway/middleware"
)

// AdminHandler groups all management endpoint handlers with their shared dependencies.
type AdminHandler struct {
	MasterKey string
	KeyStore  *middleware.KeyStore
}

// requireMasterKey checks that the authenticated key matches the master key.
func (h *AdminHandler) requireMasterKey(ctx *gofr.Context) error {
	if h.MasterKey == "" {
		return ErrUnauthorized("master key not configured")
	}

	authKey := middleware.GetAuthKey(ctx)
	if authKey != h.MasterKey {
		return ErrUnauthorized("master key required")
	}

	return nil
}
