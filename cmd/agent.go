package cmd

import (
	"fmt"
	"strings"

	"github.com/promptingcompany/openspend-cli/internal/api"
	"github.com/spf13/cobra"
)

func newAgentCmd() *cobra.Command {
	agentCmd := &cobra.Command{
		Use:   "agent",
		Short: "Agent and subject commands",
	}
	agentCmd.AddCommand(newAgentCreateCmd())
	return agentCmd
}

func newAgentCreateCmd() *cobra.Command {
	var externalKey string
	var displayName string
	var kind string
	var policyID string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a buyer subject and bind to policy",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(externalKey) == "" {
				return fmt.Errorf("--external-key is required")
			}
			cfg := mustLoadConfig()
			client := clientFromConfig(cfg)

			res, err := client.CreateAgent(cmd.Context(), api.CreateAgentRequest{
				ExternalKey: externalKey,
				DisplayName: displayName,
				Kind:        kind,
				PolicyID:    policyID,
			})
			if err != nil {
				return err
			}

			fmt.Fprintf(
				cmd.OutOrStdout(),
				"Agent subject ready: %s (%s), bound policy=%s\n",
				res.Subject.DisplayName,
				res.Subject.ID,
				res.PolicyID,
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&externalKey, "external-key", "", "Subject external key")
	cmd.Flags().StringVar(&displayName, "display-name", "", "Display name")
	cmd.Flags().StringVar(&kind, "kind", "agent", "Subject kind")
	cmd.Flags().StringVar(&policyID, "policy-id", "", "Optional policy ID override")
	return cmd
}
