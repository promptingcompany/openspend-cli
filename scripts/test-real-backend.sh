#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

OPENSPEND_BIN="${OPENSPEND_BIN:-${REPO_ROOT}/bin/openspend}"
OPENSPEND_MARKETPLACE_BASE_URL="${OPENSPEND_MARKETPLACE_BASE_URL:-https://openspend.ai}"
OPENSPEND_TEST_EMAIL="${OPENSPEND_TEST_EMAIL:-}"
OPENSPEND_TEST_PASSWORD="${OPENSPEND_TEST_PASSWORD:-}"
OPENSPEND_SESSION_TOKEN="${OPENSPEND_SESSION_TOKEN:-}"
OPENSPEND_SESSION_COOKIE="${OPENSPEND_SESSION_COOKIE:-}"
OPENSPEND_ALLOW_SIGNUP="${OPENSPEND_ALLOW_SIGNUP:-0}"
OPENSPEND_INTEGRATION_WRITE="${OPENSPEND_INTEGRATION_WRITE:-1}"

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "Missing required command: $1"
}

extract_cookie_pair() {
  local response="$1"
  printf '%s\n' "${response}" | tr -d '\r' | sed -nE 's/^set-cookie: ([^;]+).*/\1/ip' | \
    grep -E '^(better-auth\.session_token|better-auth-session_token|__Secure-better-auth\.session_token|__Secure-better-auth-session_token|__Host-better-auth\.session_token|__Host-better-auth-session_token)=' | \
    head -n 1 || true
}

escape_json() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  value="${value//$'\r'/\\r}"
  value="${value//$'\t'/\\t}"
  printf '%s' "${value}"
}

require_cmd curl
[[ -x "${OPENSPEND_BIN}" ]] || fail "CLI binary not found at ${OPENSPEND_BIN}. Run: make cli-build"

echo "[1/7] Checking marketplace endpoint at ${OPENSPEND_MARKETPLACE_BASE_URL}"
whoami_status="$(curl -sS -o /dev/null -w '%{http_code}' "${OPENSPEND_MARKETPLACE_BASE_URL}/api/cli/whoami" || true)"
if [[ "${whoami_status}" == "000" ]]; then
  fail "Marketplace is unreachable at ${OPENSPEND_MARKETPLACE_BASE_URL}"
fi
if [[ "${whoami_status}" != "401" && "${whoami_status}" != "200" ]]; then
  fail "Unexpected status from /api/cli/whoami: ${whoami_status}"
fi

session_token="${OPENSPEND_SESSION_TOKEN}"
session_cookie="${OPENSPEND_SESSION_COOKIE}"

if [[ -n "${session_token}" ]]; then
  echo "[2/7] Using provided OPENSPEND_SESSION_TOKEN"
  if [[ -z "${session_cookie}" ]]; then
    session_cookie="better-auth.session_token"
  fi
else
  [[ -n "${OPENSPEND_TEST_EMAIL}" ]] || fail "Set OPENSPEND_TEST_EMAIL or OPENSPEND_SESSION_TOKEN"
  [[ -n "${OPENSPEND_TEST_PASSWORD}" ]] || fail "Set OPENSPEND_TEST_PASSWORD or OPENSPEND_SESSION_TOKEN"

  if [[ "${OPENSPEND_ALLOW_SIGNUP}" == "1" ]]; then
    echo "[2/7] Ensuring test user exists (sign-up)"
    signup_name="CLI Integration User"
    signup_payload="$(printf '{"email":"%s","password":"%s","name":"%s"}' \
      "$(escape_json "${OPENSPEND_TEST_EMAIL}")" \
      "$(escape_json "${OPENSPEND_TEST_PASSWORD}")" \
      "$(escape_json "${signup_name}")")"
    # Ignore failure because user may already exist.
    curl -sS -o /dev/null -X POST "${OPENSPEND_MARKETPLACE_BASE_URL}/api/auth/sign-up/email" \
      -H "Content-Type: application/json" \
      --data "${signup_payload}" || true
  else
    echo "[2/7] Skipping sign-up (OPENSPEND_ALLOW_SIGNUP=${OPENSPEND_ALLOW_SIGNUP})"
  fi

  echo "[3/7] Signing in to obtain session cookie"
  signin_payload="$(printf '{"email":"%s","password":"%s","callbackURL":"/adm"}' \
    "$(escape_json "${OPENSPEND_TEST_EMAIL}")" \
    "$(escape_json "${OPENSPEND_TEST_PASSWORD}")")"
  signin_response="$(
    curl -sS -i -X POST "${OPENSPEND_MARKETPLACE_BASE_URL}/api/auth/sign-in/email" \
      -H "Content-Type: application/json" \
      --data "${signin_payload}"
  )"

  cookie_pair="$(extract_cookie_pair "${signin_response}")"
  if [[ -z "${cookie_pair}" ]]; then
    printf '%s\n' "${signin_response}" >&2
    fail "Could not capture session cookie from sign-in response"
  fi
  session_cookie="${cookie_pair%%=*}"
  session_token="${cookie_pair#*=}"
