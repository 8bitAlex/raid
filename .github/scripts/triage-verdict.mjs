// Checks Claude's triage verdict and turns it into the comment and labels
// (called by triage-apply.sh). Nothing Claude wrote is trusted: the verdict
// is validated against a strict schema, and every text in it is rewritten
// so that it can do nothing on GitHub but be read.
//
//   node triage-verdict.mjs <issue> <input dir> <verdict.json> <repo root> <out dir>
//
// Passes (exit 0) and writes <out dir>/comment.md and <out dir>/labels.txt
// (one label a line). Otherwise exit 1 with every reason on stderr, one
// `reject:` line each, and writes nothing.
//
// The schema (the triage job's prompt spells out the same):
//   category        bug | enhancement | question | documentation | invalid
//   priority        high | medium | low
//   summary         string, at most 200 characters
//   analysis        markdown string, at most 3500 characters
//   suspected_files up to 8 of {path, line}: a file of the checkout (inside
//                   it, not a symlink) and a line it has
//   duplicate_of    null, or the number of another issue in issues.json
//   fixed_in        null, or a version (0.9.1) that releases.txt lists
//   needs_info      up to 6 strings, at most 200 characters each
//   next_step       string, at most 300 characters
//   claude_can_attempt  boolean
// and no other key.
import fs from 'node:fs';
import path from 'node:path';

const CATEGORIES = ['bug', 'enhancement', 'question', 'documentation', 'invalid'];
const PRIORITIES = ['high', 'medium', 'low'];
const KEYS = [
  'category', 'priority', 'summary', 'analysis', 'suspected_files',
  'duplicate_of', 'fixed_in', 'needs_info', 'next_step', 'claude_can_attempt',
];
const MAX = { summary: 200, analysis: 3500, next_step: 300, info_item: 200, info: 6, files: 8, path: 200 };
const MAX_VERDICT_BYTES = 100 * 1024;
const MAX_SOURCE_BYTES = 8 * 1024 * 1024;
// Never a suspected file (in any folder of the path): git's own folder,
// installed packages and the folders of this workflow.
const NOT_CODE = new Set(['.git', 'triage-input', 'triage-output', 'node_modules']);

const EMAIL = /[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}/g;
// Control characters (not the newline), and the invisible and direction
// characters that disguise text.
const INVISIBLE = new RegExp(
  '[' + [[0, 9], [0x0b, 0x1f], [0x7f, 0x9f], [0xad, 0xad], [0x200b, 0x200f], [0x2028, 0x202e], [0x2060, 0x206f], [0xfeff, 0xfeff]]
    .map(([a, b]) => String.fromCharCode(a) + '-' + String.fromCharCode(b)).join('') + ']',
  'g',
);
const ZERO_WIDTH_SPACE = String.fromCharCode(0x200b);
const ANGLE_QUOTE = String.fromCharCode(0x2039);

/**
 * Text that can only be read: no HTML, links, images, mentions or e-mail.
 * Cut at `max` characters (the schema's limit before the rewrite, which
 * can only lengthen it a little).
 */
function clean(text, max, { oneLine = false } = {}) {
  let s = String(text).normalize('NFC').replace(/\r\n?/g, '\n').replace(/\t/g, '  ');
  s = s.replace(INVISIBLE, '');
  // In code, an angle bracket is only turned into a look-alike: Vec<String> stays
  // readable. Where this and GitHub disagree about what is code, the result
  // is the same look-alike, which starts nothing.
  const defang = (code) => code.replace(/</g, ANGLE_QUOTE);
  s = s.replace(/```[\s\S]*?```/g, defang).replace(/`[^`\n]*`/g, defang);
  s = s.replace(/<!--[\s\S]*?(?:-->|$)/g, '');
  // Tags, again until none is left (`<<b>script>` leaves one after a pass).
  for (let prev = null; prev !== s; ) {
    prev = s;
    s = s.replace(/<\/?[A-Za-z][^<>]*>/g, '');
  }
  s = s.replace(/</g, ANGLE_QUOTE); // nothing that starts a tag or an autolink
  s = s.replace(/!\[[^\]]*\]\([^)]*\)/g, ''); // images
  s = s.replace(/\[([^\]]*)\]\([^)]*\)/g, '$1'); // links keep their text
  s = s.replace(/^[ ]{0,3}\[[^\]]+\]:[ \t]*\S.*$/gm, ''); // link definitions
  s = s.replace(/\b(?:https?|ftp|file|data|javascript|mailto):[^\s)>\]]*/gi, '[link removed]');
  s = s.replace(/\bwww\.[^\s)>\]]+/gi, '[link removed]');
  s = s.replace(EMAIL, '[email removed]');
  s = s.replace(/\b([\w.-]+\/[\w.-]+)#(\d+)/g, '$1 #$2'); // another repository's issue
  s = s.replace(/@(?=[A-Za-z0-9_])/g, '@' + ZERO_WIDTH_SPACE); // a mention that notifies nobody
  if (oneLine) return s.replace(/\s+/g, ' ').trim();
  s = s.replace(/\n{3,}/g, '\n\n').trim();
  if ((s.match(/```/g) || []).length % 2) s += '\n```'; // a fence left open
  return s;
}

