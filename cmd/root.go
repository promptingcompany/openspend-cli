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
	root := &cobra.Command{
		Use:     "openspend",
		Short:   "OpenSpend CLI",
		Version: cliVersion,
	}
	root.SetVersionTemplate("{{printf \"%s\\n\" .Version}}")

	root.PersistentFlags().StringVar(&baseURLOverride, "base-url", "", "Marketplace base URL")

	root.AddCommand(newAuthCmd())
	root.AddCommand(newPolicyCmd())
	root.AddCommand(newAgentCmd())
	root.AddCommand(newOnboardingCmd())
	root.AddCommand(newWhoAmICmd())
	root.AddCommand(newUpdateCmd())
	root.AddCommand(newVersionCmd())

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
		BaseURL:         cfg.Marketplace.BaseURL,
		SessionToken:    cfg.Auth.SessionToken,
		SessionCookie:   cfg.Auth.SessionCookie,
		WhoAmIPath:      cfg.Marketplace.WhoAmIPath,
		PolicyInitPath:  cfg.Marketplace.PolicyInitPath,
		AgentPath:       cfg.Marketplace.AgentPath,
		BrowserAuthPath: cfg.Auth.BrowserLoginPath,
	})
}
