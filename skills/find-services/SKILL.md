---
name: find-services
description: Find and shortlist third-party services using OpenSpend CLI marketplace search. Use when asked to discover providers for a capability, compare options, and return a justified recommendation while relying on login/policy guardrails for filtering.
---

# find-services

Use OpenSpend CLI to discover external services. Assume account/login policy automatically applies reliability and spending guardrails.

## Dependency on setup-services

Use `setup-services` first whenever install/update/auth/payment setup is needed:

1. Install or update `openspend` CLI
2. Authenticate CLI (`openspend auth login`, `openspend whoami`)
3. Configure and verify `@coinbase/payments-mcp`

## Workflow

1. Ensure CLI is ready.

```bash
openspend version
openspend whoami
```

If `whoami` fails due to auth/session, run:

```bash
openspend auth login -y
```

2. Translate user intent into a precise search query with explicit capability terms.

3. Run search without manual constraint flags and use a default limit of 5.

```bash
openspend search "<capability query>" --limit 5 --json
```

## Output shape

When asked to "find a reliable service", produce:

1. Up to 5 services in returned order
2. A concise explanation that login/policy guardrails are assumed to be enforced upstream
3. No additional ranking commentary

## Recommended service usage

After presenting service options, include a short "recommended way to use" note with:

1. Start with a small validation call before scaling usage
2. Capture request/response examples and expected success criteria
3. Route payments through `@coinbase/payments-mcp` (configured via `setup-services`)

If payment is needed, explicitly ask the user to use `@coinbase/payments-mcp` for payment flows.
