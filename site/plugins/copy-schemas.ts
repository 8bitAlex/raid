import fs from 'node:fs';
import path from 'node:path';
import type { LoadContext, Plugin } from '@docusaurus/types';

// copy-schemas publishes the JSON schemas under static/schema/v1/.
// The repo's schemas/ directory is the single source of truth (also embedded
// into the Go binary). This plugin copies them into the static tree before
// Docusaurus walks static/, so it serves them at
// https://raidcli.dev/schema/v1/<name>.
//
// The /v1/ path is part of the public schema contract: schemas under that URL
// must remain backwards compatible. Breaking changes get a new /v2/ path.
const repoRoot = path.resolve(__dirname, '..', '..');
const srcDir = path.join(repoRoot, 'schemas');
const destDir = path.join(__dirname, '..', 'static', 'schema', 'v1');

fs.mkdirSync(destDir, { recursive: true });
for (const entry of fs.readdirSync(srcDir)) {
  if (!entry.endsWith('.schema.json')) continue;
  fs.copyFileSync(path.join(srcDir, entry), path.join(destDir, entry));
}

export default function copySchemasPlugin(_context: LoadContext): Plugin {
  return {
    name: 'copy-schemas',
  };
}
