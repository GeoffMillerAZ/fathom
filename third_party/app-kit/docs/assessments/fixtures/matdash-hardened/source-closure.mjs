// First-party identity helper. No paid source bodies or dependency execution.
import { createHash } from 'node:crypto';
import { closeSync, constants, fstatSync, lstatSync, openSync, readdirSync, readSync, realpathSync } from 'node:fs';
import { isAbsolute, join } from 'node:path';

export const sourceRoots = [
  '.eslintrc.cjs', '.gitignore', '.npmrc', '.prettierrc', 'README.md', 'index.html',
  'netlify.toml', 'package-lock.json', 'package.json', 'postcss.config.js', 'public',
  'reference-inputs', 'src', 'tailwind.config.ts', 'tsconfig.json', 'tsconfig.node.json', 'vite.config.ts',
];
export const sha256 = bytes => createHash('sha256').update(bytes).digest('hex');
const need = (ok, message) => { if (!ok) throw new Error(message); };
export function sourceSnapshot(project) {
  need(typeof project === 'string' && isAbsolute(project) && !/[\x00-\x1f\x7f]/.test(project), 'Select an absolute private source root.');
  const rootStat = lstatSync(project);
  need(rootStat.isDirectory() && !rootStat.isSymbolicLink(), 'Source root must be a regular directory.');
  const root = realpathSync(project), files = [];
  let entries = 0, total = 0;
  function visit(relative, depth) {
    need(depth <= 24 && ++entries <= 8000, 'Source entry/depth bound exceeded.');
    need(relative.length <= 512 && !/[\\\x00-\x20\x7f%?#:]/.test(relative) && relative.split('/').every(p => p && p !== '.' && p !== '..'), 'Unsafe source path.');
    const path = join(root, relative), stat = lstatSync(path);
    need(!stat.isSymbolicLink(), 'Source symlink refused.');
    if (stat.isDirectory()) {
      const children = readdirSync(path);
      need(children.length <= 4000, 'Source directory bound exceeded.');
      for (const child of children.sort()) visit(`${relative}/${child}`, depth + 1);
      return;
    }
    need(stat.isFile() && files.length < 4000, 'Source file type/count refused.');
    const fd = openSync(path, constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
    try {
      const opened = fstatSync(fd);
      need(opened.isFile() && Number.isSafeInteger(opened.size) && opened.size <= 16 * 1024 * 1024, 'Source file exceeds bounds.');
      const body = Buffer.alloc(opened.size + 1);
      let n, count = 0;
      while (count < body.length && (n = readSync(fd, body, count, body.length - count, null)) > 0) count += n;
      need(count === opened.size, 'Source changed during read.');
      total += count;
      need(total <= 128 * 1024 * 1024, 'Source total exceeds bounds.');
      files.push({ path: relative, bytes: count, sha256: sha256(body.subarray(0, count)) });
    } finally { closeSync(fd); }
  }
  for (const relative of sourceRoots) visit(relative, 0);
  files.sort((a, b) => a.path < b.path ? -1 : a.path > b.path ? 1 : 0);
  return { files, count: files.length, total_bytes: total, tree_sha256: sha256(JSON.stringify(files)) };
}
