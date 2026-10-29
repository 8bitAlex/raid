// The decisions of the security-update workflows, as pure functions on JSON
// that the shell scripts (security-*.sh) fetch with `gh` and `curl`. No
// network, no writes: reads stdin and files, prints key=value lines (or
// lists) on stdout, and every reason a PR or a patch is refused as a
// `reject: ...` line on stderr with exit 1. Nothing here trusts its input:
// package names, versions, paths and logins are checked against strict
// patterns before the scripts use them in a command, a path or a branch.
//
//   deps                       updated-dependencies-json (dependabot/fetch-
//                              metadata, stdin) -> validated TSV lines
//                              `index ecosystem name old new type update`
//   classify DIR               the same JSON (stdin) + the OSV answers in DIR
//                              (osv-<i>-old.json, osv-<i>-new.json) ->
//                              security=, eligible=, reason=
//   pr REPO BASE               the PR of the REST API (stdin): open, not a
//                              draft, into BASE, from REPO, by a bot we know
//                              on a branch it makes -> author=, head_ref=,
//                              head_sha=
//   commits AUTHOR             the PR's commits (stdin) -> mode=deps|fix
//   files MODE                 the PR's files (stdin) -> touches_src=,
//                              touches_site=, touches_go=, required=
//   manifests BASE HEAD [--no-major]
//                              the manifests of two checkouts -> the
//                              changes as `change: name old new` lines
//   ci REQUIRED WATCH          check runs (NDJSON file) + commit statuses
//     RUNS STATUSES            (JSON file) -> ci=green|red|wait, reason=
//   audit                      `npm audit --json` (stdin) -> fixable list
//   vuln                       `govulncheck -json` (stdin) -> fixable list
//   patch-files                `git apply --numstat --summary` of a patch
//     MODE                     (stdin) -> the same checks as `files`
//
// MODE is `deps` (only the dependency manifests may change) or `fix` (also
// the code, tests and docs of a fix that adapts to an update; never the
// manifests, never anything under .github/).
import fs from 'node:fs';
import path from 'node:path';

const MANIFESTS = ['go.mod', 'go.sum', 'site/package.json', 'site/package-lock.json'];
const FIX_PATTERNS = [/^src\/.+/, /^main\.go$/, /^site\/docs\/.+\.mdx?$/, /^README\.md$/, /^llms\.txt$/];
const LIMITS = {
  deps: { lines: 8000, files: 4 },
  fix: { lines: 600, files: 20, srcOnlyLines: 600 },
  newTransitive: 5,
  deps_per_pr: 25,
};
const ECOSYSTEMS = { go_modules: 'Go', npm_and_yarn: 'npm' };
const SAFE_PATH = /^[A-Za-z0-9_.@+\/-]+$/;
const GO_MODULE = /^[A-Za-z0-9._~\/+-]{1,200}$/;
const NPM_NAME = /^(@[a-z0-9._~-]{1,100}\/)?[a-z0-9._~-]{1,100}$/;
const VERSION = /^v?\d{1,6}(\.\d{1,6}){0,3}([-+][0-9A-Za-z.+-]{1,60})?$/;
const UPDATE_TYPES = ['version-update:semver-major', 'version-update:semver-minor', 'version-update:semver-patch'];
const BAD_CONCLUSIONS = ['failure', 'timed_out', 'action_required', 'startup_failure', 'stale'];

