// A fake `gh` for test-security.sh: answers the calls the security scripts
// make from a throwaway git repository (the "remote") and a few files in
// $FAKE_DIR, and logs every call to $FAKE_DIR/calls. It answers the shapes
// of the `--jq` filters the scripts use, not jq itself. Anything it does not
// know exits 99, so a script cannot make a call the test did not expect.
//
//   $FAKE_DIR/remote.git     bare repo: branch main and the PR's branch
//   $FAKE_DIR/pr.json        { number, author, ref, base, state, draft, repo }
//   $FAKE_DIR/comments.json  [{ user, body }] on the PR (grows with comments)
//   $FAKE_DIR/checks.N.json  check runs [{name,status,conclusion}] for the
//                            N-th look at them (the last one repeats)
//   $FAKE_DIR/statuses.json  [{context,state}]
//   $FAKE_DIR/issues.json    open `security` issues [{number,title}]
//   $FAKE_DIR/open_security_pr  number of an open security/ PR, if any
//   $FAKE_DIR/dependabot_prs.txt  lines `gh pr list` prints
//   FAKE_MERGE_FAIL, FAKE_PR_CREATE_FAIL, FAKE_PR_CREATE_URL, FAKE_HEAD_MOVES
import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';

const dir = process.env.FAKE_DIR;
const args = process.argv.slice(2);
fs.appendFileSync(path.join(dir, 'calls'), args.join(' ').replace(/\n/g, '\\n') + '\n');

const read = (f, d) => {
  try {
    return fs.readFileSync(path.join(dir, f), 'utf8');
  } catch {
    return d;
  }
};
const json = (f, d) => {
  const t = read(f, '');
  return t.trim() ? JSON.parse(t) : d;
};
const remote = path.join(dir, 'remote.git');
const git = (...a) => execFileSync('git', ['--git-dir', remote, ...a], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] }).trim();
const die = (msg, code = 99) => {
  console.error(msg);
  process.exit(code);
};
const flag = (name) => {
  const i = args.indexOf(name);
  return i >= 0 ? args[i + 1] : undefined;
};
const pr = () => json('pr.json', {});
const headSha = () => {
  try {
    return git('rev-parse', `refs/heads/${pr().ref}`);
  } catch {
    return '0'.repeat(40);
  }
};
const login = (email, name) => (/dependabot/.test(email) ? 'dependabot[bot]' : /github-actions/.test(email) ? 'github-actions[bot]' : name);
const mergeBase = (sha) => git('merge-base', pr().base, sha);
const ndjson = (list) => list.map((x) => JSON.stringify(x)).join('\n');
const comments = () => json('comments.json', []);
const addComment = (user, body) => fs.writeFileSync(path.join(dir, 'comments.json'), JSON.stringify([...comments(), { user, body }]));

const [cmd, sub] = args;

