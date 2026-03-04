package cmd

import (
	"fmt"

	"github.com/promptingcompany/openspend-cli/internal/config"
	"github.com/spf13/cobra"
)

func newWhoAmICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show current authenticated user and CLI identity",
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

			email := ""
			if res.User.Email != nil {
				email = *res.User.Email
			}
			fmt.Fprintf(cmd.OutOrStdout(), "User: %s (%s)\n", res.User.ID, email)

			identity := inferAuthIdentity(cfg.Auth.AuthTokenType, cfg.Auth.SessionToken)
			switch identity.LoginAs {
			case config.AuthLoginAsAgent:
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"CLI identity: agent (key=%s name=%s)\n",
					identity.SubjectKey,
					identity.SubjectName,
				)
			default:
				fmt.Fprintln(cmd.OutOrStdout(), "CLI identity: admin (self)")
			}
			return nil
		},
	}
}
