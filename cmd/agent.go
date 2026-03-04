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
	agentCmd.AddCommand(newAgentUpdateCmd())
	agentCmd.AddCommand(newAgentListCmd())
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
			return runAgentUpsert(cmd, externalKey, displayName, kind, policyID, true, "ready")
		},
	}

	cmd.Flags().StringVar(&externalKey, "external-key", "", "Subject external key (auto-generated if omitted)")
	cmd.Flags().StringVar(&displayName, "display-name", "", "Display name")
	cmd.Flags().StringVar(&kind, "kind", "agent", "Subject kind")
	cmd.Flags().StringVar(&policyID, "policy-id", "", "Optional policy ID override")
	return cmd
}

func newAgentUpdateCmd() *cobra.Command {
	var externalKey string
	var displayName string
	var kind string
	var policyID string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an existing buyer subject and policy binding",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAgentUpsert(cmd, externalKey, displayName, kind, policyID, false, "updated")
		},
	}

	cmd.Flags().StringVar(&externalKey, "external-key", "", "Subject external key")
	cmd.Flags().StringVar(&displayName, "display-name", "", "Display name")
	cmd.Flags().StringVar(&kind, "kind", "agent", "Subject kind")
	cmd.Flags().StringVar(&policyID, "policy-id", "", "Optional policy ID override")
	_ = cmd.MarkFlagRequired("external-key")
	return cmd
}

func runAgentUpsert(
	cmd *cobra.Command,
	externalKey string,
	displayName string,
	kind string,
	policyID string,
	allowGeneratedKey bool,
	outcome string,
) error {
	generatedExternalKey := false
	if strings.TrimSpace(externalKey) == "" {
		if !allowGeneratedKey {
			return fmt.Errorf("--external-key is required")
		}
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
		"Agent subject %s: %s (id=%s external_key=%s generated_external_key=%t), bound policy=%s\n",
		outcome,
		res.Subject.DisplayName,
		res.Subject.ID,
		res.Subject.ExternalKey,
		generatedExternalKey,
		res.PolicyID,
	)
	return nil
}

func newAgentListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List agent subjects for current user",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := mustLoadConfig()
			client := clientFromConfig(cfg)

			res, err := client.WhoAmI(cmd.Context())
			if err != nil {
				return err
			}
			if err := persistAuthFromClient(&cfg, client); err != nil {
				return err
			}

			found := 0
			for _, subject := range res.Subjects {
				if subject.Kind != "agent" && subject.Kind != "anonymous_agent" {
					continue
				}
				displayName := ""
				if subject.DisplayName != nil {
					displayName = strings.TrimSpace(*subject.DisplayName)
				}
				externalKey := ""
				if subject.ExternalKey != nil {
					externalKey = strings.TrimSpace(*subject.ExternalKey)
				}
				policyName := ""
				if subject.PolicyName != nil {
					policyName = strings.TrimSpace(*subject.PolicyName)
				}
				policyID := ""
				if subject.PolicyID != nil {
					policyID = strings.TrimSpace(*subject.PolicyID)
				}
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"- id=%s key=%s name=%s kind=%s status=%s policy=%s policy_id=%s\n",
					subject.ID,
					externalKey,
					displayName,
					subject.Kind,
					subject.Status,
					policyName,
					policyID,
				)
				found++
			}

			if found == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No agents found.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Total agents: %d\n", found)
			return nil
		},
	}
}
