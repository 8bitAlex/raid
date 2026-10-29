// What triage-prepare.sh needs from JSON, in node (on every runner and on a
// developer's machine; jq is on neither of the places the tests also run).
// Reads stdin, writes stdout, no network. Never run by the workflows
// directly, only through triage-prepare.sh and triage-apply.sh.
//
//   issue            gh issue view --json ... -> issue.json for Claude:
//                    number, title, body, labels, state, author,
//                    author_is_bot, created_at. E-mail addresses are
//                    withheld: Claude does not need them, and they would
//                    end up in a comment.
//   issues [days]    gh issue list --json ... -> issues.json: the open
//                    issues and those closed in the last `days` (90).
//   get KEY          a field of issue.json: a string as it is, a list one
//                    item per line.
import { readFileSync } from 'node:fs';

const MAX_TITLE = 300;
const MAX_BODY = 20000;

const input = () => {
  const text = readFileSync(0, 'utf8');
  return text.charCodeAt(0) === 0xfeff ? text.slice(1) : text; // no byte order mark
};
const names = (labels) => (labels ?? []).map((l) => (typeof l === 'string' ? l : l.name)).filter(Boolean);
const EMAIL = /[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}/g;

/** Text with e-mail addresses withheld. */
function withhold(text) {
  return String(text ?? '').replace(EMAIL, '[email withheld]');
}

function shapeIssue(raw) {
  const i = JSON.parse(raw);
  const login = String(i.author?.login ?? '');
  return {
    number: i.number,
    title: withhold(i.title).slice(0, MAX_TITLE),
    body: withhold(i.body).slice(0, MAX_BODY),
    labels: names(i.labels),
    state: String(i.state ?? '').toLowerCase(),
    author: login,
    author_is_bot: Boolean(i.author?.is_bot) || login.endsWith('[bot]') || login.startsWith('app/'),
    created_at: i.createdAt ?? '',
  };
}

function shapeIssues(raw, days = 90, now = Date.now()) {
  const since = now - days * 86400000;
  return JSON.parse(raw)
    .map((i) => ({
      number: i.number,
      title: withhold(i.title).slice(0, MAX_TITLE),
      state: String(i.state ?? '').toLowerCase(),
      labels: names(i.labels),
      closedAt: i.closedAt || null,
    }))
    .filter((i) => i.state === 'open' || (i.closedAt && Date.parse(i.closedAt) >= since))
    .sort((a, b) => a.number - b.number);
}

function main() {
  const [cmd, arg] = process.argv.slice(2);
  switch (cmd) {
    case 'issue':
      process.stdout.write(JSON.stringify(shapeIssue(input()), null, 2) + '\n');
      break;
    case 'issues':
      process.stdout.write(JSON.stringify(shapeIssues(input(), arg ? Number(arg) : 90), null, 2) + '\n');
      break;
    case 'get': {
      const v = JSON.parse(input())[arg];
      if (Array.isArray(v)) process.stdout.write(v.map((x) => x + '\n').join(''));
      else if (v !== undefined && v !== null) process.stdout.write(String(v) + '\n');
      break;
    }
    default:
      console.error('usage: triage-shape.mjs issue|issues [days]|get KEY < input');
      process.exit(2);
  }
}

main();
