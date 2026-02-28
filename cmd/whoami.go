package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newWhoAmICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show current authenticated user and buyer subjects",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := mustLoadConfig()
			client := clientFromConfig(cfg)

			res, err := client.WhoAmI(cmd.Context())
			if err != nil {
				return err
			}

			email := ""
			if res.User.Email != nil {
				email = *res.User.Email
			}
			fmt.Fprintf(cmd.OutOrStdout(), "User: %s (%s)\n", res.User.ID, email)
			fmt.Fprintln(cmd.OutOrStdout(), "Subjects:")
			if len(res.Subjects) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "  - none")
				return nil
			}

			for _, subject := range res.Subjects {
				display := ""
				if subject.DisplayName != nil {
					display = *subject.DisplayName
				}
				externalKey := ""
				if subject.ExternalKey != nil {
					externalKey = *subject.ExternalKey
				}
				policy := ""
				if subject.PolicyName != nil {
					policy = *subject.PolicyName
				}
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"  - %s [%s] key=%s policy=%s\n",
					display,
					subject.Kind,
					externalKey,
					policy,
				)
			}
			return nil
		},
	}
}
