package cmd

import (
	"bytes"
	"testing"
	"time"

	"github.com/promptingcompany/openspend-cli/internal/api"
	"github.com/promptingcompany/openspend-cli/internal/config"
)

func TestExtractCloudflareTunnelURL(t *testing.T) {
	t.Run("extracts trycloudflare url from log line", func(t *testing.T) {
		line := "INF | https://example-subdomain.trycloudflare.com registered"
		got := extractCloudflareTunnelURL(line)
		want := "https://example-subdomain.trycloudflare.com"
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})

	t.Run("returns empty when no tunnel url present", func(t *testing.T) {
		if got := extractCloudflareTunnelURL("cloudflared starting"); got != "" {
			t.Fatalf("expected empty result, got %q", got)
		}
	})
}

func TestCloudflaredInstallHint(t *testing.T) {
	if got := cloudflaredInstallHint("darwin"); got != "Install cloudflared with: brew install cloudflared" {
		t.Fatalf("unexpected darwin hint: %q", got)
	}
	if got := cloudflaredInstallHint("windows"); got != "Install cloudflared with: winget install --id Cloudflare.cloudflared" {
		t.Fatalf("unexpected windows hint: %q", got)
	}
	if got := cloudflaredInstallHint("linux"); got == "" {
		t.Fatalf("expected non-empty linux hint")
	}
}

func TestPrintRedirectHostCompatibilityWarning(t *testing.T) {
	t.Run("no warning for localhost callback", func(t *testing.T) {
		var out bytes.Buffer
		printRedirectHostCompatibilityWarning(
			&out,
			"https://openspend.ai",
			"http://127.0.0.1:4321/callback",
		)
		if out.Len() != 0 {
			t.Fatalf("expected no warning for localhost callback, got: %s", out.String())
		}
	})

	t.Run("warns for different callback host", func(t *testing.T) {
		var out bytes.Buffer
		printRedirectHostCompatibilityWarning(
			&out,
			"https://openspend.ai",
			"https://abc.trycloudflare.com/callback",
		)
		got := out.String()
		if got == "" {
			t.Fatalf("expected warning output, got empty")
		}
		if !bytes.Contains([]byte(got), []byte("redirect policy")) {
			t.Fatalf("expected redirect policy warning, got: %s", got)
		}
	})
}

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

func TestResolveLoginIdentityChoice_NoAgentsDefaultsToSelf(t *testing.T) {
	choice, err := resolveLoginIdentityChoice(nil, api.WhoAmIResponse{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if choice.loginAs != config.AuthLoginAsSelf {
		t.Fatalf("expected loginAs=%q, got %q", config.AuthLoginAsSelf, choice.loginAs)
	}
	if choice.subjectKey != "" {
		t.Fatalf("expected empty subject key, got %q", choice.subjectKey)
	}
}

func TestApplyExchangedAuthConfig_ResetsStaleExpiry(t *testing.T) {
	stale := time.Now().Add(2 * time.Hour).UTC()
	cfg := config.Config{
		Auth: config.AuthConfig{
			SessionToken:     "old-token",
			AuthTokenType:    config.AuthTokenCookie,
			SessionExpiresAt: stale,
		},
	}

	applyExchangedAuthConfig(&cfg, api.ExchangeCliAuthResponse{CliToken: "new-token"})
	if cfg.Auth.SessionToken != "new-token" {
		t.Fatalf("expected session token to be replaced")
	}
	if cfg.Auth.AuthTokenType != config.AuthTokenBearer {
		t.Fatalf("expected auth token type to be bearer")
	}
	if !cfg.Auth.SessionExpiresAt.IsZero() {
		t.Fatalf("expected stale expiry to be cleared when exchange has no expiresAt")
	}
}
