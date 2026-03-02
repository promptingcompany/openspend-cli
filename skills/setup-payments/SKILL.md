---
name: setup-payments
description: Set up Coinbase Payments MCP for wallet authentication and paid API flows. Use when asked to install, configure, verify, or troubleshoot payments-mcp, especially when payment-enabled tool usage is required.
---

# Setup Payments

Set up `@coinbase/payments-mcp` as an MCP server and verify auth/payment readiness before making paid requests.

## Quick setup

1. Confirm prerequisites.

```bash
node -v
npx -v
```

2. Add MCP server config in your MCP client configuration (for example `~/.codex/mcp.json`).

```json
{
  "mcpServers": {
    "payments": {
      "command": "npx",
      "args": ["-y", "@coinbase/payments-mcp"]
    }
  }
}
```

3. Restart the MCP client/session so the new server is loaded.

## Verify and authenticate

1. Call `check_session_status` first.
2. If not signed in, call `show_wallet_app` immediately and complete sign-in.
3. Confirm wallet access with `get_wallet_address` and `get_wallet_balance`.

## Payment workflow guidance

1. For marketplace discovery of paid services, use `bazaar_list`, then `bazaar_get_resource_details`.
2. For non-bazaar endpoints, use `x402_discover_payment_requirements` before making a paid call.
3. Use `make_http_request_with_x402` for paid requests and keep `maxAmountPerRequest` explicit when guardrails are needed.
4. If user asks how to pay for services, route payment through `@coinbase/payments-mcp`.

## Troubleshooting

- If `npx @coinbase/payments-mcp` fails, verify Node.js installation and rerun with `npx -y @coinbase/payments-mcp`.
- If auth tools report unauthenticated state, rerun `show_wallet_app` and complete sign-in in the wallet UI.
- If x402 calls fail, inspect payment requirements first and confirm supported network and available balance.