if (cmd === 'api') {
  const jq = flag('--jq');
  const target = args.find((a, i) => i > 0 && !a.startsWith('-') && args[i - 1] !== '-H' && args[i - 1] !== '--jq' && args[i - 1] !== '-X' && a !== 'api');
  const p = target.replace(/^repos\/example\/repo\//, '');
  let m;
  if (args.includes('DELETE')) process.exit(0);
  if (p === `pulls/${pr().number}`) {
    const state = { number: pr().number, state: pr().state, draft: Boolean(pr().draft), base: { ref: pr().base }, head: { ref: pr().ref, sha: headSha(), repo: { full_name: pr().repo } }, user: { login: pr().author } };
    if (process.env.FAKE_HEAD_MOVES) state.head.sha = 'f'.repeat(40);
    if (jq === '.head.sha') console.log(state.head.sha);
    else if (jq === '.head.ref') console.log(state.head.ref);
    else console.log(JSON.stringify(state));
  } else if ((m = /^pulls\/(\d+)\/commits/.exec(p))) {
    const sha = headSha();
    const raw = git('log', '--reverse', '--format=%H%x1f%ae%x1f%an%x1f%B%x1e', `${mergeBase(sha)}..${sha}`);
    const list = raw.split('\x1e').map((r) => r.trim()).filter(Boolean).map((r) => {
      const [h, email, name, msg] = r.split('\x1f');
      return { sha: h, author: { login: login(email, name) }, commit: { message: msg } };
    });
    console.log(ndjson(list));
  } else if ((m = /^pulls\/(\d+)\/files/.exec(p))) {
    const sha = headSha();
    const range = `${mergeBase(sha)}..${sha}`;
    const status = new Map(git('diff', '--no-renames', '--name-status', range).split('\n').filter(Boolean).map((l) => {
      const [s, ...f] = l.split('\t');
      return [f.join('\t'), { A: 'added', M: 'modified', D: 'removed' }[s] ?? 'modified'];
    }));
    const list = git('diff', '--no-renames', '--numstat', range).split('\n').filter(Boolean).map((l) => {
      const [a, d, ...f] = l.split('\t');
      const filename = f.join('\t');
      return { filename, status: status.get(filename), additions: Number(a), deletions: Number(d) };
    });
    console.log(ndjson(list));
  } else if ((m = /^compare\/[^.]+\.\.\.([0-9a-f]{40})$/.exec(p))) {
    console.log(mergeBase(m[1]));
  } else if ((m = /^contents\/(.+)\?ref=([0-9a-f]{40})$/.exec(p))) {
    try {
      process.stdout.write(execFileSync('git', ['--git-dir', remote, 'show', `${m[2]}:${m[1]}`], { stdio: ['ignore', 'pipe', 'ignore'] }));
    } catch {
      process.exit(1);
    }
  } else if ((m = /^git\/ref\/tags\/(.+)$/.exec(p))) {
    if (!read('tags', '').split('\n').includes(m[1])) process.exit(1);
    console.log('{}');
  } else if ((m = /^commits\/[0-9a-f]{40}\/check-runs/.exec(p))) {
    const n = Number(read('check-looks', '0'));
    fs.writeFileSync(path.join(dir, 'check-looks'), String(n + 1));
    let f = `checks.${n}.json`;
    while (n > 0 && !fs.existsSync(path.join(dir, f)) && Number(f.match(/\d+/)[0]) > 0) f = `checks.${Number(f.match(/\d+/)[0]) - 1}.json`;
    console.log(ndjson(json(f, [])));
  } else if ((m = /^commits\/[0-9a-f]{40}\/status$/.exec(p))) {
    console.log(ndjson(json('statuses.json', [])));
  } else if ((m = /^issues\/(\d+)\/comments/.exec(p))) {
    const want = /contains\("([^"]*)"\)/.exec(jq ?? '')?.[1];
    const ids = comments().map((c, i) => ({ ...c, id: i + 1 })).filter((c) => c.user === 'github-actions[bot]' && c.body.includes(want));
    console.log(ids.map((c) => c.id).join('\n'));
  } else if (p.startsWith('pulls?state=open')) {
    const n = read('open_security_pr', '').trim();
    if (n) console.log(n);
  } else {
    die(`unexpected gh api call: ${args.join(' ')}`);
  }
} else if (cmd === 'pr' && sub === 'merge') {
  if (process.env.FAKE_MERGE_FAIL) die('GraphQL: Pull request is not mergeable', 1);
  console.log('merged');
} else if (cmd === 'pr' && sub === 'comment') {
  const body = flag('--body') ?? fs.readFileSync(flag('--body-file'), 'utf8');
  addComment('github-actions[bot]', body);
  fs.writeFileSync(path.join(dir, `comment.${comments().length}`), body);
} else if (cmd === 'pr' && sub === 'create') {
  if (process.env.FAKE_PR_CREATE_FAIL) die('GitHub Actions is not permitted to create or approve pull requests.', 1);
  fs.copyFileSync(flag('--body-file'), path.join(dir, 'pr-body.md'));
  fs.writeFileSync(path.join(dir, 'pr-created'), JSON.stringify({ title: flag('--title'), head: flag('--head'), base: flag('--base') }));
  console.log(process.env.FAKE_PR_CREATE_URL ?? 'https://github.com/example/repo/pull/31');
} else if (cmd === 'pr' && sub === 'list') {
  process.stdout.write(read('dependabot_prs.txt', ''));
} else if (cmd === 'issue' && sub === 'list') {
  const t = /select\(\.title == \\?"([^"\\]*)\\?"\)/.exec(args.join(' '))?.[1];
  const hit = json('issues.json', []).find((i) => i.title === t);
  if (hit) console.log(hit.number);
} else if (cmd === 'issue' && (sub === 'create' || sub === 'comment')) {
  const f = flag('--body-file');
  fs.copyFileSync(f, path.join(dir, `issue-${sub}.md`));
} else if (cmd === 'label' && sub === 'create') {
  /* fine */
} else if (cmd === 'workflow' && sub === 'run') {
  /* logged */
} else {
  die(`unexpected gh call: ${args.join(' ')}`);
}