/** Whether a string is a sound, repository-relative path. */
function sanePath(p) {
  if (typeof p !== 'string' || p.length === 0 || p.length > MAX.path) return false;
  if (!/^[A-Za-z0-9_.@+\/-]+$/.test(p) || p.startsWith('/')) return false;
  const parts = p.split('/');
  return parts.every((x) => x !== '' && x !== '.' && x !== '..' && !NOT_CODE.has(x));
}

/** The reasons verdict `v` is refused (none: it is valid). */
function problems(v, { issue, issues, releases, root }) {
  const bad = [];
  const isObj = (x) => x !== null && typeof x === 'object' && !Array.isArray(x);
  if (!isObj(v)) return ['the verdict is not a JSON object'];
  for (const k of KEYS) if (!(k in v)) bad.push(`missing "${k}"`);
  for (const k of Object.keys(v)) if (!KEYS.includes(k)) bad.push(`unknown key "${k}"`);
  if (bad.length) return bad;

  if (!CATEGORIES.includes(v.category)) bad.push('category is not one of ' + CATEGORIES.join('|'));
  if (!PRIORITIES.includes(v.priority)) bad.push('priority is not one of ' + PRIORITIES.join('|'));
  for (const [k, max] of [['summary', MAX.summary], ['analysis', MAX.analysis], ['next_step', MAX.next_step]]) {
    if (typeof v[k] !== 'string' || v[k].trim() === '') bad.push(`${k} is not a text`);
    else if (v[k].length > max) bad.push(`${k} is ${v[k].length} characters, more than ${max}`);
  }
  if (typeof v.claude_can_attempt !== 'boolean') bad.push('claude_can_attempt is not true or false');

  if (v.duplicate_of !== null) {
    if (!Number.isInteger(v.duplicate_of)) bad.push('duplicate_of is neither null nor an issue number');
    else if (v.duplicate_of === issue) bad.push('duplicate_of is the issue itself');
    else if (!issues.has(v.duplicate_of)) bad.push(`duplicate_of #${v.duplicate_of} is not in the list of issues`);
  }
  if (v.fixed_in !== null) {
    if (typeof v.fixed_in !== 'string' || !/^\d{1,3}\.\d{1,3}\.\d{1,3}$/.test(v.fixed_in)) {
      bad.push('fixed_in is neither null nor a version like 0.9.1');
    } else if (!releases.has(v.fixed_in)) {
      bad.push(`fixed_in ${v.fixed_in} is not a release in releases.txt`);
    }
  }
  if (!Array.isArray(v.needs_info) || v.needs_info.length > MAX.info) {
    bad.push(`needs_info is not a list of at most ${MAX.info}`);
  } else {
    for (const q of v.needs_info) {
      if (typeof q !== 'string' || q.trim() === '' || q.length > MAX.info_item) {
        bad.push(`a needs_info item is not a text of 1 to ${MAX.info_item} characters`);
        break;
      }
    }
  }

  if (!Array.isArray(v.suspected_files) || v.suspected_files.length > MAX.files) {
    bad.push(`suspected_files is not a list of at most ${MAX.files}`);
  } else {
    const realRoot = fs.realpathSync(root);
    for (const f of v.suspected_files) {
      const shown = typeof f?.path === 'string' ? JSON.stringify(f.path.slice(0, 80)) : '(no path)';
      if (!isObj(f) || Object.keys(f).sort().join() !== 'line,path') {
        bad.push('a suspected_files item is not {path, line}');
      } else if (!sanePath(f.path)) {
        bad.push(`suspected file ${shown} is not a path inside the repository`);
      } else if (!Number.isInteger(f.line) || f.line < 1) {
        bad.push(`suspected file ${shown} has a line that is not a positive integer`);
      } else {
        const lines = sourceLines(path.join(root, ...f.path.split('/')), realRoot);
        if (lines === null) bad.push(`suspected file ${shown} is not a file of the checkout`);
        else if (f.line > lines) bad.push(`suspected file ${shown} has ${lines} lines, not ${f.line}`);
      }
    }
  }
  return bad;
}

