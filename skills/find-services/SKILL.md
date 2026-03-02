---
name: find-services
description: Find and shortlist third-party services using OpenSpend CLI marketplace search. Use when asked to discover providers for a capability, compare options, and return a justified recommendation while relying on login/policy guardrails for filtering.
---

# find-services

Use OpenSpend CLI to discover external services. Assume account/login policy automatically applies reliability and spending guardrails.

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
