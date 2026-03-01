package cmd

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/promptingcompany/openspend-cli/internal/api"
	"github.com/promptingcompany/openspend-cli/internal/config"
	"github.com/promptingcompany/openspend-cli/internal/tui"
	"github.com/spf13/cobra"
)

func newOnboardingCmd() *cobra.Command {
	onboardingCmd := &cobra.Command{
		Use:   "onboarding",
		Short: "Guided onboarding flows",
	}
	onboardingCmd.AddCommand(newBuyerQuickstartCmd())
	return onboardingCmd
}

func newBuyerQuickstartCmd() *cobra.Command {
	var externalKey string
	var displayName string
	var withTUI bool
	var loginTimeout int
	var openYes bool
	var openNo bool
	var callbackHost string

	cmd := &cobra.Command{
		Use:   "buyer-quickstart",
		Short: "Authenticate, initialize policy, and create one buyer agent",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(externalKey) == "" {
				externalKey = fmt.Sprintf("buyer-agent-%d", time.Now().Unix())
			}
			if strings.TrimSpace(displayName) == "" {
				displayName = "Buyer Agent"
			}

			cfg := mustLoadConfig()
			openChoice, err := resolveBrowserOpenChoice(cmd, openYes, openNo)
			if err != nil {
				return err
			}

			if withTUI {
				return runQuickstartTUI(
					cmd,
					cfg,
					loginTimeout,
					openChoice,
					callbackHost,
					externalKey,
					displayName,
				)
			}
			return runQuickstart(
				cmd,
				cfg,
				loginTimeout,
				openChoice,
				callbackHost,
				externalKey,
				displayName,
			)
		},
	}

	cmd.Flags().StringVar(&externalKey, "external-key", "", "Buyer agent external key")
	cmd.Flags().StringVar(&displayName, "display-name", "Buyer Agent", "Buyer agent display name")
	cmd.Flags().BoolVar(&withTUI, "tui", true, "Render Bubble Tea guided output")
	cmd.Flags().IntVar(&loginTimeout, "login-timeout", 180, "Browser login timeout in seconds")
	cmd.Flags().BoolVarP(&openYes, "yes", "y", false, "Automatically open browser without prompting")
	cmd.Flags().BoolVarP(&openNo, "no", "n", false, "Do not open browser automatically")
	cmd.Flags().StringVar(
		&callbackHost,
		"callback-host",
		"127.0.0.1",
		"Host to advertise in callback URL",
	)
	return cmd
}

func runQuickstart(
	cmd *cobra.Command,
	cfg config.Config,
	loginTimeout int,
	openChoice bool,
	callbackHost string,
	externalKey,
	displayName string,
) error {
	client := clientFromConfig(cfg)

	token, err := runBrowserLogin(cmd, cfg, loginTimeout, openChoice, callbackHost)
	if err != nil {
		return err
	}
	cfg.Auth.SessionToken = token
	if err := config.Save(cfg); err != nil {
		return err
	}
	client = clientFromConfig(cfg)

	if _, err := client.InitPolicy(cmd.Context(), api.InitPolicyRequest{}); err != nil {
		return err
	}

	if _, err := client.CreateAgent(cmd.Context(), api.CreateAgentRequest{
		ExternalKey: externalKey,
		DisplayName: displayName,
		Kind:        "agent",
	}); err != nil {
		return err
	}

	who, err := client.WhoAmI(cmd.Context())
	if err != nil {
		return err
	}
	if err := persistAuthFromClient(&cfg, client); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Onboarding complete for user %s. Subjects: %d\n", who.User.ID, len(who.Subjects))
	fmt.Fprintln(cmd.OutOrStdout(), "Next: run `openspend whoami` or start discovery/use-service flows.")
	return nil
}

func runQuickstartTUI(
	cmd *cobra.Command,
	cfg config.Config,
	loginTimeout int,
	openChoice bool,
	callbackHost string,
	externalKey,
	displayName string,
) error {
	model := tui.NewOnboardingModel()
	program := tea.NewProgram(model)

	go func() {
		client := clientFromConfig(cfg)

		program.Send(tui.StepUpdate(0, tui.StepRunning, "logging in"))
		token, err := runBrowserLogin(
			cmd,
			cfg,
			loginTimeout,
			openChoice,
			callbackHost,
		)
		if err != nil {
			program.Send(tui.StepUpdate(0, tui.StepError, err.Error()))
			program.Send(tui.Done())
			return
		}
		cfg.Auth.SessionToken = token
		if err := config.Save(cfg); err != nil {
			program.Send(tui.StepUpdate(0, tui.StepError, err.Error()))
			program.Send(tui.Done())
			return
		}
		program.Send(tui.StepUpdate(0, tui.StepDone, "authenticated"))

		client = clientFromConfig(cfg)

		program.Send(tui.StepUpdate(1, tui.StepRunning, "creating policy"))
		if _, err := client.InitPolicy(cmd.Context(), api.InitPolicyRequest{}); err != nil {
			program.Send(tui.StepUpdate(1, tui.StepError, err.Error()))
			program.Send(tui.Done())
			return
		}
		program.Send(tui.StepUpdate(1, tui.StepDone, "policy ready"))

		program.Send(tui.StepUpdate(2, tui.StepRunning, "creating agent"))
		if _, err := client.CreateAgent(cmd.Context(), api.CreateAgentRequest{
			ExternalKey: externalKey,
			DisplayName: displayName,
			Kind:        "agent",
		}); err != nil {
			program.Send(tui.StepUpdate(2, tui.StepError, err.Error()))
			program.Send(tui.Done())
			return
		}
		program.Send(tui.StepUpdate(2, tui.StepDone, "agent bound"))

		program.Send(tui.StepUpdate(3, tui.StepRunning, "verifying context"))
		who, err := client.WhoAmI(cmd.Context())
		if err != nil {
			program.Send(tui.StepUpdate(3, tui.StepError, err.Error()))
			program.Send(tui.Done())
			return
		}
		if err := persistAuthFromClient(&cfg, client); err != nil {
			program.Send(tui.StepUpdate(3, tui.StepError, err.Error()))
			program.Send(tui.Done())
			return
		}
		program.Send(tui.StepUpdate(3, tui.StepDone, fmt.Sprintf("user=%s subjects=%d", who.User.ID, len(who.Subjects))))
		program.Send(tui.Done())
	}()

	_, err := program.Run()
	return err
}
