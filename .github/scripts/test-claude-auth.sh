#!/usr/bin/env bash
# Tests claude-auth.sh (the pre-flight check of Claude's federation) with a
# fake `curl`: the identifiers missing, no id-token permission, GitHub
# refusing the token, Anthropic refusing or accepting the exchange, and that
# neither token is ever printed.
#
#   bash .github/scripts/test-claude-auth.sh
set -euo pipefail
source "$(dirname "$0")/test-lib.sh"
isolate_git
mkdir -p "$tmp/work"

payload=$(node -e '
  const c = { sub: "repo:8bitAlex/raid:ref:refs/heads/main", aud: "https://api.anthropic.com", ref: "refs/heads/main",
    repository_owner: "8bitAlex", job_workflow_ref: "8bitAlex/raid/.github/workflows/triage.yml@refs/heads/main" };
  process.stdout.write(Buffer.from(JSON.stringify(c)).toString("base64url"));
')
export FAKE_JWT="eyJhbGciOiJSUzI1NiJ9.$payload.SECRETSIGNATURE"

# The fake curl: -o FILE gets the body, -w prints the HTTP status. A GET
# (--get) is GitHub's OIDC endpoint, a POST is Anthropic's token exchange.
cat >"$tmp/bin/curl" <<'EOF'
#!/usr/bin/env bash
out=; get=no
while (($#)); do
  case $1 in
    -o) out=$2; shift ;;
    --get) get=yes ;;
  esac
  shift
done
if [[ $get == yes ]]; then
  echo "oidc" >>"$FAKE_CALLS"
  [[ ${FAKE_OIDC_STATUS:-200} == 200 ]] && echo "{\"value\":\"$FAKE_JWT\"}" >"$out" || echo '{}' >"$out"
  printf '%s' "${FAKE_OIDC_STATUS:-200}"
else
  echo "exchange" >>"$FAKE_CALLS"
  if [[ ${FAKE_EXCHANGE_STATUS:-200} == 200 ]]; then
    echo '{"access_token":"ACCESSSECRET","expires_in":300}' >"$out"
  else
    echo '{"error":"invalid_grant"}' >"$out"
  fi
  printf '%s' "${FAKE_EXCHANGE_STATUS:-200}"
fi
EOF
chmod +x "$tmp/bin/curl"
export FAKE_CALLS=$tmp/calls

ok_env=(ANTHROPIC_FEDERATION_RULE_ID=fdrl_x ANTHROPIC_ORGANIZATION_ID=org-1 ACTIONS_ID_TOKEN_REQUEST_URL=https://example.test/oidc ACTIONS_ID_TOKEN_REQUEST_TOKEN=reqtoken)
auth() {
  : >"$FAKE_CALLS"
  run claude-auth.sh "${ok_env[@]}" "$@"
}

# How many lines of the log show a token (the add-mask command is how the
# runner is told to hide it, so it is not counted).
leaked() {
  grep -v '^::add-mask::' "$tmp/log" | grep -c -e SECRETSIGNATURE -e ACCESSSECRET || true
}

echo "== everything in place"
auth
check "exit" "$code" 0
contains "says it can authenticate" "$(cat "$tmp/log")" "Claude can authenticate"
contains "prints the subject" "$(cat "$tmp/log")" "sub=repo:8bitAlex/raid:ref:refs/heads/main"
contains "prints the job workflow ref" "$(cat "$tmp/log")" "job_workflow_ref=8bitAlex/raid/.github/workflows/triage.yml@refs/heads/main"
check "subject output" "$(out_value subject)" "repo:8bitAlex/raid:ref:refs/heads/main"
check "token and access token never printed" "$(leaked)" 0
check "the token is masked" "$(grep -c '^::add-mask::' "$tmp/log" || true)" 1
check "one OIDC request, one exchange" "$(tr '\n' ' ' <"$FAKE_CALLS")" "oidc exchange "

echo "== the identifiers are missing"
run claude-auth.sh ACTIONS_ID_TOKEN_REQUEST_URL=u ACTIONS_ID_TOKEN_REQUEST_TOKEN=t ANTHROPIC_FEDERATION_RULE_ID= ANTHROPIC_ORGANIZATION_ID=
check "exit" "$code" 1
contains "names the variables" "$(cat "$tmp/log")" "ANTHROPIC_FEDERATION_RULE_ID ANTHROPIC_ORGANIZATION_ID"
contains "points at the docs" "$(cat "$tmp/log")" "docs/AUTOMATION.md"
contains "reason output" "$(out_value reason)" "Claude cannot authenticate"

echo "== the job has no id-token permission"
run claude-auth.sh ANTHROPIC_FEDERATION_RULE_ID=fdrl_x ANTHROPIC_ORGANIZATION_ID=org-1
check "exit" "$code" 1
contains "says what to add" "$(cat "$tmp/log")" "id-token: write"

echo "== GitHub refuses the token"
auth FAKE_OIDC_STATUS=403
check "exit" "$code" 1
contains "says so" "$(cat "$tmp/log")" "GitHub refused the OIDC token request (HTTP 403)"
check "no exchange" "$(grep -c exchange "$FAKE_CALLS" || true)" 0

echo "== Anthropic refuses the exchange"
auth FAKE_EXCHANGE_STATUS=401
check "exit" "$code" 1
contains "says so" "$(cat "$tmp/log")" "Anthropic refused the OIDC token exchange (HTTP 401, invalid_grant)"
contains "shows the claims presented" "$(cat "$tmp/log")" "sub=repo:8bitAlex/raid:ref:refs/heads/main"
check "tokens never printed" "$(leaked)" 0

finish
