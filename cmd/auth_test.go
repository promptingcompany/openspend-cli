package cmd

import (
	"testing"
	"time"

	"github.com/promptingcompany/openspend-cli/internal/config"
)

func TestClearAuthSession(t *testing.T) {
	t.Run("clears active bearer session", func(t *testing.T) {
		cfg := config.Config{
			Auth: config.AuthConfig{
				SessionToken:     "token-123",
				AuthTokenType:    config.AuthTokenBearer,
				SessionExpiresAt: time.Now().Add(30 * time.Minute).UTC(),
			},
		}

		changed := clearAuthSession(&cfg)
		if !changed {
			t.Fatalf("expected changes when clearing active session")
		}
		if cfg.Auth.SessionToken != "" {
			t.Fatalf("expected session token to be cleared")
		}
		if !cfg.Auth.SessionExpiresAt.IsZero() {
			t.Fatalf("expected session expiry to be cleared")
		}
		if cfg.Auth.AuthTokenType != config.AuthTokenCookie {
			t.Fatalf("expected auth token type to reset to cookie, got %q", cfg.Auth.AuthTokenType)
		}
	})

	t.Run("already logged out is no-op", func(t *testing.T) {
		cfg := config.Config{
			Auth: config.AuthConfig{
				AuthTokenType: config.AuthTokenCookie,
			},
		}

		changed := clearAuthSession(&cfg)
		if changed {
			t.Fatalf("expected no changes for logged-out config")
		}
	})
}
