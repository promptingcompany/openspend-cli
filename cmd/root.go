package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/promptingcompany/openspend-cli/internal/api"
	"github.com/promptingcompany/openspend-cli/internal/config"
	"github.com/spf13/cobra"
)

var baseURLOverride string
var cliVersion = "dev"

func SetVersion(version string) {
	if strings.TrimSpace(version) == "" {
		return
	}
	cliVersion = version
}

func NewRootCmd() *cobra.Command {
	role := detectConfiguredRole()

	root := &cobra.Command{
		Use:     "openspend",
		Short:   "OpenSpend CLI",
		Version: cliVersion,
	}
	root.SetVersionTemplate("{{printf \"%s\\n\" .Version}}")
	root.CompletionOptions.DisableDefaultCmd = true

	root.PersistentFlags().StringVar(&baseURLOverride, "base-url", "", "Marketplace base URL")

	root.AddCommand(newAuthCmd())
	root.AddCommand(newSearchCmd())
	root.AddCommand(newWhoAmICmd())
	root.AddCommand(newUpdateCmd())
	root.AddCommand(newVersionCmd())
	if role != config.AuthLoginAsAgent {
		root.AddCommand(newDashboardCmd())
	}

	return root
}

func executeWithContext() error {
	return NewRootCmd().ExecuteContext(context.Background())
}

func mustLoadConfig() config.Config {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}
	config.ApplyEnvOverrides(&cfg)
	if baseURLOverride != "" {
		cfg.Marketplace.BaseURL = baseURLOverride
	}
	return cfg
}

func clientFromConfig(cfg config.Config) *api.Client {
	return api.New(api.Options{
		BaseURL:             cfg.Marketplace.BaseURL,
		SessionToken:        cfg.Auth.SessionToken,
		AuthTokenType:       cfg.Auth.AuthTokenType,
		SessionCookie:       cfg.Auth.SessionCookie,
		SessionExpiresAt:    cfg.Auth.SessionExpiresAt,
		WhoAmIPath:          cfg.Marketplace.WhoAmIPath,
		PolicyInitPath:      cfg.Marketplace.PolicyInitPath,
		PolicyDetailsPath:   cfg.Marketplace.PolicyDetailsPath,
		AgentPath:           cfg.Marketplace.AgentPath,
		SearchPath:          cfg.Marketplace.SearchPath,
		BrowserAuthPath:     cfg.Auth.BrowserLoginPath,
		CliAuthExchangePath: cfg.Auth.CliAuthExchangePath,
		SessionRefreshPath:  cfg.Auth.SessionRefreshPath,
	})
}

func persistAuthFromClient(cfg *config.Config, client *api.Client) error {
	if cfg == nil || client == nil {
		return nil
	}

	updated := false
	if cfg.Auth.SessionToken != client.SessionToken() {
		cfg.Auth.SessionToken = client.SessionToken()
		updated = true
	}
	if cfg.Auth.SessionCookie != client.SessionCookie() {
		cfg.Auth.SessionCookie = client.SessionCookie()
		updated = true
	}
	if !cfg.Auth.SessionExpiresAt.Equal(client.SessionExpiresAt()) {
		cfg.Auth.SessionExpiresAt = client.SessionExpiresAt()
		updated = true
	}
	if cfg.Auth.AuthTokenType != client.AuthTokenType() {
		cfg.Auth.AuthTokenType = client.AuthTokenType()
		updated = true
	}
	if !updated {
		return nil
	}
	return config.Save(*cfg)
}

func detectConfiguredRole() string {
	cfg, err := config.Load()
	if err != nil {
		return config.AuthLoginAsSelf
	}
	config.ApplyEnvOverrides(&cfg)
	identity := inferAuthIdentity(cfg.Auth.AuthTokenType, cfg.Auth.SessionToken)
	if identity.LoginAs == config.AuthLoginAsAgent {
		return config.AuthLoginAsAgent
	}
	return config.AuthLoginAsSelf
}