const reject = [];
const bad = (msg) => reject.push(msg);
const done = (lines) => {
  if (reject.length) {
    for (const r of reject) console.error(`reject: ${r}`);
    process.exit(1);
  }
  if (lines.length) process.stdout.write(lines.join('\n') + '\n');
};
const stdin = () => {
  const t = fs.readFileSync(0, 'utf8');
  return t.charCodeAt(0) === 0xfeff ? t.slice(1) : t;
};
const parse = (text, what) => {
  try {
    return JSON.parse(text);
  } catch {
    bad(`${what} is not JSON`);
    return null;
  }
};
/** A list of JSON records, as a JSON array (or arrays of pages) or one per line. */
const records = (text, what) => {
  const trimmed = text.trim();
  if (trimmed === '') return [];
  if (trimmed.startsWith('[')) {
    const v = parse(trimmed, what);
    return Array.isArray(v) ? v.flat() : [];
  }
  const out = [];
  for (const line of trimmed.split('\n')) {
    if (!line.trim()) continue;
    try {
      out.push(JSON.parse(line));
    } catch {
      bad(`${what} has a line that is not JSON`);
    }
  }
  return out;
};
const major = (v) => Number(/^v?(\d+)/.exec(String(v))?.[1] ?? NaN);

// ---------------------------------------------------------------------------
// Dependabot's metadata: which packages, from which version to which.

/** The validated list of updated dependencies of a Dependabot PR. */
function validDeps(json) {
  const list = typeof json === 'string' ? parse(json, 'the dependency metadata') : json;
  if (list === null) return [];
  if (!Array.isArray(list) || list.length === 0) {
    bad('the dependency metadata is empty (not a Dependabot dependency update)');
    return [];
  }
  if (list.length > LIMITS.deps_per_pr) bad(`the PR updates ${list.length} dependencies, more than ${LIMITS.deps_per_pr}`);
  const out = [];
  list.forEach((d, i) => {
    const eco = ECOSYSTEMS[d?.packageEcosystem];
    const name = String(d?.dependencyName ?? '');
    if (!eco) return bad(`dependency ${i + 1}: ecosystem ${JSON.stringify(String(d?.packageEcosystem).slice(0, 40))} is not go_modules or npm_and_yarn`);
    if (!(eco === 'Go' ? GO_MODULE : NPM_NAME).test(name)) return bad(`dependency ${i + 1}: name ${JSON.stringify(name.slice(0, 60))} is not a valid ${eco} package name`);
    const prev = String(d?.prevVersion ?? '');
    const next = String(d?.newVersion ?? '');
    if (!VERSION.test(prev) || !VERSION.test(next)) return bad(`dependency ${name}: a version is not a version (${JSON.stringify(prev.slice(0, 30))} to ${JSON.stringify(next.slice(0, 30))})`);
    const type = String(d?.dependencyType ?? '');
    if (!['direct:production', 'direct:development', 'indirect'].includes(type)) return bad(`dependency ${name}: dependency type ${JSON.stringify(type.slice(0, 30))} is not known`);
    const update = String(d?.updateType ?? '');
    if (!UPDATE_TYPES.includes(update)) return bad(`dependency ${name}: update type ${JSON.stringify(update.slice(0, 40))} is not known`);
    out.push({ i, eco, name, prev: prev.replace(/^v/, ''), next: next.replace(/^v/, ''), type, update, alert: String(d?.alertState ?? '') });
  });
  return out;
}

/**
 * Whether the PR is a security update that may be merged by itself.
 * `osv(i)` gives { old: [ids], new: [ids] }: the advisories that affect the
 * old version and those that still affect the new one.
 */
function classify(deps, osv) {
  const why = [];
  const fixed = [];
  let alertOpen = false;
  for (const d of deps) {
    if (d.alert === 'OPEN') alertOpen = true;
    const o = osv(d.i);
    const gone = o.old.filter((id) => !o.new.includes(id));
    if (gone.length) fixed.push({ name: d.name, ids: gone });
    if (d.type.startsWith('direct') && d.update === 'version-update:semver-major') {
      why.push(`${d.name} ${d.prev} to ${d.next} is a semver-major bump of a direct dependency`);
    }
  }
  const security = alertOpen || fixed.length > 0;
  if (!security) why.unshift('not a security update: no open Dependabot alert, and no known advisory of the old versions is fixed by the new ones');
  return { security, alertOpen, fixed, eligible: why.length === 0, reason: why.join('; ') };
}

// ---------------------------------------------------------------------------
// The PR itself.

