package cmd

import "github.com/spf13/cobra"

func newDashboardCmd() *cobra.Command {
	dashboardCmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Dashboard management commands",
	}
	dashboardCmd.AddCommand(newAgentCmd())
	dashboardCmd.AddCommand(newPolicyCmd())
	return dashboardCmd
}
