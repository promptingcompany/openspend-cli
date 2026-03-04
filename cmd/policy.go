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
