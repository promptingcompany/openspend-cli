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
	"strings"
	"time"

	"github.com/promptingcompany/openspend-cli/internal/config"
	"github.com/spf13/cobra"
)

func newAuthCmd() *cobra.Command {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication commands",
	}
	authCmd.AddCommand(newAuthLoginCmd())
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
			token, err := runBrowserLogin(
				cmd,
				cfg,
				timeoutSeconds,
				openChoice,
				callbackHost,
			)
			if err != nil {
				return err
			}

			cfg.Auth.SessionToken = token
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Logged in successfully against %s\n", cfg.Marketplace.BaseURL)
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

func runBrowserLogin(
	cmd *cobra.Command,
	cfg config.Config,
	timeoutSeconds int,
	openChoice bool,
	callbackHost string,
) (string, error) {
	client := clientFromConfig(cfg)

	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		return "", fmt.Errorf("failed to bind callback server: %w", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	callbackURL := fmt.Sprintf("http://%s:%d/callback", callbackHost, port)
	loginURL, err := client.BrowserLoginURL(callbackURL)
	if err != nil {
		return "", err
	}

	tokenCh := make(chan string, 1)
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
		tokenCh <- token
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
	case token := <-tokenCh:
		return token, nil
	case err := <-errCh:
		return "", err
	case <-time.After(timeout):
		return "", fmt.Errorf("timed out waiting for browser callback after %s", timeout)
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
