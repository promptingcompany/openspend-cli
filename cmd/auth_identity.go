package cmd

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/promptingcompany/openspend-cli/internal/config"
)

type authIdentity struct {
	LoginAs     string
	SubjectKey  string
	SubjectName string
}

type cliTokenClaims struct {
	LoginAs            string  `json:"loginAs"`
	SubjectExternalKey *string `json:"subjectExternalKey"`
	SubjectDisplayName *string `json:"subjectDisplayName"`
	Exp                int64   `json:"exp"`
}

func inferAuthIdentity(authTokenType, token string) authIdentity {
	defaultIdentity := authIdentity{LoginAs: config.AuthLoginAsSelf}
	if strings.TrimSpace(authTokenType) != config.AuthTokenBearer {
		return defaultIdentity
	}

	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 || parts[0] != "ospcli-v1" {
		return defaultIdentity
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return defaultIdentity
	}

	var claims cliTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return defaultIdentity
	}
	if claims.Exp <= 0 {
		return defaultIdentity
	}
	if !time.Now().Before(time.Unix(claims.Exp, 0)) {
		return defaultIdentity
	}
	if claims.LoginAs != config.AuthLoginAsAgent {
		return defaultIdentity
	}

	identity := authIdentity{LoginAs: config.AuthLoginAsAgent}
	if claims.SubjectExternalKey != nil {
		identity.SubjectKey = strings.TrimSpace(*claims.SubjectExternalKey)
	}
	if claims.SubjectDisplayName != nil {
		identity.SubjectName = strings.TrimSpace(*claims.SubjectDisplayName)
	}
	return identity
}
