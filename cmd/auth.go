package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/promptingcompany/openspend-cli/internal/api"
	"github.com/promptingcompany/openspend-cli/internal/config"
	"github.com/spf13/cobra"
)

func newAuthCmd() *cobra.Command {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication commands",
	}
	authCmd.AddCommand(newAuthLoginCmd())
	authCmd.AddCommand(newAuthLogoutCmd())
	return authCmd
}

func newAuthLoginCmd() *cobra.Command {
	var timeoutSeconds int
	var openYes bool
	var openNo bool
	var callbackHost string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Open marketplace login in browser and capture CLI session",
		RunE: func(cmd *cobra.Command, _ []string) error {
			openChoice, err := resolveBrowserOpenChoice(cmd, openYes, openNo)
			if err != nil {
				return err
			}

			cfg := mustLoadConfig()
			loginCallback, err := runBrowserLogin(
				cmd,
				cfg,
				timeoutSeconds,
				openChoice,
				callbackHost,
			)
			if err != nil {
				return err
			}

			loginCfg := cfg
			loginCfg.Auth.SessionToken = loginCallback.token
			loginCfg.Auth.AuthTokenType = config.AuthTokenCookie
			if strings.TrimSpace(loginCallback.cookieName) != "" {
				loginCfg.Auth.SessionCookie = strings.TrimSpace(loginCallback.cookieName)
			}

			client := clientFromConfig(loginCfg)
			// Best effort: fetch session metadata/expiry from Better Auth endpoint.
			if err := client.SyncSession(cmd.Context()); err != nil {
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"Warning: could not sync session metadata: %v\n",
					err,
				)
			}

			who, err := client.WhoAmI(cmd.Context())
			if err != nil {
				return fmt.Errorf("authenticated but failed to load subjects for identity selection: %w", err)
			}
			choice, err := resolveLoginIdentityChoice(cmd, who)
			if err != nil {
				return err
			}
			exchangeReq := api.ExchangeCliAuthRequest{
				LoginAs: choice.loginAs,
			}
			if strings.TrimSpace(choice.subjectKey) != "" {
				exchangeReq.SubjectExternalKey = strings.TrimSpace(choice.subjectKey)
			}
			exchangeRes, err := client.ExchangeCliAuth(cmd.Context(), exchangeReq)
			if err != nil {
				return err
			}

			subjectKey := ""
			if exchangeRes.SubjectExternalKey != nil {
				subjectKey = strings.TrimSpace(*exchangeRes.SubjectExternalKey)
			}
			subjectName := ""
			if exchangeRes.SubjectDisplayName != nil {
				subjectName = strings.TrimSpace(*exchangeRes.SubjectDisplayName)
			}

			client.SetAuthToken(exchangeRes.CliToken, config.AuthTokenBearer)

			applyExchangedAuthConfig(&cfg, exchangeRes)
			if err := persistAuthFromClient(&cfg, client); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Logged in successfully against %s\n", cfg.Marketplace.BaseURL)
			switch exchangeRes.LoginAs {
			case config.AuthLoginAsAgent:
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"CLI identity: agent (%s key=%s)\n",
					subjectName,
					subjectKey,
				)
			default:
				fmt.Fprintln(cmd.OutOrStdout(), "CLI identity: admin (self)")
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&timeoutSeconds, "timeout", 180, "Login timeout in seconds")
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

func newAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Clear local CLI authentication session",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := mustLoadConfig()
			if !clearAuthSession(&cfg) {
				fmt.Fprintln(cmd.OutOrStdout(), "Already logged out.")
				return nil
			}
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Logged out.")
			return nil
		},
	}
}

func clearAuthSession(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}

	changed := false
	if strings.TrimSpace(cfg.Auth.SessionToken) != "" {
		cfg.Auth.SessionToken = ""
		changed = true
	}
	if !cfg.Auth.SessionExpiresAt.IsZero() {
		cfg.Auth.SessionExpiresAt = time.Time{}
		changed = true
	}
	if cfg.Auth.AuthTokenType != config.AuthTokenCookie {
		cfg.Auth.AuthTokenType = config.AuthTokenCookie
		changed = true
	}
	return changed
}

type loginIdentityChoice struct {
	loginAs    string
	subjectKey string
}

type selectableAgent struct {
	externalKey string
	displayName string
}

func resolveLoginIdentityChoice(
	cmd *cobra.Command,
	who api.WhoAmIResponse,
) (loginIdentityChoice, error) {
	agents := extractSelectableAgents(who)
	if len(agents) == 0 {
		return loginIdentityChoice{loginAs: config.AuthLoginAsSelf}, nil
	}
	return promptIdentityChoice(cmd, agents)
}

func applyExchangedAuthConfig(cfg *config.Config, exchangeRes api.ExchangeCliAuthResponse) {
	if cfg == nil {
		return
	}
	cfg.Auth.SessionToken = exchangeRes.CliToken
	cfg.Auth.AuthTokenType = config.AuthTokenBearer
	// Always reset first so a token is never paired with stale expiry metadata.
	cfg.Auth.SessionExpiresAt = time.Time{}
	if exchangeRes.ExpiresAt != nil {
		cfg.Auth.SessionExpiresAt = exchangeRes.ExpiresAt.UTC()
	}
}

