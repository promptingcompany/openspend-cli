package cmd

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/promptingcompany/openspend-cli/internal/config"
)

func TestInferAuthIdentity_RejectsMissingOrZeroExp(t *testing.T) {
	token := makeBearerTokenForTest(t, map[string]any{
		"loginAs":            "agent",
		"subjectExternalKey": "agent-key",
		"subjectDisplayName": "Agent Name",
	})

	identity := inferAuthIdentity(config.AuthTokenBearer, token)
	if identity.LoginAs != config.AuthLoginAsSelf {
		t.Fatalf("expected missing exp token to resolve to self, got %q", identity.LoginAs)
	}
}

func TestInferAuthIdentity_AcceptsValidFutureExp(t *testing.T) {
	token := makeBearerTokenForTest(t, map[string]any{
		"loginAs":            "agent",
		"subjectExternalKey": "agent-key",
		"subjectDisplayName": "Agent Name",
		"exp":                time.Now().Add(15 * time.Minute).Unix(),
	})

	identity := inferAuthIdentity(config.AuthTokenBearer, token)
	if identity.LoginAs != config.AuthLoginAsAgent {
		t.Fatalf("expected future exp token to resolve to agent, got %q", identity.LoginAs)
	}
	if identity.SubjectKey != "agent-key" {
		t.Fatalf("expected subject key to be agent-key, got %q", identity.SubjectKey)
	}
}

func makeBearerTokenForTest(t *testing.T, payload map[string]any) string {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	return "ospcli-v1." + base64.RawURLEncoding.EncodeToString(data) + ".sig"
}
