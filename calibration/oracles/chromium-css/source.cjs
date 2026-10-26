'use strict';

const crypto = require('node:crypto');
const { execFileSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');

const sha256 = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const binaryCompare = (a, b) => a < b ? -1 : a > b ? 1 : 0;

// Deliberately identical to lightningcss_oracle.cjs's listCSS walk, including
// generated CSS files and its only two directory exclusions.
function listCSS(root, dirs = []) {
  const out = [];
  function walk(rel) {
    const abs = path.join(root, rel);
    if (fs.statSync(abs).isFile()) { if (rel.endsWith('.css')) out.push(rel); return; }
    for (const entry of fs.readdirSync(abs, { withFileTypes: true }).sort((a, b) => binaryCompare(a.name, b.name))) {
      if (entry.name === '.git' || entry.name === 'node_modules') continue;
      const next = rel ? rel + '/' + entry.name : entry.name;
      if (entry.isDirectory()) walk(next);
      else if (entry.isFile() && entry.name.endsWith('.css')) out.push(next);
    }
  }
  for (const dir of dirs.length ? dirs : ['']) {
    if (path.isAbsolute(dir) || dir.split(/[\\/]/).includes('..')) throw new Error('DIR must be SUBJECT-relative');
    walk(dir.replace(/\/+$/, ''));
  }
  return out.sort(binaryCompare);
}

function git(root, ...args) {
  return execFileSync('git', ['-C', root, ...args], { encoding: 'utf8' }).trim();
}

function gitIdentity(root) {
  const actual = fs.realpathSync(root);
  const top = fs.realpathSync(git(root, 'rev-parse', '--show-toplevel'));
  if (actual !== top) throw new Error('SUBJECT must be the Git repository root');
  const commit = git(root, 'rev-parse', 'HEAD');
  if (!/^[0-9a-f]{40}$/.test(commit)) throw new Error('SUBJECT has no 40-character Git HEAD');
  if (git(root, 'status', '--porcelain', '--untracked-files=all')) {
    throw new Error('SUBJECT Git tree must be clean');
  }
  return commit;
}

function snapshot(root, dirs = []) {
  const commit = gitIdentity(root);
  const files = listCSS(root, dirs).map(rel => ({ path: rel, sha256: sha256(fs.readFileSync(path.join(root, rel))) }));
  if (gitIdentity(root) !== commit) throw new Error('SUBJECT Git HEAD changed during source snapshot');
  return { commit, files, digest: sha256(JSON.stringify({ commit, files })) };
}

function requireSameSource(expected, root, dirs = []) {
  if (!expected || !/^[0-9a-f]{40}$/.test(expected.commit) || !Array.isArray(expected.files) ||
      expected.digest !== sha256(JSON.stringify({ commit: expected.commit, files: expected.files }))) {
    throw new Error('invalid primary source manifest');
  }
  const current = snapshot(root, dirs);
  if (JSON.stringify(expected) !== JSON.stringify(current)) throw new Error('primary source manifest differs from current source');
  return current;
}

module.exports = { listCSS, snapshot, requireSameSource, binaryCompare };
