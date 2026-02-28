package cmd

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const defaultReleaseRepoSlug = "promptingcompany/openspend-cli"

type githubRelease struct {
	TagName string `json:"tag_name"`
}

func newUpdateCmd() *cobra.Command {
	var requestedVersion string
	var repoSlug string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update openspend CLI binary in place",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(repoSlug) == "" {
				repoSlug = defaultReleaseRepoSlug
			}

			version, err := resolveVersion(repoSlug, requestedVersion)
			if err != nil {
				return err
			}

			current := strings.TrimSpace(cliVersion)
			fmt.Fprintf(cmd.OutOrStdout(), "Current version: %s\n", fallbackVersion(current))
			fmt.Fprintf(cmd.OutOrStdout(), "Target version: %s\n", version)

			exePath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("failed to locate current executable: %w", err)
			}
			exePath, err = filepath.EvalSymlinks(exePath)
			if err != nil {
				return fmt.Errorf("failed to resolve executable path: %w", err)
			}

			if err := updateExecutable(repoSlug, version, exePath); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Updated openspend to %s at %s\n", version, exePath)
			return nil
		},
	}

	cmd.Flags().StringVar(&requestedVersion, "version", "latest", "Release tag/version to install (e.g. v0.1.0-rc.10 or latest)")
	cmd.Flags().StringVar(&repoSlug, "repo", defaultReleaseRepoSlug, "GitHub repository slug for release assets")
	return cmd
}

func fallbackVersion(v string) string {
	if strings.TrimSpace(v) == "" {
		return "unknown"
	}
	return v
}

func resolveVersion(repoSlug, requested string) (string, error) {
	req := strings.TrimSpace(requested)
	if req == "" || req == "latest" {
		u := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repoSlug)
		resp, err := http.Get(u)
		if err != nil {
			return "", fmt.Errorf("failed to fetch latest release: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			return "", fmt.Errorf("failed to fetch latest release: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
		}

		var rel githubRelease
		if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
			return "", fmt.Errorf("failed to parse latest release: %w", err)
		}
		if strings.TrimSpace(rel.TagName) == "" {
			return "", errors.New("latest release tag not found")
		}
		return rel.TagName, nil
	}

	if strings.HasPrefix(req, "v") {
		return req, nil
	}
	return "v" + req, nil
}

func updateExecutable(repoSlug, version, exePath string) error {
	osPart, archPart, err := releaseAssetParts()
	if err != nil {
		return err
	}

	archiveName := fmt.Sprintf("openspend_%s_%s_%s.tar.gz", strings.TrimPrefix(version, "v"), osPart, archPart)
	downloadURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repoSlug, version, archiveName)

	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download release archive: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("failed to download release archive: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	newBin, err := extractBinaryFromTarGz(resp.Body)
	if err != nil {
		return err
	}

	tmpPath := exePath + ".new"
	if err := os.WriteFile(tmpPath, newBin, 0o755); err != nil {
		return fmt.Errorf("failed to write new binary: %w", err)
	}
	if err := os.Rename(tmpPath, exePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to replace executable at %s: %w", exePath, err)
	}
	return nil
}

func extractBinaryFromTarGz(r io.Reader) ([]byte, error) {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read gzip archive: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, errors.New("openspend binary not found in archive")
			}
			return nil, fmt.Errorf("failed to read tar archive: %w", err)
		}

		if hdr.Typeflag == tar.TypeReg && filepath.Base(hdr.Name) == "openspend" {
			buf, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("failed to read openspend binary from archive: %w", err)
			}
			return buf, nil
		}
	}
}

func releaseAssetParts() (string, string, error) {
	var osPart string
	switch runtime.GOOS {
	case "darwin", "linux":
		osPart = runtime.GOOS
	default:
		return "", "", fmt.Errorf("unsupported OS for auto-update: %s", runtime.GOOS)
	}

	var archPart string
	switch runtime.GOARCH {
	case "amd64":
		archPart = "amd64"
	case "arm64":
		archPart = "arm64"
	default:
		return "", "", fmt.Errorf("unsupported architecture for auto-update: %s", runtime.GOARCH)
	}

	return osPart, archPart, nil
}