/** The number of lines of a regular file inside `realRoot`, else null. */
function sourceLines(file, realRoot) {
  try {
    if (!fs.lstatSync(file).isFile()) return null; // not a symlink, not a folder
    const real = fs.realpathSync(file);
    if (real !== realRoot && !real.startsWith(realRoot + path.sep)) return null;
    const size = fs.statSync(file).size;
    if (size > MAX_SOURCE_BYTES) return Infinity; // too big to count: any line will do
    const text = fs.readFileSync(file, 'utf8');
    return text.split('\n').length - (text.endsWith('\n') ? 1 : 0);
  } catch {
    return null;
  }
}

/** The comment, from a verdict that passed `problems`. */
function comment(v, { runUrl, sha }) {
  const label = { bug: 'Bug', enhancement: 'Enhancement', question: 'Question', documentation: 'Documentation', invalid: 'Invalid' }[v.category];
  const out = [`## Automated triage`, ``, `**${label}, priority ${v.priority}.** ${clean(v.summary, MAX.summary, { oneLine: true })}`, ``, clean(v.analysis, MAX.analysis)];
  if (v.suspected_files.length) {
    out.push(``, `**Suspected code**`);
    for (const f of v.suspected_files) out.push(`- \`${f.path}:${f.line}\``);
  }
  if (v.duplicate_of !== null) out.push(``, `**Possible duplicate of** #${v.duplicate_of}.`);
  if (v.fixed_in !== null) out.push(``, `**Fixed in** ${v.fixed_in}: update to that version or a later one.`);
  if (v.needs_info.length) {
    out.push(``, `**Questions for the reporter**`);
    for (const q of v.needs_info) out.push(`- ${clean(q, MAX.info_item, { oneLine: true })}`);
  }
  out.push(``, `**Next step.** ${clean(v.next_step, MAX.next_step, { oneLine: true })}`);
  if (v.claude_can_attempt) out.push(``, 'This looks small and clear enough for an automated fix attempt; a maintainer decides.');
  const run = /^https:\/\/github\.com\/[\w.-]+\/[\w.-]+\/actions\/runs\/\d+$/.test(runUrl || '') ? ` ([run](${runUrl}))` : '';
  const at = /^[0-9a-f]{7,40}$/.test(sha || '') ? ` at \`${sha.slice(0, 8)}\`` : '';
  out.push(``, `---`, `<sub>Written by an automated Claude run${run} from the issue text and the code${at}. It may be wrong; a person has not reviewed it.</sub>`);
  return out.join('\n') + '\n';
}

/** The labels to add. */
function labels(v) {
  const l = ['triaged', v.category, `priority: ${v.priority}`];
  if (v.needs_info.length) l.push('needs-info');
  if (v.duplicate_of !== null) l.push('duplicate');
  return l;
}

function main() {
  const [issueArg, inDir, verdictFile, root, outDir] = process.argv.slice(2);
  const fail = (reasons) => {
    for (const r of reasons) console.error(`reject: ${r}`);
    process.exit(1);
  };
  const issue = Number(issueArg);
  let v, issues, releases;
  try {
    if (fs.lstatSync(verdictFile).isSymbolicLink() || fs.statSync(verdictFile).size > MAX_VERDICT_BYTES) {
      fail(['the verdict is a link or larger than 100 KB']);
    }
    v = JSON.parse(fs.readFileSync(verdictFile, 'utf8'));
  } catch (e) {
    fail([`the verdict cannot be read as JSON (${e.code || e.name})`]);
  }
  try {
    issues = new Set(JSON.parse(fs.readFileSync(path.join(inDir, 'issues.json'), 'utf8')).map((i) => i.number));
    releases = new Set(
      fs.readFileSync(path.join(inDir, 'releases.txt'), 'utf8').split('\n')
        .map((l) => /^v(\d+\.\d+\.\d+)(?:\s|$)/.exec(l)?.[1]).filter(Boolean),
    );
  } catch (e) {
    fail([`the input in ${inDir} cannot be read (${e.code || e.name})`]);
  }
  const bad = problems(v, { issue, issues, releases, root });
  if (bad.length) fail(bad);
  fs.mkdirSync(outDir, { recursive: true });
  fs.writeFileSync(path.join(outDir, 'comment.md'), comment(v, { runUrl: process.env.RUN_URL, sha: process.env.SHA }));
  fs.writeFileSync(path.join(outDir, 'labels.txt'), labels(v).join('\n') + '\n');
}

main();
