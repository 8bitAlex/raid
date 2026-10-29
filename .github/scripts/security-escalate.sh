#!/usr/bin/env bash
# What happens to an eligible security PR whose CI did not go green
# (Lane 1 -> Lane 2):
#   - red, the change touches Go, and CLAUDE_FIX is `true`: the new version
#     probably needs a code change. Leaves a marker comment (proof, for
#     security-fix.yml, that this commit was assessed: "security-eligible")
#     and starts security-fix.yml for the PR. Claude tries once per PR.
#   - anything else (red without Go, CLAUDE_FIX off, a timeout): reports to
#     a person with security-report.sh (a comment and ONE issue).
#   - moved (the PR's head changed): nothing; the new commit starts a new run.
#
# Environment:
#   PR, SHA              the PR and the assessed head commit (required)
#   CI_STATE, CI_REASON  from security-wait-ci.sh
#   TOUCHES_GO           true when go.mod, go.sum or a .go file changed
#   CLAUDE_FIX           the repository variable SECURITY_CLAUDE_FIX
#   SECURITY_AUTOMERGE   must be `true`
#   RUN_URL, GITHUB_REPOSITORY, GH_TOKEN
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
pr=${PR:?PR}
sha=${SHA:?SHA}
state=${CI_STATE:?CI_STATE}
why=${CI_REASON:-}

[[ $pr =~ ^[0-9]{1,7}$ && $sha =~ ^[0-9a-f]{40}$ ]] || { echo "::error::bad PR or commit"; exit 1; }
[[ ${SECURITY_AUTOMERGE:-} == true ]] || { echo "kill switch off"; exit 0; }

case $state in
  moved)
    echo "the PR moved on: a new run will look at it"
    exit 0
    ;;
  red)
    if [[ ${CLAUDE_FIX:-} == true && ${TOUCHES_GO:-} == true ]]; then
      gh pr comment "$pr" --body "<!-- security-eligible:$sha -->
CI failed on this security update ($why). Asking Claude to adapt the code once (security-fix workflow). Nothing is merged unless the same checks pass again."
      gh workflow run security-fix.yml --ref "${BASE_BRANCH:-main}" -f "pr=$pr" -f "sha=$sha"
      out escalated claude
      summarize "### Security update: PR #$pr" "CI failed ($why); security-fix.yml started."
      exit 0
    fi
    reason="CI failed on this security update and it was not retried by Claude: $why"
    ;;
  timeout) reason="CI did not finish in time: $why" ;;
  *) reason="unexpected CI state $state: $why" ;;
esac

PR=$pr REASON=$reason bash "$here/security-report.sh"
out escalated person
