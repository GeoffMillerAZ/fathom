// Opt-in, container-local reporting for native workspace scripts (D28).
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');

function argumentsFor(argv, env, cwd) {
  if (!env.BENCH_REPORT_DIR || env.VITEST) return [];
  const root = path.resolve(env.BENCH_REPORT_ROOT || '/work/source');
  const directory = path.resolve(cwd);
  if (directory !== root && !directory.startsWith(root + path.sep)) return [];
  if (argv.some(arg => arg === '--outputFile' || arg.startsWith('--outputFile='))) return [];
  const main = path.basename(argv[1] || '');
  if (!['vitest', 'vitest.mjs'].includes(main)) return [];
  if (argv.some(arg => ['list', '--help', '--version'].includes(arg))) return [];
  const phase = String(env.BENCH_REPORT_STAGE || 'count').replace(/[^a-zA-Z0-9_-]/g, '_');
  const key = crypto.createHash('sha256').update(JSON.stringify([cwd, argv.slice(2), env.NITRO_BUILDER || ''])).digest('hex').slice(0, 20);
  return ['--reporter=json', '--outputFile=' + path.join(env.BENCH_REPORT_DIR, phase, key + '.json')];
}

const extra = argumentsFor(process.argv, process.env, process.cwd());
if (extra.length) {
  fs.mkdirSync(path.dirname(extra[1].slice('--outputFile='.length)), { recursive: true });
  process.argv.push(...extra);
}
module.exports = { argumentsFor };