const BRANCH = {
  'dependabot[bot]': /^dependabot\/(go_modules|npm_and_yarn)\/[A-Za-z0-9._\/@+-]{1,200}$/,
  'github-actions[bot]': /^security\/\d{4}-\d{2}-\d{2}(-\d{1,2})?$/,
};

/** The PR of the REST API against: open, not a draft, into `base`, from this repository, by a bot we know. */
function prInfo(pr, repo, base) {
  if (!pr || typeof pr !== 'object') return bad('the pull request cannot be read'), null;
  const author = String(pr.user?.login ?? '');
  const ref = String(pr.head?.ref ?? '');
  const sha = String(pr.head?.sha ?? '');
  if (pr.state !== 'open') bad(`the PR is ${pr.state}`);
  if (pr.draft) bad('the PR is a draft');
  if (pr.base?.ref !== base) bad(`the PR is not into ${base}`);
  if (pr.head?.repo?.full_name !== repo) bad('the PR comes from another repository');
  if (!BRANCH[author]) bad(`the PR author ${JSON.stringify(author.slice(0, 40))} is not dependabot[bot] or github-actions[bot]`);
  else if (!BRANCH[author].test(ref)) bad(`the branch ${JSON.stringify(ref.slice(0, 80))} is not one ${author} makes`);
  if (!/^[0-9a-f]{40}$/.test(sha)) bad('the head commit is not a commit id');
  return { author, ref, sha };
}

// ---------------------------------------------------------------------------
// Who wrote the PR's commits.

const FIX_SUBJECT = /^fix\(deps\): adapt [^\n]{1,100}$/;
const ADVISORIES_SUBJECT = /^chore\(deps\): fix security advisories \(\d{1,3}\)$/;

/** deps or fix, from the PR author and its commits (the API's list). */
function commitMode(prAuthor, commits) {
  if (!Array.isArray(commits) || commits.length === 0) return bad('the PR has no commits'), null;
  if (commits.length > 10) return bad(`the PR has ${commits.length} commits, more than 10`), null;
  if (!['dependabot[bot]', 'github-actions[bot]'].includes(prAuthor)) return bad(`the PR author ${JSON.stringify(String(prAuthor).slice(0, 40))} is not dependabot[bot] or github-actions[bot]`), null;
  let mode = 'deps';
  commits.forEach((c, i) => {
    const login = c?.author?.login ?? '';
    const subject = String(c?.commit?.message ?? '').split('\n')[0];
    if (i === 0 && login !== prAuthor) bad(`the first commit is by ${JSON.stringify(login)}, not by the PR author ${prAuthor}`);
    else if (login === 'dependabot[bot]') {
      if (prAuthor !== 'dependabot[bot]') bad(`commit ${i + 1} is by dependabot[bot] in a PR by ${prAuthor}`);
    } else if (login === 'github-actions[bot]') {
      if (FIX_SUBJECT.test(subject)) mode = 'fix';
      else if (!(ADVISORIES_SUBJECT.test(subject) && prAuthor === 'github-actions[bot]')) bad(`commit ${i + 1} by github-actions[bot] has an unexpected subject`);
    } else bad(`commit ${i + 1} is by ${JSON.stringify(String(login).slice(0, 40))}: only Dependabot's and this automation's commits may be merged by it`);
  });
  return mode;
}

// ---------------------------------------------------------------------------
// Which files a PR (or a patch) may touch.

