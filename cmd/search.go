package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/promptingcompany/openspend-cli/internal/api"
	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	var networks []string
	var limit int
	var budgetMax float64
	var budgetAsset string
	var minServiceScore float64
	var minProviderScore float64
	var minPaymentScore float64
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search marketplace services",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := mustLoadConfig()
			client := clientFromConfig(cfg)

			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				return fmt.Errorf("query is required")
			}

			req := api.SearchRequest{
				Query:            query,
				Networks:         networks,
				Limit:            limit,
				BudgetAsset:      strings.TrimSpace(budgetAsset),
				MinServiceScore:  optionalFloat(minServiceScore),
				MinProviderScore: optionalFloat(minProviderScore),
				MinPaymentScore:  optionalFloat(minPaymentScore),
			}
			if budgetMax > 0 {
				req.BudgetMax = &budgetMax
			}

			res, err := client.Search(cmd.Context(), req)
			if err != nil {
				return err
			}
			if err := persistAuthFromClient(&cfg, client); err != nil {
				return err
			}

			if jsonOut {
				payload, err := json.MarshalIndent(res, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(payload))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Results: %d\n", len(res.Items))
			for i, item := range res.Items {
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"%d. %s\n",
					i+1,
					item.ResourceURL,
				)
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"   score=%.3f min_price=%v %s networks=%s\n",
					item.Score,
					item.MinPrice,
					item.Asset,
					strings.Join(item.Networks, ","),
				)
				if strings.TrimSpace(item.Description) != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "   %s\n", item.Description)
				}
				if strings.TrimSpace(item.Origin.URL) != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "   origin=%s\n", item.Origin.URL)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringSliceVar(&networks, "network", nil, "Network filter (repeatable)")
	cmd.Flags().IntVar(&limit, "limit", 9, "Maximum number of results")
	cmd.Flags().Float64Var(&budgetMax, "budget-max", 0, "Optional maximum price budget filter")
	cmd.Flags().StringVar(&budgetAsset, "budget-asset", "", "Optional budget asset filter (for example USDC)")
	cmd.Flags().Float64Var(&minServiceScore, "min-service-score", 0, "Optional minimum service score filter")
	cmd.Flags().Float64Var(&minProviderScore, "min-provider-score", 0, "Optional minimum provider score filter")
	cmd.Flags().Float64Var(&minPaymentScore, "min-payment-score", 0, "Optional minimum payment score filter")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print raw JSON response")

	return cmd
}

func optionalFloat(value float64) *float64 {
	if value == 0 {
		return nil
	}
	return &value
}
