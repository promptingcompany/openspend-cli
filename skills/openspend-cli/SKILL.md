---
name: using-openspend-cli
description: Guides end-to-end setup and usage of openspend-cli. Use when asked to install openspend via curl or Homebrew, authenticate, verify identity, initialize dashboard policy, create or update dashboard agents, run search, or update the CLI.
---

# Using openspend-cli

Install `openspend` with one of these methods.

Method 1 (`curl` installer):

```bash
curl -fsSL https://openspend.ai/install | sh
```

Method 2 (`homebrew`):

```bash
brew install promptingcompany/tap/openspend
```

Verify install:

```bash
openspend version
```

Authenticate:

```bash
openspend auth login
openspend whoami
```

Set up dashboard policy and agent:

```bash
openspend dashboard policy init --buyer
openspend dashboard agent create --external-key buyer-agent-1 --display-name "Buyer Agent"
```

Update an existing agent:

```bash
openspend dashboard agent update --external-key buyer-agent-1 --display-name "Buyer Agent v2"
```

Search services:

```bash
openspend search "stable diffusion image generation" --limit 5
```

Update CLI:

```bash
openspend update
```

## Useful auth flags

- Use `openspend auth login -y` to open the browser without prompt.
- Use `openspend auth login -n` to skip browser launch and copy the login URL manually.
- Use `--callback-host <host>` in sandboxed or remote-browser environments where localhost callback is not reachable.

## Config path

Session/config file:

```text
~/.config/openspend/config.toml
```
