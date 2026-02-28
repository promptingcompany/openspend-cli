---
name: using-openspend-cli
description: Guides installation, authentication, and buyer onboarding with openspend-cli. Use when asked how to install openspend, log in, initialize buyer policy, create a buyer agent, run buyer onboarding, check identity, or update the CLI.
---

# Using openspend-cli

Run the installer script first:

```bash
bash scripts/install.sh
```

Then run the default onboarding workflow:

```bash
openspend auth login
openspend policy init --buyer
openspend agent create --external-key buyer-agent-1 --display-name "Buyer Agent"
openspend onboarding buyer-quickstart
openspend whoami
```

## Update

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
