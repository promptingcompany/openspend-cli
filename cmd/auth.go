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
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/promptingcompany/openspend-cli/internal/api"
	"github.com/promptingcompany/openspend-cli/internal/config"
	"github.com/spf13/cobra"
)

var cloudflareTunnelURLRegex = regexp.MustCompile(`https://[a-zA-Z0-9.-]+\.trycloudflare\.com`)

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
	var useLegacyBrowserCallback bool
	var useCloudflareTunnel bool
	var cloudflaredBin string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Open marketplace login in browser and capture CLI session",
		Long: strings.TrimSpace(`
Open marketplace login in browser and capture CLI session.

Default mode uses a device-style browser approval flow (no localhost callback required).
Legacy callback mode is available with ` + "`--legacy-browser-callback`" + `.
In legacy mode, optional remote/sandbox callback uses ` + "`--cloudflare-tunnel`" + `.
`),
		Example: strings.TrimSpace(`
  openspend auth login
  openspend auth login --legacy-browser-callback
  openspend auth login --legacy-browser-callback --cloudflare-tunnel
`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			openChoice, err := resolveBrowserOpenChoice(cmd, openYes, openNo)
			if err != nil {
				return err
			}
			if useCloudflareTunnel && !useLegacyBrowserCallback {
				return errors.New("--cloudflare-tunnel requires --legacy-browser-callback")
			}
			if strings.TrimSpace(cloudflaredBin) != "cloudflared" && !useLegacyBrowserCallback {
				return errors.New("--cloudflared-bin requires --legacy-browser-callback")
			}

			cfg := mustLoadConfig()
			loginCfg := cfg
			if useLegacyBrowserCallback {
				fmt.Fprintln(
					cmd.OutOrStdout(),
					"Using deprecated legacy callback login mode. Prefer default device flow.",
				)
				loginCallback, err := runBrowserLogin(
					cmd,
					cfg,
					timeoutSeconds,
					openChoice,
					callbackHost,
					useCloudflareTunnel,
					cloudflaredBin,
				)
				if err != nil {
					return err
				}
				loginCfg.Auth.SessionToken = loginCallback.token
				loginCfg.Auth.AuthTokenType = config.AuthTokenCookie
				if strings.TrimSpace(loginCallback.cookieName) != "" {
					loginCfg.Auth.SessionCookie = strings.TrimSpace(loginCallback.cookieName)
				}
			} else {
				if callbackHost != "127.0.0.1" {
					fmt.Fprintf(
						cmd.OutOrStdout(),
						"Note: --callback-host is ignored in device flow mode (value=%q).\n",
						callbackHost,
					)
				}
				deviceLogin, err := runDeviceBrowserLogin(
					cmd,
					cfg,
					timeoutSeconds,
					openChoice,
				)
				if err != nil {
					return err
				}
				loginCfg.Auth.SessionToken = deviceLogin.CliToken
				loginCfg.Auth.AuthTokenType = config.AuthTokenBearer
				if parsedExpiry, parseErr := time.Parse(time.RFC3339, deviceLogin.CliTokenExpiresAt); parseErr == nil {
					loginCfg.Auth.SessionExpiresAt = parsedExpiry.UTC()
				}
			}

			client := clientFromConfig(loginCfg)
			if useLegacyBrowserCallback {
				// Best effort: fetch session metadata/expiry from Better Auth endpoint.
				if err := client.SyncSession(cmd.Context()); err != nil {
					fmt.Fprintf(
						cmd.OutOrStdout(),
						"Warning: could not sync session metadata: %v\n",
						err,
					)
				}
			}

			who, err := client.WhoAmI(cmd.Context())
			if err != nil {
				return fmt.Errorf("authenticated but failed to load subjects for identity selection: %w", err)
			}
			choice, err := resolveLoginIdentityChoice(cmd, who)
			if err != nil {
				return err
			}
			exchangeRes := api.ExchangeCliAuthResponse{
				CliToken: loginCfg.Auth.SessionToken,
				LoginAs:  config.AuthLoginAsSelf,
			}
			if !loginCfg.Auth.SessionExpiresAt.IsZero() {
				expires := loginCfg.Auth.SessionExpiresAt.UTC()
				exchangeRes.ExpiresAt = &expires
			}
			shouldExchange := useLegacyBrowserCallback || choice.loginAs == config.AuthLoginAsAgent
			if shouldExchange {
				exchangeReq := api.ExchangeCliAuthRequest{
					LoginAs: choice.loginAs,
				}
				if strings.TrimSpace(choice.subjectKey) != "" {
					exchangeReq.SubjectExternalKey = strings.TrimSpace(choice.subjectKey)
				}
				exchangeRes, err = client.ExchangeCliAuth(cmd.Context(), exchangeReq)
				if err != nil {
					return err
				}
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
	cmd.Flags().BoolVar(
		&useLegacyBrowserCallback,
		"legacy-browser-callback",
		false,
		"Use deprecated localhost callback mode instead of default device flow",
	)
	cmd.Flags().StringVar(
		&callbackHost,
		"callback-host",
		"127.0.0.1",
		"Host to advertise in callback URL (legacy callback mode only)",
	)
	cmd.Flags().BoolVar(
		&useCloudflareTunnel,
		"cloudflare-tunnel",
		false,
		"Expose callback via temporary Cloudflare Tunnel (legacy callback mode only; requires cloudflared)",
	)
	cmd.Flags().StringVar(
		&cloudflaredBin,
		"cloudflared-bin",
		"cloudflared",
		"Path to cloudflared binary used with --cloudflare-tunnel",
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
	useCloudflareTunnel bool,
	cloudflaredBin string,
) (browserLoginCallback, error) {
	client := clientFromConfig(cfg)
	printCloudflareTunnelModeHint(
		cmd.OutOrStdout(),
		useCloudflareTunnel,
		cloudflaredBin,
	)

	listenAddr := "0.0.0.0:0"
	if useCloudflareTunnel {
		// Tunnel mode only needs local loopback exposure.
		listenAddr = "127.0.0.1:0"
	}
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return browserLoginCallback{}, fmt.Errorf("failed to bind callback server: %w", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	callbackURL := fmt.Sprintf("http://%s:%d/callback", callbackHost, port)
	stopTunnel := func() {}
	if useCloudflareTunnel {
		publicURL, cleanup, tunnelErr := startCloudflareQuickTunnel(
			cmd.OutOrStdout(),
			cloudflaredBin,
			fmt.Sprintf("http://127.0.0.1:%d", port),
			20*time.Second,
		)
		if tunnelErr != nil {
			return browserLoginCallback{}, tunnelErr
		}
		stopTunnel = cleanup
		callbackURL = strings.TrimRight(publicURL, "/") + "/callback"
		fmt.Fprintf(cmd.OutOrStdout(), "Callback tunnel URL: %s\n", callbackURL)
	}
	defer stopTunnel()
	printRedirectHostCompatibilityWarning(
		cmd.OutOrStdout(),
		cfg.Marketplace.BaseURL,
		callbackURL,
	)

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

func runDeviceBrowserLogin(
	cmd *cobra.Command,
	cfg config.Config,
	timeoutSeconds int,
	openChoice bool,
) (api.CliDeviceAuthPollResponse, error) {
	client := clientFromConfig(cfg)
	startRes, err := client.StartCliDeviceAuth(cmd.Context())
	if err != nil {
		return api.CliDeviceAuthPollResponse{}, err
	}

	fmt.Fprintln(
		cmd.OutOrStdout(),
		"Using device login flow (no local callback server required).",
	)
	fmt.Fprintf(cmd.OutOrStdout(), "Verification URL: %s\n", startRes.VerificationURI)
	fmt.Fprintf(cmd.OutOrStdout(), "Verification Code: %s\n", startRes.UserCode)
	if strings.TrimSpace(startRes.VerificationURIComplete) != "" {
		fmt.Fprintf(
			cmd.OutOrStdout(),
			"Verification URL (prefilled): %s\n",
			startRes.VerificationURIComplete,
		)
	}

	targetURL := startRes.VerificationURIComplete
	if strings.TrimSpace(targetURL) == "" {
		targetURL = startRes.VerificationURI
	}
	if openChoice {
		if err := openBrowser(targetURL); err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Could not auto-open browser: %v\n", err)
			fmt.Fprintln(cmd.OutOrStdout(), "Open the URL manually.")
		}
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Waiting for approval...")
	timeout := time.Duration(timeoutSeconds) * time.Second
	deadline := time.Now().Add(timeout)
	intervalSeconds := startRes.IntervalSeconds
	if intervalSeconds <= 0 {
		intervalSeconds = 2
	}

	for {
		pollRes, pollErr := client.PollCliDeviceAuth(cmd.Context(), api.CliDeviceAuthPollRequest{
			LoginSessionID: startRes.LoginSessionID,
			PollToken:      startRes.PollToken,
		})
		if pollErr != nil {
			return api.CliDeviceAuthPollResponse{}, pollErr
		}

		switch strings.ToLower(strings.TrimSpace(pollRes.Status)) {
		case "approved":
			if strings.TrimSpace(pollRes.CliToken) == "" {
				return api.CliDeviceAuthPollResponse{}, errors.New("login approved but cli token was empty")
			}
			return pollRes, nil
		case "pending":
			if pollRes.IntervalSeconds > 0 {
				intervalSeconds = pollRes.IntervalSeconds
			}
		case "denied":
			return api.CliDeviceAuthPollResponse{}, errors.New("login was denied")
		case "expired":
			return api.CliDeviceAuthPollResponse{}, errors.New("login session expired; run openspend auth login again")
		case "consumed":
			return api.CliDeviceAuthPollResponse{}, errors.New("login session already consumed; run openspend auth login again")
		default:
			return api.CliDeviceAuthPollResponse{}, fmt.Errorf("unexpected login status: %q", pollRes.Status)
		}

		if time.Now().After(deadline) {
			return api.CliDeviceAuthPollResponse{}, fmt.Errorf("timed out waiting for browser approval after %s", timeout)
		}
		sleepFor := time.Duration(intervalSeconds) * time.Second
		if sleepFor <= 0 {
			sleepFor = 2 * time.Second
		}
		timeRemaining := time.Until(deadline)
		if sleepFor > timeRemaining {
			sleepFor = timeRemaining
		}
		if sleepFor > 0 {
			time.Sleep(sleepFor)
		}
	}
}

func startCloudflareQuickTunnel(
	out io.Writer,
	cloudflaredBin string,
	localURL string,
	startupTimeout time.Duration,
) (string, func(), error) {
	if strings.TrimSpace(cloudflaredBin) == "" {
		cloudflaredBin = "cloudflared"
	}
	if _, err := exec.LookPath(cloudflaredBin); err != nil {
		return "", nil, fmt.Errorf(
			"cloudflare tunnel requested but %q is not installed or not in PATH. %s",
			cloudflaredBin,
			cloudflaredInstallHint(runtime.GOOS),
		)
	}

	proc := exec.Command(
		cloudflaredBin,
		"tunnel",
		"--url",
		localURL,
		"--no-autoupdate",
	)
	stdout, err := proc.StdoutPipe()
	if err != nil {
		return "", nil, fmt.Errorf("failed to capture cloudflared stdout: %w", err)
	}
	stderr, err := proc.StderrPipe()
	if err != nil {
		return "", nil, fmt.Errorf("failed to capture cloudflared stderr: %w", err)
	}
	if err := proc.Start(); err != nil {
		return "", nil, fmt.Errorf("failed to start cloudflared: %w", err)
	}

	cleanup := func() {
		if proc.Process == nil {
			return
		}
		if proc.ProcessState != nil && proc.ProcessState.Exited() {
			return
		}
		_ = proc.Process.Kill()
	}

	lineCh := make(chan string, 64)
	readOutput := func(r io.Reader) {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			select {
			case lineCh <- line:
			default:
			}
		}
	}
	go readOutput(stdout)
	go readOutput(stderr)

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- proc.Wait()
	}()

	timeout := time.NewTimer(startupTimeout)
	defer timeout.Stop()
	for {
		select {
		case line := <-lineCh:
			if strings.TrimSpace(line) == "" {
				continue
			}
			if out != nil {
				fmt.Fprintf(out, "cloudflared: %s\n", line)
			}
			if match := extractCloudflareTunnelURL(line); match != "" {
				return match, cleanup, nil
			}
		case err := <-waitCh:
			cleanup()
			if err == nil {
				return "", nil, errors.New("cloudflared exited before publishing a tunnel URL")
			}
			return "", nil, fmt.Errorf("cloudflared exited early: %w", err)
		case <-timeout.C:
			cleanup()
			return "", nil, fmt.Errorf(
				"timed out after %s waiting for cloudflared tunnel URL",
				startupTimeout,
			)
		}
	}
}

func extractCloudflareTunnelURL(line string) string {
	return cloudflareTunnelURLRegex.FindString(line)
}

func cloudflaredInstallHint(goos string) string {
	switch goos {
	case "darwin":
		return "Install cloudflared with: brew install cloudflared"
	case "windows":
		return "Install cloudflared with: winget install --id Cloudflare.cloudflared"
	default:
		return "Install cloudflared: https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/"
	}
}

func printCloudflareTunnelModeHint(out io.Writer, enabled bool, cloudflaredBin string) {
	if out == nil {
		return
	}
	if enabled {
		fmt.Fprintf(
			out,
			"Using Cloudflare Tunnel callback mode (binary: %s).\n",
			strings.TrimSpace(cloudflaredBin),
		)
		return
	}
	fmt.Fprintln(
		out,
		"Using local callback mode. Optional: add --cloudflare-tunnel for remote/sandbox environments.",
	)
	fmt.Fprintf(out, "%s\n", cloudflaredInstallHint(runtime.GOOS))
}

func printRedirectHostCompatibilityWarning(out io.Writer, baseURL string, callbackURL string) {
	if out == nil {
		return
	}
	baseHost := ""
	if u, err := url.Parse(baseURL); err == nil {
		baseHost = strings.ToLower(strings.TrimSpace(u.Hostname()))
	}
	callbackHost := ""
	if u, err := url.Parse(callbackURL); err == nil {
		callbackHost = strings.ToLower(strings.TrimSpace(u.Hostname()))
	}
	if callbackHost == "" {
		return
	}
	if callbackHost == "localhost" || callbackHost == "127.0.0.1" {
		return
	}
	if baseHost != "" && callbackHost == baseHost {
		return
	}
	fmt.Fprintf(
		out,
		"Warning: callback host %q may be rejected by server redirect policy. "+
			"Many deployments only allow localhost/127.0.0.1 or the marketplace host (%q).\n",
		callbackHost,
		baseHost,
	)
	fmt.Fprintln(
		out,
		"If login fails with \"redirect_uri is required and must use localhost, 127.0.0.1, or the current request host\", use local callback mode or update backend redirect policy.",
	)
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
