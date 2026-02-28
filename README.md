# openspend-cli

Go-based CLI for OpenSpend marketplace onboarding and buyer setup.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/promptingcompany/openspend-cli/main/install.sh | sh
```

This installer currently builds from source using `go install`.

### Homebrew

```bash
brew install promptingcompany/tap/openspend
```

## Release automation (maintainers)

- Homebrew publishing is handled by GoReleaser config in `.goreleaser.yaml`.
- GitHub Action `.github/workflows/release.yaml` runs on tags matching `v*`.
- Required repository secret:
  - `HOMEBREW_TAP_GITHUB_TOKEN`: PAT with write access to `promptingcompany/homebrew-tap`.

### Cutting a release candidate

```bash
git tag v0.1.0-rc.1
git push origin v0.1.0-rc.1
```

This triggers the CLI release workflow, publishes binaries, and updates `promptingcompany/homebrew-tap`.

## Commands

- `openspend auth login`
- `openspend policy init --buyer`
- `openspend agent create --external-key buyer-agent-1 --display-name "Buyer Agent"`
- `openspend onboarding buyer-quickstart`
- `openspend whoami`

## Notes

- `openspend auth login` opens the marketplace sign-in page in your browser and captures a local callback.
- `openspend auth login` asks `Open login page in your browser now? (Y/n)` before opening.
- Use `-y` to open without prompt, or `-n` to skip opening and copy URL manually.
- For automated/sandbox browser flows, set `--callback-host` (for example `192.0.0.2`) so callback is reachable.
- CLI stores settings and session token in `~/.config/openspend/config.toml` (TOML codec).
- Default marketplace URL: `http://localhost:5555`.
- Override per command with `--base-url`.

## Config

```toml
[marketplace]
base_url = "http://localhost:5555"
whoami_path = "/api/cli/whoami"
policy_init_path = "/api/cli/policy/init"
agent_path = "/api/cli/agent"

[auth]
browser_login_path = "/api/cli/auth/login"
session_cookie = "better-auth.session_token"
session_token = ""
```