func extractSelectableAgents(who api.WhoAmIResponse) []selectableAgent {
	agents := make([]selectableAgent, 0)
	for _, subject := range who.Subjects {
		if subject.Status != "active" {
			continue
		}
		if subject.Kind != "agent" && subject.Kind != "anonymous_agent" {
			continue
		}
		if subject.ExternalKey == nil || strings.TrimSpace(*subject.ExternalKey) == "" {
			continue
		}
		name := strings.TrimSpace(*subject.ExternalKey)
		if subject.DisplayName != nil && strings.TrimSpace(*subject.DisplayName) != "" {
			name = strings.TrimSpace(*subject.DisplayName)
		}
		agents = append(agents, selectableAgent{
			externalKey: strings.TrimSpace(*subject.ExternalKey),
			displayName: name,
		})
	}
	return agents
}

func promptIdentityChoice(
	cmd *cobra.Command,
	agents []selectableAgent,
) (loginIdentityChoice, error) {
	fmt.Fprintln(cmd.OutOrStdout(), "Choose CLI identity:")
	fmt.Fprintln(cmd.OutOrStdout(), "  1) Admin (self)")
	for i, agent := range agents {
		fmt.Fprintf(
			cmd.OutOrStdout(),
			"  %d) Agent: %s (key=%s)\n",
			i+2,
			agent.displayName,
			agent.externalKey,
		)
	}

	reader := bufio.NewReader(os.Stdin)
	maxChoice := len(agents) + 1
	for {
		fmt.Fprintf(cmd.OutOrStdout(), "Select identity [1-%d] (default 1): ", maxChoice)
		raw, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return loginIdentityChoice{}, err
		}
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return loginIdentityChoice{loginAs: config.AuthLoginAsSelf}, nil
		}

		selection, parseErr := strconv.Atoi(raw)
		if parseErr != nil || selection < 1 || selection > maxChoice {
			fmt.Fprintf(cmd.OutOrStdout(), "Please enter a number from 1 to %d.\n", maxChoice)
			if errors.Is(err, io.EOF) {
				return loginIdentityChoice{loginAs: config.AuthLoginAsSelf}, nil
			}
			continue
		}

		if selection == 1 {
			return loginIdentityChoice{loginAs: config.AuthLoginAsSelf}, nil
		}

		agent := agents[selection-2]
		return loginIdentityChoice{
			loginAs:    config.AuthLoginAsAgent,
			subjectKey: agent.externalKey,
		}, nil
	}
}

type browserLoginCallback struct {
	token      string
	cookieName string
}

func runBrowserLogin(
	cmd *cobra.Command,
	cfg config.Config,
	timeoutSeconds int,
	openChoice bool,
	callbackHost string,
) (browserLoginCallback, error) {
	client := clientFromConfig(cfg)

	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		return browserLoginCallback{}, fmt.Errorf("failed to bind callback server: %w", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	callbackURL := fmt.Sprintf("http://%s:%d/callback", callbackHost, port)
	loginURL, err := client.BrowserLoginURL(callbackURL)
	if err != nil {
		return browserLoginCallback{}, err
	}

	tokenCh := make(chan browserLoginCallback, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("session_token")
		if token == "" {
			http.Error(w, "missing session_token", http.StatusBadRequest)
			errCh <- fmt.Errorf("callback missing session_token")
			return
		}

		html := `<!doctype html><html><body><h3>OpenSpend CLI authenticated.</h3><p>You can return to terminal.</p></body></html>`
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
		tokenCh <- browserLoginCallback{
			token:      token,
			cookieName: r.URL.Query().Get("session_cookie"),
		}
	})

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if serveErr := srv.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
			errCh <- serveErr
		}
	}()
	defer func() {
		_ = srv.Close()
	}()

	fmt.Fprintf(cmd.OutOrStdout(), "Login URL: %s\n", loginURL)
	if openChoice {
		if err := openBrowser(loginURL); err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Could not auto-open browser: %v\n", err)
			fmt.Fprintln(cmd.OutOrStdout(), "Open the URL manually.")
		}
	}

	timeout := time.Duration(timeoutSeconds) * time.Second
	select {
	case loginRes := <-tokenCh:
		return loginRes, nil
	case err := <-errCh:
		return browserLoginCallback{}, err
	case <-time.After(timeout):
		return browserLoginCallback{}, fmt.Errorf("timed out waiting for browser callback after %s", timeout)
	}
}

func openBrowser(rawURL string) error {
	_, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return err
	}

	var command string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		command = "open"
		args = []string{rawURL}
	case "windows":
		command = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", rawURL}
	default:
		command = "xdg-open"
		args = []string{rawURL}
	}

	return exec.Command(command, args...).Start()
}

func resolveBrowserOpenChoice(cmd *cobra.Command, openYes bool, openNo bool) (bool, error) {
	if openYes && openNo {
		return false, errors.New("cannot use both -y/--yes and -n/--no")
	}
	if openYes {
		return true, nil
	}
	if openNo {
		return false, nil
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Fprint(cmd.OutOrStdout(), "Open login page in your browser now? (Y/n): ")
		raw, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return false, err
		}

		answer := strings.ToLower(strings.TrimSpace(raw))
		switch answer {
		case "", "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Fprintln(cmd.OutOrStdout(), "Please answer Y or n.")
		}

		if errors.Is(err, io.EOF) {
			return false, nil
		}
	}
}
