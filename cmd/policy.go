package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/promptingcompany/openspend-cli/internal/api"
	"github.com/spf13/cobra"
)

func newPolicyCmd() *cobra.Command {
	policyCmd := &cobra.Command{
		Use:   "policy",
		Short: "Policy management",
	}
	policyCmd.AddCommand(newPolicyInitCmd())
	policyCmd.AddCommand(newPolicyListCmd())
	policyCmd.AddCommand(newPolicyDescribeCmd())
	return policyCmd
}

func newPolicyInitCmd() *cobra.Command {
	var buyer bool
	var name string
	var description string
	var asset string
	var network string
	var denyHosts string
	var maxPrice int64

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a base policy",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !buyer {
				return fmt.Errorf("only buyer initialization is supported for now; pass --buyer")
			}
			cfg := mustLoadConfig()
			client := clientFromConfig(cfg)

			var maxPricePtr *int64
			if maxPrice > 0 {
				maxPricePtr = &maxPrice
			}

			payload := api.InitPolicyRequest{
				Name:        name,
				Description: description,
				Asset:       asset,
				Network:     network,
				MaxPrice:    maxPricePtr,
			}
			if denyHosts != "" {
				payload.DenyHosts = strings.Split(denyHosts, ",")
			}

			res, err := client.InitPolicy(cmd.Context(), payload)
			if err != nil {
				return err
			}
			if err := persistAuthFromClient(&cfg, client); err != nil {
				return err
			}

			state := "updated"
			if res.Created {
				state = "created"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Buyer policy %s: %s (%s)\n", state, res.Policy.Name, res.Policy.ID)
			return nil
		},
	}

	cmd.Flags().BoolVar(&buyer, "buyer", false, "Initialize a buyer policy")
	cmd.Flags().StringVar(&name, "name", "CLI Buyer Policy", "Policy name")
	cmd.Flags().StringVar(&description, "description", "Default buyer policy created by OpenSpend CLI", "Policy description")
	cmd.Flags().StringVar(&asset, "asset", "", "Optional preferred asset")
	cmd.Flags().StringVar(&network, "network", "", "Optional preferred network")
	cmd.Flags().StringVar(&denyHosts, "deny-hosts", "", "Comma-separated deny hosts")
	cmd.Flags().Int64Var(&maxPrice, "max-price", 0, "Optional max price (base units)")
	return cmd
}

func newPolicyListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List policies visible to current user",
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

			type policySummary struct {
				ID         string
				Name       string
				Mode       string
				SubjectCnt int
			}

			policies := make(map[string]policySummary)
			for _, subject := range res.Subjects {
				id := ""
				if subject.PolicyID != nil {
					id = strings.TrimSpace(*subject.PolicyID)
				}
				name := ""
				if subject.PolicyName != nil {
					name = strings.TrimSpace(*subject.PolicyName)
				}
				mode := ""
				if subject.PolicyMode != nil {
					mode = strings.TrimSpace(*subject.PolicyMode)
				}
				if id == "" && name == "" {
					continue
				}

				key := id
				if key == "" {
					key = "name:" + name
				}
				current := policies[key]
				if current.ID == "" {
					current.ID = id
				}
				if current.Name == "" {
					current.Name = name
				}
				if current.Mode == "" {
					current.Mode = mode
				}
				current.SubjectCnt++
				policies[key] = current
			}

			if len(policies) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No policies found.")
				return nil
			}

			keys := make([]string, 0, len(policies))
			for key := range policies {
				keys = append(keys, key)
			}
			sort.Strings(keys)

			for _, key := range keys {
				p := policies[key]
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"- id=%s name=%s mode=%s subjects=%d\n",
					p.ID,
					p.Name,
					p.Mode,
					p.SubjectCnt,
				)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Total policies: %d\n", len(keys))
			return nil
		},
	}
}

func newPolicyDescribeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "describe <policy-id>",
		Short: "Describe a policy by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			policyID := strings.TrimSpace(args[0])
			if policyID == "" {
				return fmt.Errorf("policy ID is required")
			}

			cfg := mustLoadConfig()
			client := clientFromConfig(cfg)

			res, err := client.GetPolicyDetails(cmd.Context(), policyID)
			if err != nil {
				return err
			}
			if err := persistAuthFromClient(&cfg, client); err != nil {
				return err
			}

			policyName := strings.TrimSpace(res.Policy.Name)
			if policyName == "" {
				policyName = "(unnamed)"
			}
			description := "(none)"
			if res.Policy.Description != nil && strings.TrimSpace(*res.Policy.Description) != "" {
				description = strings.TrimSpace(*res.Policy.Description)
			}
			minScore := "(none)"
			if res.Summary.MinScore != nil {
				minScore = fmt.Sprintf("%d", *res.Summary.MinScore)
			}
			budgetMax := "(none)"
			if res.Summary.BudgetMax != nil && strings.TrimSpace(*res.Summary.BudgetMax) != "" {
				budgetMax = strings.TrimSpace(*res.Summary.BudgetMax)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Policy: %s (%s)\n", policyName, res.Policy.ID)
			fmt.Fprintf(cmd.OutOrStdout(), "Mode: %s\n", res.Policy.Mode)
			fmt.Fprintf(cmd.OutOrStdout(), "Status: %s\n", res.Policy.Status)
			fmt.Fprintf(cmd.OutOrStdout(), "Version: %d\n", res.Policy.Version)
			fmt.Fprintf(cmd.OutOrStdout(), "Description: %s\n", description)
			fmt.Fprintln(cmd.OutOrStdout(), "Summary:")
			fmt.Fprintf(cmd.OutOrStdout(), "- min_score=%s\n", minScore)
			fmt.Fprintf(cmd.OutOrStdout(), "- budget_max=%s\n", budgetMax)
			fmt.Fprintf(cmd.OutOrStdout(), "- allow_assets=%s\n", strings.Join(res.Summary.AllowAssets, ","))
			fmt.Fprintf(cmd.OutOrStdout(), "- allow_networks=%s\n", strings.Join(res.Summary.AllowNetworks, ","))
			fmt.Fprintf(cmd.OutOrStdout(), "- deny_hosts=%s\n", strings.Join(res.Summary.DenyHosts, ","))
			fmt.Fprintf(
				cmd.OutOrStdout(),
				"- require_identified_agent=%t\n",
				res.Summary.RequireIdentifiedAgent,
			)

			fmt.Fprintf(cmd.OutOrStdout(), "Rules: %d\n", len(res.Rules))
			for _, rule := range res.Rules {
				resourceHost := ""
				if rule.ResourceHost != nil {
					resourceHost = strings.TrimSpace(*rule.ResourceHost)
				}
				asset := ""
				if rule.Asset != nil {
					asset = strings.TrimSpace(*rule.Asset)
				}
				network := ""
				if rule.Network != nil {
					network = strings.TrimSpace(*rule.Network)
				}
				ruleMinScore := ""
				if rule.MinScore != nil {
					ruleMinScore = fmt.Sprintf("%d", *rule.MinScore)
				}
				maxPrice := ""
				if rule.MaxPrice != nil {
					maxPrice = strings.TrimSpace(*rule.MaxPrice)
				}
				requireIdentified := ""
				if rule.RequireIdentifiedAgent != nil {
					requireIdentified = fmt.Sprintf("%t", *rule.RequireIdentifiedAgent)
				}

				fmt.Fprintf(
					cmd.OutOrStdout(),
					"- id=%s effect=%s scope=%s enabled=%t priority=%d min_score=%s max_price=%s asset=%s network=%s resource_host=%s require_identified=%s\n",
					rule.ID,
					rule.Effect,
					rule.Scope,
					rule.Enabled,
					rule.Priority,
					ruleMinScore,
					maxPrice,
					asset,
					network,
					resourceHost,
					requireIdentified,
				)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Subject bindings: %d\n", len(res.SubjectBindings))
			for _, binding := range res.SubjectBindings {
				externalKey := ""
				if binding.ExternalKey != nil {
					externalKey = strings.TrimSpace(*binding.ExternalKey)
				}
				displayName := ""
				if binding.DisplayName != nil {
					displayName = strings.TrimSpace(*binding.DisplayName)
				}

				fmt.Fprintf(
					cmd.OutOrStdout(),
					"- subject_id=%s key=%s name=%s kind=%s status=%s precedence=%d active=%t\n",
					binding.SubjectID,
					externalKey,
					displayName,
					binding.Kind,
					binding.Status,
					binding.Precedence,
					binding.Active,
				)
			}
			return nil
		},
	}
}
