#!/usr/bin/env bash
# Checks, before Claude runs, that Claude will be able to authenticate: the
# first step of every job that runs claude-code-action. Claude authenticates
# by Workload Identity Federation (GitHub Actions OIDC -> Anthropic), with
# no API key or OAuth token secret:
#   1. the federation identifiers are set (repository variables
#      ANTHROPIC_FEDERATION_RULE_ID and ANTHROPIC_ORGANIZATION_ID; optional
#      ANTHROPIC_SERVICE_ACCOUNT_ID and ANTHROPIC_WORKSPACE_ID);
#   2. GitHub issues this job an OIDC token (the job needs
#      `permissions: id-token: write`) for the audience OIDC_AUDIENCE
#      (default https://api.anthropic.com), from
#      $ACTIONS_ID_TOKEN_REQUEST_URL with $ACTIONS_ID_TOKEN_REQUEST_TOKEN;
#   3. Anthropic exchanges it: POST https://api.anthropic.com/v1/oauth/token
#      with grant_type urn:ietf:params:oauth:grant-type:jwt-bearer, the token
#      as assertion, and the identifiers. A 200 with an access_token passes.
# The access token is thrown away (claude-code-action fetches its own OIDC
# token and the CLI exchanges that; each GitHub token is single-use), and
# neither token is printed. The token's claims (sub, ref, job_workflow_ref)
# are printed, since the federation rule must match them. Any failure exits
# 1 with what to fix, and writes reason= to GITHUB_OUTPUT.
#
# Environment:
#   ANTHROPIC_FEDERATION_RULE_ID, ANTHROPIC_ORGANIZATION_ID (required)
#   ANTHROPIC_SERVICE_ACCOUNT_ID, ANTHROPIC_WORKSPACE_ID (optional)
#   OIDC_AUDIENCE    default https://api.anthropic.com
#   ANTHROPIC_TOKEN_URL  default https://api.anthropic.com/v1/oauth/token
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

audience=${OIDC_AUDIENCE:-https://api.anthropic.com}
token_url=${ANTHROPIC_TOKEN_URL:-https://api.anthropic.com/v1/oauth/token}
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

fail() {
  echo "::error::Claude cannot authenticate: $1"
  summarize "### Claude cannot authenticate" "$1"
  out reason "Claude cannot authenticate: $1"
  exit 1
}

# A field of a JSON file ($1), read with node (on every runner; jq is not
# on every machine the tests run on).
json_field() {
  node -e '
    const fs = require("fs");
    try {
      const v = JSON.parse(fs.readFileSync(process.argv[1], "utf8"))[process.argv[2]];
      if (v !== undefined && v !== null) process.stdout.write(typeof v === "string" ? v : JSON.stringify(v));
    } catch (e) {}
  ' "$1" "$2"
}

# 1. The identifiers, and GitHub's OIDC endpoint.
require_setup() {
  local missing=()
  [[ -n ${ANTHROPIC_FEDERATION_RULE_ID:-} ]] || missing+=(ANTHROPIC_FEDERATION_RULE_ID)
  [[ -n ${ANTHROPIC_ORGANIZATION_ID:-} ]] || missing+=(ANTHROPIC_ORGANIZATION_ID)
  if ((${#missing[@]})); then
    fail "the repository variable(s) ${missing[*]} are not set. Set them (Settings > Secrets and variables > Actions > Variables, or \`gh variable set NAME --body VALUE\`) to the federation rule's ID (fdrl_...) and the Anthropic organization's ID (a UUID), from the Claude Console's Workload identity page; ANTHROPIC_SERVICE_ACCOUNT_ID (svac_...) and ANTHROPIC_WORKSPACE_ID (wrkspc_...) as well if the rule needs them. See docs/AUTOMATION.md, \"Claude's credentials\"."
  fi
  if [[ -z ${ACTIONS_ID_TOKEN_REQUEST_URL:-} || -z ${ACTIONS_ID_TOKEN_REQUEST_TOKEN:-} ]]; then
    fail "GitHub offers this job no OIDC token: the job needs \`permissions: id-token: write\`."
  fi
}

# 2. The GitHub OIDC token for the audience, masked in the log. Sets jwt.
github_oidc_token() {
  local status
  status=$(curl -sS -o "$work/oidc.json" -w '%{http_code}' \
    -H "Authorization: Bearer $ACTIONS_ID_TOKEN_REQUEST_TOKEN" \
    --get --data-urlencode "audience=$audience" "$ACTIONS_ID_TOKEN_REQUEST_URL") || status=000
  jwt=$(json_field "$work/oidc.json" value)
  rm -f "$work/oidc.json"
  if [[ $status != 200 || -z $jwt ]]; then
    fail "GitHub refused the OIDC token request (HTTP $status). Check that the job has \`permissions: id-token: write\`."
  fi
  echo "::add-mask::$jwt"
}

# The claims of the token in $jwt that the federation rule matches (the
# payload is not secret).
token_claims() {
  cut -d. -f2 <<<"$jwt" | node -e '
    let s = require("fs").readFileSync(0, "utf8").trim().replace(/-/g, "+").replace(/_/g, "/");
    while (s.length % 4) s += "=";
    try {
      const c = JSON.parse(Buffer.from(s, "base64").toString("utf8"));
      console.log(`sub=${c.sub} aud=${c.aud} ref=${c.ref} repository_owner=${c.repository_owner} job_workflow_ref=${c.job_workflow_ref}`);
    } catch (e) { console.log("(claims unreadable)"); }
  '
}

# 3. The exchange at Anthropic. Sets status (HTTP) and leaves the response
# in $work/response.json; the token is forgotten.
exchange_token() {
  ASSERTION=$jwt node -e '
    const body = {
      grant_type: "urn:ietf:params:oauth:grant-type:jwt-bearer",
      assertion: process.env.ASSERTION,
      federation_rule_id: process.env.ANTHROPIC_FEDERATION_RULE_ID,
      organization_id: process.env.ANTHROPIC_ORGANIZATION_ID,
    };
    if (process.env.ANTHROPIC_SERVICE_ACCOUNT_ID) body.service_account_id = process.env.ANTHROPIC_SERVICE_ACCOUNT_ID;
    if (process.env.ANTHROPIC_WORKSPACE_ID) body.workspace_id = process.env.ANTHROPIC_WORKSPACE_ID;
    require("fs").writeFileSync(process.argv[1], JSON.stringify(body));
  ' "$work/request.json"
  unset jwt
  status=$(curl -sS -o "$work/response.json" -w '%{http_code}' -X POST "$token_url" \
    -H "content-type: application/json" --data-binary "@$work/request.json") || status=000
  rm -f "$work/request.json"
}

require_setup
github_oidc_token
claims=$(token_claims)
echo "GitHub OIDC token for $audience: $claims"
exchange_token
if [[ $status == 200 && -n $(json_field "$work/response.json" access_token) ]]; then
  echo "Anthropic accepted the OIDC token (federation rule $ANTHROPIC_FEDERATION_RULE_ID); Claude can authenticate"
  out subject "$(sed -n 's/^sub=\([^ ]*\).*/\1/p' <<<"$claims")"
  exit 0
fi
error=$(json_field "$work/response.json" error)
fail "Anthropic refused the OIDC token exchange (HTTP $status${error:+, $error}) for federation rule $ANTHROPIC_FEDERATION_RULE_ID. The token presented: $claims. Check that the rule's subject and audience match these claims, and the rule, organization and service account IDs; the Claude Console's Workload identity > authentication history shows the exact reason."
