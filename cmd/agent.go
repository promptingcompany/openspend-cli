package cmd

import (
	"fmt"
	"strings"
	"time"

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
			generatedExternalKey := false
			if strings.TrimSpace(externalKey) == "" {
				externalKey = fmt.Sprintf("buyer-agent-%d", time.Now().Unix())
				generatedExternalKey = true
				fmt.Fprintf(cmd.OutOrStdout(), "No --external-key provided; using generated key: %s\n", externalKey)
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
			if err := persistAuthFromClient(&cfg, client); err != nil {
				return err
			}

			fmt.Fprintf(
				cmd.OutOrStdout(),
				"Agent subject ready: %s (id=%s external_key=%s generated_external_key=%t), bound policy=%s\n",
				res.Subject.DisplayName,
				res.Subject.ID,
				res.Subject.ExternalKey,
				generatedExternalKey,
				res.PolicyID,
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&externalKey, "external-key", "", "Subject external key (auto-generated if omitted)")
	cmd.Flags().StringVar(&displayName, "display-name", "", "Display name")
	cmd.Flags().StringVar(&kind, "kind", "agent", "Subject kind")
	cmd.Flags().StringVar(&policyID, "policy-id", "", "Optional policy ID override")
	return cmd
}
