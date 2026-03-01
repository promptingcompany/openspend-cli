package cmd

import (
	"fmt"
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