fi

echo "[4/7] Preparing isolated CLI HOME"
tmp_home="$(mktemp -d)"
trap 'rm -rf "${tmp_home}"' EXIT
mkdir -p "${tmp_home}/.config/openspend"
cat > "${tmp_home}/.config/openspend/config.toml" <<CFG
[marketplace]
base_url = "${OPENSPEND_MARKETPLACE_BASE_URL}"
whoami_path = "/api/cli/whoami"
policy_init_path = "/api/cli/policy/init"
agent_path = "/api/cli/agent"

[auth]
browser_login_path = "/api/cli/auth/login"
session_token = "${session_token}"
session_cookie = "${session_cookie}"
session_refresh_path = "/api/auth/get-session"
CFG

run_cli() {
  local output
  if ! output="$(HOME="${tmp_home}" "${OPENSPEND_BIN}" --base-url "${OPENSPEND_MARKETPLACE_BASE_URL}" "$@" 2>&1)"; then
    printf '%s\n' "${output}" >&2
    fail "Command failed: openspend $*"
  fi
  printf '%s\n' "${output}"
}

echo "[5/7] Verifying authenticated whoami"
whoami_output="$(run_cli whoami)"
printf '%s\n' "${whoami_output}" | grep -F 'User:' >/dev/null || {
  printf '%s\n' "${whoami_output}" >&2
  fail "whoami output missing user"
}

if [[ "${OPENSPEND_INTEGRATION_WRITE}" != "1" ]]; then
  echo "[6/7] Skipping write path (OPENSPEND_INTEGRATION_WRITE=${OPENSPEND_INTEGRATION_WRITE})"
  echo "[7/7] PASS: real backend authentication and read path verified at ${OPENSPEND_MARKETPLACE_BASE_URL}"
  exit 0
fi

run_id="$(date +%s)-$$"
policy_name="CLI Integration Policy ${run_id}"
agent_key="buyer-agent-int-${run_id}"
agent_name="CLI Integration Agent ${run_id}"

echo "[6/7] Running write-path checks (policy init + agent create)"
policy_output="$(run_cli policy init --buyer --name "${policy_name}" --description "CLI integration policy ${run_id}")"
printf '%s\n' "${policy_output}" | grep -Eq 'Buyer policy (created|updated):' || {
  printf '%s\n' "${policy_output}" >&2
  fail "policy init output did not match expected success format"
}

agent_output="$(run_cli agent create --external-key "${agent_key}" --display-name "${agent_name}")"
printf '%s\n' "${agent_output}" | grep -F "Agent subject ready:" >/dev/null || {
  printf '%s\n' "${agent_output}" >&2
  fail "agent create did not report success"
}
printf '%s\n' "${agent_output}" | grep -F "external_key=${agent_key}" >/dev/null || {
  printf '%s\n' "${agent_output}" >&2
  fail "agent create output missing expected external key"
}

echo "[7/7] Verifying created subject appears in whoami"
whoami_after="$(run_cli whoami)"
printf '%s\n' "${whoami_after}" | grep -F "key=${agent_key}" >/dev/null || {
  printf '%s\n' "${whoami_after}" >&2
  fail "whoami output missing newly created agent key"
}

echo "PASS: openspend-cli integration succeeded against ${OPENSPEND_MARKETPLACE_BASE_URL}"
