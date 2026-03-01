package handler

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/http/response"

	"aryanmehrotra/llm-gateway/middleware"
)

// AdminPage serves the admin dashboard HTML at GET /admin using GoFr templates.
func (h *AdminHandler) AdminPage() gofr.Handler {
	return func(_ *gofr.Context) (any, error) {
		return response.Template{
			Data: nil,
			Name: "admin.html",
		}, nil
	}
}

// AuthCheck handles GET /auth/check — returns the user's role based on their key.
// If the key matches the master key, role is "admin". Otherwise "user" (virtual key holder).
func (h *AdminHandler) AuthCheck() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		authKey := middleware.GetAuthKey(ctx)

		role := "user"
		if authKey == h.MasterKey {
			role = "admin"
		}

		return response.Raw{Data: map[string]string{
			"role": role,
		}}, nil
	}
}