/** [{ filename, status, additions, deletions }] against the allowlist of `mode`. */
function checkFiles(files, mode) {
  const info = { touches_src: false, touches_site: false, touches_go: false, lines: 0, code_lines: 0, code_files: 0 };
  if (!Array.isArray(files) || files.length === 0) return bad('the change has no files'), info;
  if (mode !== 'deps' && mode !== 'fix') return bad(`unknown mode ${mode}`), info;
  for (const f of files) {
    const name = String(f?.filename ?? '');
    const status = String(f?.status ?? '');
    const lines = Number(f?.additions ?? 0) + Number(f?.deletions ?? 0);
    const shown = JSON.stringify(name.slice(0, 80));
    if (!name || !SAFE_PATH.test(name) || name.startsWith('/') || name.split('/').some((p) => p === '' || p === '.' || p === '..')) {
      bad(`file ${shown} is not a plain repository path`);
      continue;
    }
    if (name === '.github' || name.startsWith('.github/') || name.split('/').some((p) => p.toLowerCase() === '.git')) {
      bad(`file ${shown} is under .github/ or .git/, which the automation never touches`);
      continue;
    }
    const isManifest = MANIFESTS.includes(name);
    if (isManifest) {
      if (mode === 'fix' && f.fromFix) bad(`file ${shown}: a fix may not change the dependency manifests`);
      if (status !== 'modified') bad(`file ${shown} is ${status || 'of unknown status'}: manifests may only be modified`);
    } else if (mode === 'deps') {
      bad(`file ${shown} is not a dependency manifest (${MANIFESTS.join(', ')})`);
      continue;
    } else if (!FIX_PATTERNS.some((re) => re.test(name))) {
      bad(`file ${shown} is outside what a fix may change (src/, main.go, site/docs/, README.md, llms.txt)`);
      continue;
    } else if (!['added', 'modified', 'removed'].includes(status)) {
      bad(`file ${shown} is ${status || 'of unknown status'}: renames, copies and mode changes are not allowed`);
      continue;
    }
    info.lines += lines;
    if (!isManifest) {
      info.code_lines += lines;
      info.code_files++;
    }
    if (name.startsWith('src/') || name === 'main.go') info.touches_src = true;
    if (name.startsWith('site/')) info.touches_site = true;
    if (name === 'go.mod' || name === 'go.sum' || name.endsWith('.go')) info.touches_go = true;
  }
  const lim = LIMITS[mode];
  if (mode === 'deps' && info.lines > lim.lines) bad(`the change is ${info.lines} lines, more than ${lim.lines}`);
  if (mode === 'fix') {
    if (info.code_lines > lim.lines) bad(`the fix is ${info.code_lines} changed lines of code and docs, more than ${lim.lines}`);
    if (info.code_files > lim.files) bad(`the fix touches ${info.code_files} files, more than ${lim.files}`);
  }
  return info;
}

/** The check runs a PR needs, from what it touches. */
function requiredChecks(info) {
  const r = [];
  if (info.touches_go || info.touches_src) r.push('build (ubuntu-latest)', 'build (macos-latest)', 'build (windows-latest)', 'build');
  if (info.touches_site) r.push('docs');
  if (!r.length) r.push('build (ubuntu-latest)');
  return r;
}

// ---------------------------------------------------------------------------
// The CI of a commit.

/**
 * green: every required check completed with success and nothing present
 * failed; wait: something required is missing or running, or a watched
 * check (CodeQL) is running; red: a required check did not succeed, or any
 * check or status failed.
 */
function ciState(required, watch, runs, statuses, selfPrefix = 'Security: ') {
  const others = runs.filter((r) => !String(r.name).startsWith(selfPrefix));
  const byName = new Map(others.map((r) => [r.name, r])); // the API lists the latest run of each name
  const failed = [];
  const waiting = [];
  for (const r of byName.values()) {
    if (r.status === 'completed' && BAD_CONCLUSIONS.includes(r.conclusion)) failed.push(`${r.name} (${r.conclusion})`);
  }
  for (const s of statuses) if (['failure', 'error'].includes(s.state)) failed.push(`${s.context} (${s.state})`);
  for (const name of required) {
    const r = byName.get(name);
    if (!r) waiting.push(`${name} has not started`);
    else if (r.status !== 'completed') waiting.push(`${name} is ${r.status}`);
    else if (r.conclusion !== 'success') failed.push(`${name} (${r.conclusion})`);
  }
  for (const name of watch) {
    const r = byName.get(name);
    if (r && r.status !== 'completed') waiting.push(`${name} is ${r.status}`);
  }
  if (failed.length) return { ci: 'red', reason: `failed: ${[...new Set(failed)].join(', ')}` };
  if (waiting.length) return { ci: 'wait', reason: waiting.join(', ') };
  return { ci: 'green', reason: 'all required checks passed' };
}

