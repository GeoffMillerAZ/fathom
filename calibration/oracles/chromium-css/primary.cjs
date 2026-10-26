'use strict';

// Produce the unchanged LightningCSS oracle and bind it to a clean, frozen
// source snapshot shared with ranges.cjs.
const { spawn } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const { snapshot, requireSameSource, sidecarFile, writeBoundPrimary } = require('./source.cjs');

async function lightning(subject, dirs) {
  const script = path.resolve(__dirname, '../lightningcss/lightningcss_oracle.cjs');
  const child = spawn(process.execPath, [script, subject, ...dirs], { stdio: ['ignore', 'pipe', 'inherit'] });
  const chunks = [];
  child.stdout.on('data', chunk => chunks.push(chunk));
  const { code, signal } = await new Promise((resolve, reject) => {
    let firstError;
    let hardKill;
    child.once('error', error => { firstError ||= error; });
    child.stdout.once('error', error => {
      firstError ||= error;
      try { child.kill('SIGTERM'); } catch (_) { /* preserve stdout error */ }
      hardKill = setTimeout(() => {
        try { child.kill('SIGKILL'); } catch (_) { /* preserve stdout error */ }
      }, 2000);
    });
    // close fires after stdio closes, unlike exit: no trailing JSON is lost.
    child.once('close', (exitCode, exitSignal) => {
      clearTimeout(hardKill);
      if (firstError) reject(firstError);
      else resolve({ code: exitCode, signal: exitSignal });
    });
  });
  if (code !== 0) throw new Error(`LightningCSS oracle exited code=${code} signal=${signal}`);
  const oracle = JSON.parse(Buffer.concat(chunks).toString('utf8'));
  if (oracle.tool !== 'lightningcss' || !Array.isArray(oracle.definitions) ||
      !Array.isArray(oracle.calls) || !Array.isArray(oracle.imports)) {
    throw new Error('unexpected LightningCSS oracle shape');
  }
  return oracle;
}

async function main(args) {
  const [subject, output, ...dirs] = args;
  if (!subject || !output) throw new Error('usage: node primary.cjs SUBJECT OUTPUT_JSON [DIR...]');
  if (fs.existsSync(output) || fs.existsSync(sidecarFile(output))) {
    throw new Error('output or source sidecar already exists; remove stale files before run');
  }
  const before = snapshot(subject, dirs);
  const oracle = await lightning(subject, dirs);
  requireSameSource(before, subject, dirs);
  writeBoundPrimary(output, oracle, before);
  process.stderr.write(JSON.stringify({ source_commit: before.commit, source_digest: before.digest,
    source_files: before.files.length }) + '\n');
}

if (require.main === module) main(process.argv.slice(2)).catch(error => {
  console.error(error.message);
  process.exitCode = 1;
});

module.exports = { lightning };
