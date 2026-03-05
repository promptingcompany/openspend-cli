package cmd

import (
	"fmt"
	"io"
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
	policyCmd.AddCommand(newPolicyUpdateCmd())
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
			printPolicyDetails(cmd.OutOrStdout(), res)
			return nil
		},
	}
}

func newPolicyUpdateCmd() *cobra.Command {
	var (
		name                        string
		description                 string
		status                      string
		mode                        string
		minScore                    int
		maxPrice                    int64
		asset                       string
		network                     string
		denyHosts                   string
		requireIdentifiedAgent      bool
		clearDescription            bool
		clearMinScore               bool
		clearMaxPrice               bool
		clearAsset                  bool
		clearNetwork                bool
		clearRequireIdentifiedAgent bool
	)

	cmd := &cobra.Command{
		Use:   "update <policy-id>",
		Short: "Update a policy by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			policyID := strings.TrimSpace(args[0])
			if policyID == "" {
				return fmt.Errorf("policy ID is required")
			}

			patch := map[string]any{}

			if cmd.Flags().Changed("name") {
				value := strings.TrimSpace(name)
				if value == "" {
					return fmt.Errorf("--name must not be empty")
				}
				patch["name"] = value
			}

			if clearDescription && cmd.Flags().Changed("description") {
				return fmt.Errorf("use either --description or --clear-description")
			}
			if clearDescription {
				patch["description"] = nil
			}
			if cmd.Flags().Changed("description") {
				patch["description"] = strings.TrimSpace(description)
			}

			if cmd.Flags().Changed("status") {
				value := strings.ToLower(strings.TrimSpace(status))
				switch value {
				case "active", "inactive":
					patch["status"] = value
				default:
					return fmt.Errorf("--status must be one of: active, inactive")
				}
			}

			if cmd.Flags().Changed("mode") {
				value := strings.ToLower(strings.TrimSpace(mode))
				switch value {
				case "buy", "sell", "both":
					patch["mode"] = value
				default:
					return fmt.Errorf("--mode must be one of: buy, sell, both")
				}
			}

			if clearMinScore && cmd.Flags().Changed("min-score") {
				return fmt.Errorf("use either --min-score or --clear-min-score")
			}
			if clearMinScore {
				patch["minScore"] = nil
			}
			if cmd.Flags().Changed("min-score") {
				if minScore < 0 {
					return fmt.Errorf("--min-score must be non-negative")
				}
				patch["minScore"] = minScore
			}

			if clearMaxPrice && cmd.Flags().Changed("max-price") {
				return fmt.Errorf("use either --max-price or --clear-max-price")
			}
			if clearMaxPrice {
				patch["maxPrice"] = nil
			}
			if cmd.Flags().Changed("max-price") {
				if maxPrice < 0 {
					return fmt.Errorf("--max-price must be non-negative")
				}
				patch["maxPrice"] = maxPrice
			}

			if clearAsset && cmd.Flags().Changed("asset") {
				return fmt.Errorf("use either --asset or --clear-asset")
			}
			if clearAsset {
				patch["asset"] = nil
			}
			if cmd.Flags().Changed("asset") {
				value := strings.TrimSpace(asset)
				if value == "" {
					return fmt.Errorf("--asset must not be empty")
				}
				patch["asset"] = value
			}

			if clearNetwork && cmd.Flags().Changed("network") {
				return fmt.Errorf("use either --network or --clear-network")
			}
			if clearNetwork {
				patch["network"] = nil
			}
			if cmd.Flags().Changed("network") {
				value := strings.TrimSpace(network)
				if value == "" {
					return fmt.Errorf("--network must not be empty")
				}
				patch["network"] = value
			}

			if clearRequireIdentifiedAgent && cmd.Flags().Changed("require-identified-agent") {
				return fmt.Errorf("use either --require-identified-agent or --clear-require-identified-agent")
			}
			if clearRequireIdentifiedAgent {
				patch["requireIdentifiedAgent"] = nil
			}
			if cmd.Flags().Changed("require-identified-agent") {
				patch["requireIdentifiedAgent"] = requireIdentifiedAgent
			}

			if cmd.Flags().Changed("deny-hosts") {
				items := make([]string, 0)
				seen := make(map[string]struct{})
				for _, raw := range strings.Split(denyHosts, ",") {
					host := strings.TrimSpace(raw)
					if host == "" {
						continue
					}
					if _, exists := seen[host]; exists {
						continue
					}
					seen[host] = struct{}{}
					items = append(items, host)
				}
				patch["denyHosts"] = items
			}

			if len(patch) == 0 {
				return fmt.Errorf("no update fields provided")
			}

			cfg := mustLoadConfig()
			client := clientFromConfig(cfg)

			res, err := client.UpdatePolicy(cmd.Context(), policyID, patch)
			if err != nil {
				return err
			}
			if err := persistAuthFromClient(&cfg, client); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Policy updated.")
			printPolicyDetails(cmd.OutOrStdout(), res)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Updated policy name")
	cmd.Flags().StringVar(&description, "description", "", "Updated policy description")
	cmd.Flags().BoolVar(&clearDescription, "clear-description", false, "Clear policy description")
	cmd.Flags().StringVar(&status, "status", "", "Updated policy status (active|inactive)")
	cmd.Flags().StringVar(&mode, "mode", "", "Updated policy mode (buy|sell|both)")
	cmd.Flags().IntVar(&minScore, "min-score", 0, "Updated minimum score for allow global rule")
	cmd.Flags().BoolVar(&clearMinScore, "clear-min-score", false, "Clear minimum score on allow global rule")
	cmd.Flags().Int64Var(&maxPrice, "max-price", 0, "Updated max price (base units) for allow global rule")
	cmd.Flags().BoolVar(&clearMaxPrice, "clear-max-price", false, "Clear max price on allow global rule")
	cmd.Flags().StringVar(&asset, "asset", "", "Updated preferred asset for allow global rule")
	cmd.Flags().BoolVar(&clearAsset, "clear-asset", false, "Clear preferred asset on allow global rule")
	cmd.Flags().StringVar(&network, "network", "", "Updated preferred network for allow global rule")
	cmd.Flags().BoolVar(&clearNetwork, "clear-network", false, "Clear preferred network on allow global rule")
	cmd.Flags().BoolVar(
		&requireIdentifiedAgent,
		"require-identified-agent",
		false,
		"Set requireIdentifiedAgent on allow global rule",
	)
	cmd.Flags().BoolVar(
		&clearRequireIdentifiedAgent,
		"clear-require-identified-agent",
		false,
		"Clear requireIdentifiedAgent on allow global rule",
	)
	cmd.Flags().StringVar(
		&denyHosts,
		"deny-hosts",
		"",
		"Replace deny service hosts with a comma-separated list (empty to clear)",
	)
	return cmd
}

func printPolicyDetails(out io.Writer, res api.PolicyDetailsResponse) {
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

	fmt.Fprintf(out, "Policy: %s (%s)\n", policyName, res.Policy.ID)
	fmt.Fprintf(out, "Mode: %s\n", res.Policy.Mode)
	fmt.Fprintf(out, "Status: %s\n", res.Policy.Status)
	fmt.Fprintf(out, "Version: %d\n", res.Policy.Version)
	fmt.Fprintf(out, "Description: %s\n", description)
	fmt.Fprintln(out, "Summary:")
	fmt.Fprintf(out, "- min_score=%s\n", minScore)
	fmt.Fprintf(out, "- budget_max=%s\n", budgetMax)
	fmt.Fprintf(out, "- allow_assets=%s\n", strings.Join(res.Summary.AllowAssets, ","))
	fmt.Fprintf(out, "- allow_networks=%s\n", strings.Join(res.Summary.AllowNetworks, ","))
	fmt.Fprintf(out, "- deny_hosts=%s\n", strings.Join(res.Summary.DenyHosts, ","))
	fmt.Fprintf(
		out,
		"- require_identified_agent=%t\n",
		res.Summary.RequireIdentifiedAgent,
	)

	fmt.Fprintf(out, "Rules: %d\n", len(res.Rules))
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
			out,
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

	fmt.Fprintf(out, "Subject bindings: %d\n", len(res.SubjectBindings))
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
			out,
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
}
