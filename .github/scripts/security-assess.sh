#!/usr/bin/env bash
# Decides whether a PR may be merged by the security automation, from what
# the GitHub API says about it. It never checks out or runs anything from the
# PR: the PR's manifests are read through the contents API and only parsed
# (security-policy.mjs). Read-only; writes only the step outputs.
#
# It checks, and stops at the first thing that fails:
#   - the kill switch SECURITY_AUTOMERGE is `true`;
#   - the PR is open, not a draft, into the default branch, from a branch of
#     this repository that Dependabot (dependabot/...) or this automation
#     (security/DATE) made, by that bot;
#   - every commit is by Dependabot, or this automation with a known subject
#     (mode `deps`, or `fix` when a fix of Claude's is on top);
#   - the files are all on the allowlist of the mode: only go.mod, go.sum,
#     site/package.json and site/package-lock.json, plus (mode `fix`) src/,
#     main.go, site/docs/, README.md and llms.txt; never .github/; and the
#     size is within limits;
#   - with CLASSIFY=yes (Dependabot's PRs, with UPDATED_JSON from
#     dependabot/fetch-metadata): it is a SECURITY update, either because
#     Dependabot's alert for it is OPEN (when the optional alert token was
#     given) or because a known advisory (OSV) that affects the old version
#     no longer affects the new one; and no direct dependency goes up a
#     semver-major version;
#   - the manifests, against the merge base: no new direct dependency, at
#     most 5 new indirect ones, nothing that is not from the registry, no
#     changed go/toolchain/replace directive, no changed package.json script;
#     Lane 3's own PRs also may not move any package to another major;
#   - when src/ changes: src/resources/app.properties has a new, higher
#     version whose tag does not exist yet (what CI's version-check job needs
#     on a pull_request, and which a dispatched CI run does not check).
#
# Outputs: eligible=true|false, reason=, head_sha=, head_ref=, mode=,
# required= (the check runs CI must pass), touches_src=, touches_site=,
# touches_go=, advisories=.
#
# Environment:
#   PR                the pull request number (required)
#   GITHUB_REPOSITORY, GH_TOKEN
#   SECURITY_AUTOMERGE  must be `true`
#   BASE_BRANCH       the default branch, default main
#   CLASSIFY          yes (default) or no: whether to decide that the PR is a
#                     security update (Lane 2 and 3 skip it: the PR was
#                     assessed, or made by the automation, before)
#   UPDATED_JSON      fetch-metadata's updated-dependencies-json
#   EXPECT_SHA        the head commit the caller assessed or pushed: stop if
#                     the PR's head is another
#   OSV_URL           default https://api.osv.dev/v1/query
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
pol() { node "$here/security-policy.mjs" "$@"; }
pr=${PR:?PR}
repo=${GITHUB_REPOSITORY:?GITHUB_REPOSITORY}
base=${BASE_BRANCH:-main}
classify=${CLASSIFY:-yes}
osv_url=${OSV_URL:-https://api.osv.dev/v1/query}
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

stop() {
  echo "not eligible: $1"
  out eligible false
  out reason "$1"
  summarize "### Security update: PR #$pr not eligible" "$1"
  exit 0
}

# Runs the policy command, its answer in RES; on a refusal stops with every
# reason. (Not in a command substitution: stop must end this script.)
RES=
must() {
  if ! RES=$(pol "$@" 2>"$work/err"); then
    stop "$(sed 's/^reject: //' "$work/err" | paste -sd ';' - | sed 's/;/; /g')"
  fi
}
get() { sed -n "s/^$1=//p" <<<"$2" | head -n 1; }

[[ ${SECURITY_AUTOMERGE:-} == true ]] || stop "the kill switch is off: the repository variable SECURITY_AUTOMERGE is not \`true\`"
[[ $pr =~ ^[0-9]{1,7}$ ]] || { echo "::error::not a PR number: $pr"; exit 1; }

# --- The PR -----------------------------------------------------------------
gh api "repos/$repo/pulls/$pr" >"$work/pr.json"
must pr "$repo" "$base" <"$work/pr.json"
info=$RES
author=$(get author "$info")
head_ref=$(get head_ref "$info")
head_sha=$(get head_sha "$info")
if [[ -n ${EXPECT_SHA:-} && $head_sha != "$EXPECT_SHA" ]]; then
  stop "the PR's head moved on to ${head_sha:0:8} (this run looked at ${EXPECT_SHA:0:8})"
fi
if [[ $classify == yes && $author != "dependabot[bot]" ]]; then
  stop "the PR is not Dependabot's"
fi

# --- Commits and files ------------------------------------------------------
gh api --paginate "repos/$repo/pulls/$pr/commits?per_page=100" --jq '.[]' >"$work/commits.json"
must commits "$author" <"$work/commits.json"
mode=$(get mode "$RES")
gh api --paginate "repos/$repo/pulls/$pr/files?per_page=100" --jq '.[]' >"$work/files.json"
must files "$mode" <"$work/files.json"
files=$RES
required=$(get required "$files")
touches_src=$(get touches_src "$files")

# --- Is it a security update? -----------------------------------------------
advisories=
if [[ $classify == yes ]]; then
  [[ -n ${UPDATED_JSON:-} ]] || stop "Dependabot's metadata is empty: not a dependency update"
  must deps <<<"$UPDATED_JSON"
  deps=$RES
  while IFS=$'\t' read -r i eco name old new _type _update; do
    for which in old new; do
      version=$old
      [[ $which == new ]] && version=$new
      # Names and versions were validated: nothing here can break the JSON.
      body=$(printf '{"package":{"name":"%s","ecosystem":"%s"},"version":"%s"}' "$name" "$eco" "$version")
      if ! curl -sS --fail --max-time 30 -X POST -H 'content-type: application/json' --data "$body" \
        -o "$work/osv-$i-$which.json" "$osv_url" 2>"$work/curl.err"; then
        echo "::warning::OSV could not be asked about $name $version"
        rm -f "$work/osv-$i-$which.json"
      fi
    done
  done <<<"$deps"
  verdict=$(pol classify "$work" <<<"$UPDATED_JSON" 2>"$work/err") || stop "$(sed 's/^reject: //' "$work/err" | paste -sd ';' -)"
  advisories=$(get advisories "$verdict")
  [[ $(get eligible "$verdict") == true ]] || stop "$(get reason "$verdict")"
fi

# --- The manifests, against the merge base ----------------------------------
mb=$(gh api "repos/$repo/compare/$base...$head_sha" --jq .merge_base_commit.sha)
[[ $mb =~ ^[0-9a-f]{40}$ ]] || stop "the merge base with $base cannot be found"
fetch_at() { # sha path dir
  mkdir -p "$3/$(dirname "$2")"
  gh api -H "Accept: application/vnd.github.raw" "repos/$repo/contents/$2?ref=$1" >"$3/$2" 2>/dev/null || rm -f "$3/$2"
}
for f in go.mod site/package.json site/package-lock.json src/resources/app.properties; do
  fetch_at "$mb" "$f" "$work/base"
  fetch_at "$head_sha" "$f" "$work/head"
done
flags=()
[[ $author == github-actions* ]] && flags+=(--no-major)
must manifests "$work/base" "$work/head" ${flags[@]+"${flags[@]}"}
changes=$RES

# --- The version, when the code changes -------------------------------------
if [[ $touches_src == true ]]; then
  base_v=$(app_version "$work/base/src/resources/app.properties")
  head_v=$(app_version "$work/head/src/resources/app.properties")
  [[ $head_v =~ ^[0-9]+\.[0-9]+\.[0-9]+$ && $base_v =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || stop "src/ changes but app.properties has no readable version"
  [[ $head_v != "$base_v" && $(printf '%s\n%s\n' "$base_v" "$head_v" | sort -V | tail -n 1) == "$head_v" ]] \
    || stop "src/ changes, so src/resources/app.properties needs a higher version than $base_v (CI's version-check)"
  if gh api "repos/$repo/git/ref/tags/v$head_v" >/dev/null 2>&1; then
    stop "src/ changes, but the tag v$head_v exists already (CI's version-check would fail)"
  fi
fi

out eligible true
out head_sha "$head_sha"
out head_ref "$head_ref"
out mode "$mode"
out required "$required"
out touches_src "$touches_src"
out touches_site "$(get touches_site "$files")"
out touches_go "$(get touches_go "$files")"
out advisories "$advisories"
summarize "### Security update: PR #$pr eligible" "$(
  echo "- mode \`$mode\`, head \`${head_sha:0:8}\`, checks to pass: $required"
  [[ -z $advisories ]] || echo "- advisories fixed: $advisories"
  echo "- changes: $(grep -c . <<<"$changes" || true)"
)"