// ---------------------------------------------------------------------------
// The manifests: what a dependency change may change.

const readIf = (dir, rel) => {
  try {
    return fs.readFileSync(path.join(dir, ...rel.split('/')), 'utf8');
  } catch {
    return null;
  }
};

/** Modules of a go.mod: { path -> {version, indirect} }, and its other directives. */
function parseGoMod(text) {
  const mods = new Map();
  const other = [];
  let inRequire = false;
  for (const raw of text.replace(/\r\n?/g, '\n').split('\n')) {
    const line = raw.trim();
    if (line === '' || line.startsWith('//')) continue;
    if (inRequire) {
      if (line === ')') {
        inRequire = false;
        continue;
      }
      const m = /^(\S+)\s+(\S+)(\s*\/\/\s*indirect)?/.exec(line);
      if (m) mods.set(m[1], { version: m[2], indirect: Boolean(m[3]) });
      continue;
    }
    if (/^require\s*\($/.test(line)) {
      inRequire = true;
      continue;
    }
    const one = /^require\s+(\S+)\s+(\S+)(\s*\/\/\s*indirect)?/.exec(line);
    if (one) {
      mods.set(one[1], { version: one[2], indirect: Boolean(one[3]) });
      continue;
    }
    other.push(line.replace(/\s+/g, ' '));
  }
  return { mods, other };
}

/** Changes between two checkouts' manifests; `noMajor` also refuses any major jump. */
function manifestChanges(baseDir, headDir, noMajor = false, only = '') {
  const changes = [];
  const wantGo = only === '' || only === 'go';
  const wantNpm = only === '' || only === 'npm';
  // Go
  const gb = wantGo ? readIf(baseDir, 'go.mod') : null;
  const gh = wantGo ? readIf(headDir, 'go.mod') : null;
  if (!wantGo) {
    /* not asked about */
  } else if (gb !== null && gh !== null) {
    const b = parseGoMod(gb);
    const h = parseGoMod(gh);
    if (JSON.stringify(b.other) !== JSON.stringify(h.other)) {
      bad('go.mod: a directive other than require changed (go, toolchain, replace, exclude, retract or module): needs a person');
    }
    let added = 0;
    for (const [mod, v] of h.mods) {
      const was = b.mods.get(mod);
      if (!was) {
        if (!v.indirect) bad(`go.mod: ${mod} is a new direct dependency`);
        else added++;
        changes.push(['go', mod, '', v.version]);
      } else if (was.version !== v.version) {
        changes.push(['go', mod, was.version, v.version]);
        if (noMajor && major(was.version) !== major(v.version)) bad(`go.mod: ${mod} jumps from ${was.version} to ${v.version}`);
      }
      if (was && was.indirect !== v.indirect && !v.indirect) bad(`go.mod: ${mod} turned from indirect to direct`);
    }
    for (const mod of b.mods.keys()) if (!h.mods.has(mod)) changes.push(['go', mod, b.mods.get(mod).version, '']);
    if (added > LIMITS.newTransitive) bad(`go.mod: ${added} new indirect modules, more than ${LIMITS.newTransitive}`);
  } else if (gb !== gh) bad('go.mod exists in only one of the two trees');

  // npm: package.json
  const pb = wantNpm ? readIf(baseDir, 'site/package.json') : null;
  const ph = wantNpm ? readIf(headDir, 'site/package.json') : null;
  if (pb !== null && ph !== null) {
    const b = parse(pb, 'site/package.json (base)');
    const h = parse(ph, 'site/package.json (head)');
    if (b && h) {
      const sections = ['dependencies', 'devDependencies', 'optionalDependencies', 'peerDependencies', 'overrides'];
      for (const k of new Set([...Object.keys(b), ...Object.keys(h)])) {
        if (sections.includes(k)) {
          const kb = Object.keys(b[k] ?? {}).sort().join('\n');
          const kh = Object.keys(h[k] ?? {}).sort().join('\n');
          if (kb !== kh) bad(`site/package.json: the packages listed in ${k} changed (a dependency was added or removed)`);
        } else if (JSON.stringify(b[k]) !== JSON.stringify(h[k])) {
          bad(`site/package.json: "${k}" changed (only dependency versions may)`);
        }
      }
    }
  } else if (pb !== ph) bad('site/package.json exists in only one of the two trees');

  // npm: package-lock.json
  const lb = wantNpm ? readIf(baseDir, 'site/package-lock.json') : null;
  const lh = wantNpm ? readIf(headDir, 'site/package-lock.json') : null;
  if (lb !== null && lh !== null) {
    const b = parse(lb, 'site/package-lock.json (base)');
    const h = parse(lh, 'site/package-lock.json (head)');
    if (b && h) {
      const bp = b.packages ?? {};
      const hp = h.packages ?? {};
      let added = 0;
      for (const [key, v] of Object.entries(hp)) {
        if (key === '') continue;
        const name = key.replace(/^.*node_modules\//, '');
        const was = bp[key];
        const registry = !v.resolved || /^https:\/\/registry\.npmjs\.org\//.test(v.resolved);
        if ((!was || was.version !== v.version) && !registry) bad(`site/package-lock.json: ${name} ${v.version} does not come from the npm registry (${String(v.resolved).slice(0, 60)})`);
        if (!was) {
          added++;
          changes.push(['npm', name, '', v.version]);
        } else if (was.version !== v.version) {
          changes.push(['npm', name, was.version, v.version]);
          if (noMajor && major(was.version) !== major(v.version)) bad(`site/package-lock.json: ${name} jumps from ${was.version} to ${v.version}`);
        }
      }
      for (const [key, v] of Object.entries(bp)) if (key !== '' && !hp[key]) changes.push(['npm', key.replace(/^.*node_modules\//, ''), v.version, '']);
      if (added > LIMITS.newTransitive) bad(`site/package-lock.json: ${added} new packages, more than ${LIMITS.newTransitive}`);
    }
  } else if (lb !== lh) bad('site/package-lock.json exists in only one of the two trees');
  return changes;
}

// ---------------------------------------------------------------------------
// Lane 3: what the audits say can be fixed without a breaking change.

const SEVERITIES = ['low', 'moderate', 'high', 'critical'];

/** From `npm audit --json`: fixable (no major bump) and skipped vulnerabilities. */
function auditFixable(report, minSeverity = 'low') {
  const out = { fixable: [], skipped: [] };
  const vulns = report?.vulnerabilities;
  if (!vulns || typeof vulns !== 'object') return out;
  for (const [name, v] of Object.entries(vulns)) {
    if (!NPM_NAME.test(name)) continue;
    const sev = String(v?.severity ?? '');
    if (SEVERITIES.indexOf(sev) < SEVERITIES.indexOf(minSeverity)) continue;
    const fix = v?.fixAvailable;
    if (fix === true || (fix && typeof fix === 'object' && fix.isSemVerMajor === false)) out.fixable.push({ name, severity: sev });
    else out.skipped.push({ name, severity: sev, reason: fix && typeof fix === 'object' ? 'the fix is a semver-major bump' : 'no fix is available' });
  }
  return out;
}

/** The JSON values of a stream (govulncheck prints one after another). */
function jsonStream(text) {
  const values = [];
  let depth = 0;
  let start = -1;
  let str = false;
  let esc = false;
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (str) {
      if (esc) esc = false;
      else if (c === '\\') esc = true;
      else if (c === '"') str = false;
      continue;
    }
    if (c === '"') str = true;
    else if (c === '{') {
      if (depth++ === 0) start = i;
    } else if (c === '}' && --depth === 0 && start >= 0) {
      try {
        values.push(JSON.parse(text.slice(start, i + 1)));
      } catch {
        /* a broken value is skipped */
      }
      start = -1;
    }
  }
  return values;
}

/** From `govulncheck -json`: modules with a fixed version of the same major. */
function vulnFixable(text) {
  const mods = new Map();
  for (const m of jsonStream(text)) {
    const f = m.finding;
    if (!f) continue;
    const frame = f.trace?.[0] ?? {};
    const module = String(frame.module ?? '');
    if (!module) continue;
    const e = mods.get(module) ?? { module, from: String(frame.version ?? ''), to: '', ids: new Set() };
    if (f.osv) e.ids.add(String(f.osv));
    const fixed = String(f.fixed_version ?? '');
    if (fixed && (!e.to || cmpSemver(fixed, e.to) > 0)) e.to = fixed;
    if (!e.from) e.from = String(frame.version ?? '');
    mods.set(module, e);
  }
  const out = { fixable: [], skipped: [] };
  for (const e of mods.values()) {
    const ids = [...e.ids].filter((id) => /^[A-Za-z0-9-]{3,40}$/.test(id));
    if (e.module === 'stdlib' || e.module === 'toolchain') out.skipped.push({ module: e.module, reason: 'fixed by upgrading the Go toolchain, not a module' });
    else if (!GO_MODULE.test(e.module)) continue;
    else if (!e.to) out.skipped.push({ module: e.module, reason: 'no fixed version is known' });
    else if (!/^v\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$/.test(e.to) || !/^v\d+\.\d+\.\d+/.test(e.from)) out.skipped.push({ module: e.module, reason: 'unusual version' });
    else if (major(e.from) !== major(e.to)) out.skipped.push({ module: e.module, reason: 'the fix is a different major version' });
    else out.fixable.push({ module: e.module, from: e.from, to: e.to, ids });
  }
  return out;
}

function cmpSemver(a, b) {
  const pa = String(a).replace(/^v/, '').split(/[.-]/).map((x) => parseInt(x, 10) || 0);
  const pb = String(b).replace(/^v/, '').split(/[.-]/).map((x) => parseInt(x, 10) || 0);
  for (let i = 0; i < 3; i++) if (pa[i] !== pb[i]) return (pa[i] ?? 0) - (pb[i] ?? 0);
  return 0;
}

// ---------------------------------------------------------------------------
// A patch (git apply --numstat --summary) read as a list of files.

/** numstat + summary text -> the same file records the API gives for a PR. */
function patchFiles(text) {
  const files = [];
  const status = new Map();
  for (const line of text.split('\n')) {
    let m;
    if ((m = /^ (create|delete) mode (\d+) (.+)$/.exec(line))) {
      if (m[2] === '120000') bad(`${m[3]} is a symbolic link`);
      if (m[2] === '100755') bad(`${m[3]} is an executable file`);
      status.set(m[3], m[1] === 'create' ? 'added' : 'removed');
    } else if ((m = /^ (rename|copy) /.exec(line))) {
      bad(`the patch has a ${m[1]}`);
    } else if (/^ mode change /.test(line)) {
      bad('the patch changes a file mode');
    }
  }
  for (const line of text.split('\n')) {
    const m = /^(\d+|-)\t(\d+|-)\t(.+)$/.exec(line);
    if (!m) continue;
    if (m[1] === '-') {
      bad(`${m[3]} is a binary change`);
      continue;
    }
    files.push({ filename: m[3], status: status.get(m[3]) ?? 'modified', additions: Number(m[1]), deletions: Number(m[2]), fromFix: true });
  }
  return files;
}

// ---------------------------------------------------------------------------

function main() {
  const [cmd, ...args] = process.argv.slice(2);
  switch (cmd) {
    case 'deps': {
      const deps = validDeps(stdin());
      done(deps.map((d) => [d.i, d.eco, d.name, d.prev, d.next, d.type, d.update].join('\t')));
      break;
    }
    case 'classify': {
      const dir = args[0];
      const deps = validDeps(stdin());
      if (reject.length) return done([]);
      const osv = (i) => {
        const ids = (f) => {
          try {
            return (JSON.parse(fs.readFileSync(path.join(dir, f), 'utf8')).vulns ?? []).map((v) => String(v.id)).filter((id) => /^[A-Za-z0-9-]{3,60}$/.test(id));
          } catch {
            return [];
          }
        };
        return { old: ids(`osv-${i}-old.json`), new: ids(`osv-${i}-new.json`) };
      };
      const r = classify(deps, osv);
      done([
        `security=${r.security}`,
        `eligible=${r.eligible}`,
        `reason=${r.reason}`,
        `advisories=${r.fixed.map((f) => `${f.name}: ${f.ids.join(' ')}`).join('; ')}`,
      ]);
      break;
    }
    case 'pr': {
      const info = prInfo(parse(stdin(), 'the pull request'), args[0], args[1]);
      done(info ? [`author=${info.author}`, `head_ref=${info.ref}`, `head_sha=${info.sha}`] : []);
      break;
    }
    case 'commits': {
      const commits = records(stdin(), 'the commit list');
      const mode = commitMode(args[0], commits);
      done([`mode=${mode}`]);
      break;
    }
    case 'files': {
      const files = records(stdin(), 'the file list');
      const info = checkFiles(files, args[0]);
      done([
        `touches_src=${info.touches_src}`,
        `touches_site=${info.touches_site}`,
        `touches_go=${info.touches_go}`,
        `lines=${info.lines}`,
        `required=${requiredChecks(info).join(',')}`,
      ]);
      break;
    }
    case 'patch-files': {
      const files = patchFiles(stdin());
      const info = checkFiles(files, args[0]);
      done([
        `touches_src=${info.touches_src}`,
        `touches_site=${info.touches_site}`,
        `touches_go=${info.touches_go}`,
        `lines=${info.lines}`,
        `files=${files.map((f) => f.filename).join(',')}`,
      ]);
      break;
    }
    case 'manifests': {
      const only = args.find((a) => a.startsWith('--only='))?.slice(7) ?? '';
      const changes = manifestChanges(args[0], args[1], args.includes('--no-major'), only);
      done(changes.map((c) => `change: ${c.join(' ')}`));
      break;
    }
    case 'ci': {
      const [required, watch, runsFile, statusesFile] = args;
      const runs = records(fs.readFileSync(runsFile, 'utf8'), 'the check runs');
      const statuses = records(fs.readFileSync(statusesFile, 'utf8'), 'the statuses');
      if (reject.length) return done([]);
      const r = ciState(required.split(',').filter(Boolean), watch.split(',').filter(Boolean), runs, statuses);
      done([`ci=${r.ci}`, `reason=${r.reason}`]);
      break;
    }
    case 'audit': {
      const report = parse(stdin(), 'the npm audit report');
      const r = auditFixable(report, args[0] || 'low');
      done([
        ...r.fixable.map((v) => ['fixable', v.name, v.severity].join('\t')),
        ...r.skipped.map((v) => ['skipped', v.name, v.severity, v.reason].join('\t')),
      ]);
      break;
    }
    case 'vuln': {
      const r = vulnFixable(stdin());
      done([
        ...r.fixable.map((v) => ['fixable', v.module, v.from, v.to, v.ids.join(' ')].join('\t')),
        ...r.skipped.map((v) => ['skipped', v.module, v.reason].join('\t')),
      ]);
      break;
    }
    default:
      console.error('usage: security-policy.mjs deps|classify|commits|files|patch-files|manifests|ci|audit|vuln');
      process.exit(2);
  }
}

main();
