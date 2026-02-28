# openspend-cli

Go-based CLI for OpenSpend marketplace onboarding and buyer setup.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/promptingcompany/openspend-cli/main/install.sh | sh
```

The installer downloads the latest GitHub release binary for your OS/arch by
default, and falls back to a source build only if binary download fails.

### Update Existing Install

```bash
curl -fsSL https://raw.githubusercontent.com/promptingcompany/openspend-cli/main/update.sh | sh
```

The updater installs into the same directory as your current `openspend`
binary when possible, then replaces it with the requested version.

### Install/Update Options

- `OPENSPEND_VERSION`:
  - `latest` (default), or a specific tag/version like `v0.1.0-rc.10` / `0.1.0-rc.10`
- `OPENSPEND_INSTALL_BIN_DIR`:
  - install/update target directory (default `~/.local/bin`)
- `OPENSPEND_REPO_SLUG`:
  - GitHub repo slug used for release downloads (default `promptingcompany/openspend-cli`)

Examples:

```bash
OPENSPEND_VERSION=v0.1.0-rc.10 curl -fsSL https://raw.githubusercontent.com/promptingcompany/openspend-cli/main/install.sh | sh
OPENSPEND_INSTALL_BIN_DIR="$HOME/bin" curl -fsSL https://raw.githubusercontent.com/promptingcompany/openspend-cli/main/update.sh | sh
```

### Homebrew

```bash
brew install promptingcompany/tap/openspend
```

If `openspend` is not found after install, add this to your shell profile:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

## Release automation (maintainers)

- GitHub Action `.github/workflows/release.yaml` runs when a GitHub release is published.

### Cutting a release candidate

```bash
git tag v0.1.0-rc.1
git push origin v0.1.0-rc.1
```

Publish a GitHub release for the tag to trigger binary build/upload automation.

## Commands

- `openspend auth login`
- `openspend policy init --buyer`
- `openspend agent create --external-key buyer-agent-1 --display-name "Buyer Agent"`
- `openspend onboarding buyer-quickstart`
- `openspend whoami`
- `openspend update`

## Notes

- `openspend auth login` opens the marketplace sign-in page in your browser and captures a local callback.
- `openspend auth login` asks `Open login page in your browser now? (Y/n)` before opening.
- Use `-y` to open without prompt, or `-n` to skip opening and copy URL manually.
- For automated/sandbox browser flows, set `--callback-host` (for example `192.0.0.2`) so callback is reachable.
- CLI stores settings and session token in `~/.config/openspend/config.toml` (TOML codec).
- Default marketplace URL: `https://openspend.ai`.
- Override per command with `--base-url`.
- Runtime env overrides:
  - `OPENSPEND_MARKETPLACE_BASE_URL` (or legacy `OPENSPEND_BASE_URL`)
  - `OPENSPEND_MARKETPLACE_WHOAMI_PATH`
  - `OPENSPEND_MARKETPLACE_POLICY_INIT_PATH`
  - `OPENSPEND_MARKETPLACE_AGENT_PATH`
  - `OPENSPEND_AUTH_BROWSER_LOGIN_PATH`
  - `OPENSPEND_AUTH_SESSION_COOKIE`

## Config

```toml
[marketplace]
base_url = "https://openspend.ai"
whoami_path = "/api/cli/whoami"
policy_init_path = "/api/cli/policy/init"
agent_path = "/api/cli/agent"

[auth]
browser_login_path = "/api/cli/auth/login"
session_cookie = "better-auth.session_token"
session_token = ""
```
