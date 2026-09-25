import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';

const previews = ['dashboard', 'components', 'forms'].flatMap((specimen) =>
  ['light', 'dark'].map((scheme) => `preview-${specimen}-${scheme}`));
const benchmarkNames = ['baseline-page', 'candidate-preview', 'source-comparison', 'candidate-export', ...previews];
const parityNames = [...benchmarkNames, 'invalid-field', 'stale-digest'];
const wantedStatus = Object.fromEntries(parityNames.map((name) =>
  [name, name === 'invalid-field' ? 422 : name === 'stale-digest' ? 409 : 200]));
const sha256 = /^[0-9a-f]{64}$/;

function responseLimit(name) {
  if (name === 'candidate-export') return 1 << 20;
  if (name.startsWith('preview-')) return 600 << 10;
  return 128 << 10;
}

function parseBenchmarks(text, errors) {
  const rows = new Map(benchmarkNames.map((name) => [name, []]));
  const linePattern = /^BenchmarkThemeLabRequest\/([^\s]+?)-2\s+(\S+)\s+(\S+) ns\/op\s+(\S+) response-bytes\/op\s+(\S+) B\/op\s+(\S+) allocs\/op$/;
  for (const line of text.split(/\r?\n/)) {
    if (!line.startsWith('BenchmarkThemeLabRequest/')) continue;
    const match = line.match(linePattern);
    if (!match) {
      errors.push(`malformed benchmark row: ${line}`);
      continue;
    }
    const [, name, nRaw, nsRaw, responseRaw, bytesRaw, allocsRaw] = match;
    if (!rows.has(name)) {
      errors.push(`unexpected benchmark case: ${name}`);
      continue;
    }
    const values = [nRaw, nsRaw, responseRaw, bytesRaw, allocsRaw].map(Number);
    if (values.some((value) => !Number.isFinite(value) || value < 0)) {
      errors.push(`non-finite benchmark value: ${name}`);
      continue;
    }
    const [n, ns, responseBytes, allocatedBytes, allocs] = values;
    if (![n, responseBytes, allocatedBytes, allocs].every(Number.isSafeInteger)) errors.push(`non-integer benchmark value: ${name}`);
    if (n !== 10) errors.push(`${name}: operations ${n}, want 10`);
    if (ns > 50e6) errors.push(`${name}: ${ns} ns/op exceeds 50000000`);
    if (allocatedBytes > 16 * (1 << 20)) errors.push(`${name}: ${allocatedBytes} B/op exceeds 16 MiB`);
    if (responseBytes > responseLimit(name)) errors.push(`${name}: response ${responseBytes} exceeds ${responseLimit(name)}`);
    rows.get(name).push({ ns, responseBytes, allocatedBytes });
  }
  for (const [name, samples] of rows) if (samples.length !== 5) errors.push(`${name}: samples ${samples.length}, want 5`);
  return rows;
}

function parseParity(text, errors) {
  const marked = text.split(/\r?\n/).filter((line) => line.includes('theme-lab-parity-json='));
  let raw;
  if (marked.length === 1) raw = marked[0].slice(marked[0].indexOf('theme-lab-parity-json=') + 22);
  else if (marked.length > 1) errors.push(`parity log contains ${marked.length} snapshots, want 1`);
  else raw = text.trim();
  let parsed;
  try { parsed = JSON.parse(raw); } catch { errors.push('parity snapshot is not valid JSON'); return new Map(); }
  if (!parsed || parsed.schema_version !== 'appkit-theme-lab-parity-v1' || !Array.isArray(parsed.observations)) {
    errors.push('parity snapshot schema or observations are invalid');
    return new Map();
  }
  const observations = new Map();
  for (const item of parsed.observations) {
    if (!item || typeof item.name !== 'string' || observations.has(item.name)) {
      errors.push(`duplicate or malformed parity case: ${item?.name ?? '<missing>'}`);
      continue;
    }
    if (!parityNames.includes(item.name)) errors.push(`unexpected parity case: ${item.name}`);
    observations.set(item.name, item);
  }
  for (const name of parityNames) if (!observations.has(name)) errors.push(`missing parity case: ${name}`);
  if (observations.size !== 12) errors.push(`parity cases ${observations.size}, want 12`);
  return observations;
}

function validateParity(observations, rows, errors) {
  for (const [name, item] of observations) {
    if (!parityNames.includes(name)) continue;
    if (item.status !== wantedStatus[name]) errors.push(`${name}: status ${item.status}, want ${wantedStatus[name]}`);
    if (!sha256.test(item.body_sha256 ?? '')) errors.push(`${name}: invalid body SHA-256`);
    if (!Number.isSafeInteger(item.body_bytes) || item.body_bytes < 0 || item.body_bytes > responseLimit(name)) errors.push(`${name}: invalid or oversized body_bytes`);
    if (!Array.isArray(item.headers) || item.headers.some((pair) => !Array.isArray(pair) || pair.length !== 2 || pair.some((v) => typeof v !== 'string'))) {
      errors.push(`${name}: malformed headers`);
      continue;
    }
    const headers = new Map(item.headers.map(([key, value]) => [key.toLowerCase(), value]));
    if (headers.size !== item.headers.length) errors.push(`${name}: duplicate headers`);
    if (headers.get('content-length') !== String(item.body_bytes)) errors.push(`${name}: Content-Length does not match body_bytes`);
    const expectedType = name === 'candidate-export' ? 'application/zip' : 'text/html; charset=utf-8';
    if (headers.get('content-type') !== expectedType) errors.push(`${name}: Content-Type is not ${expectedType}`);
    if (name === 'candidate-export' && headers.get('x-content-sha256') !== item.body_sha256) errors.push(`${name}: export hash header does not match body`);
    if (name.startsWith('preview-') && !sha256.test(headers.get('x-theme-candidate-sha256') ?? '')) errors.push(`${name}: candidate hash header is missing`);
    for (const sample of rows.get(name) ?? []) if (sample.responseBytes !== item.body_bytes) errors.push(`${name}: benchmark/parity response bytes differ`);
  }
}

export function checkRequestCosts(benchmarkText, parityText) {
  const errors = [];
  const rows = parseBenchmarks(String(benchmarkText), errors);
  const observations = parseParity(String(parityText), errors);
  validateParity(observations, rows, errors);
  const samples = [...rows.values()].flat();
  return { pass: errors.length === 0, summary: {
    benchmark_cases: rows.size, benchmark_samples: samples.length, parity_cases: observations.size,
    max_ns_per_op: Math.max(0, ...samples.map((row) => row.ns)),
    max_allocated_bytes_per_op: Math.max(0, ...samples.map((row) => row.allocatedBytes)),
  }, errors };
}

if (process.argv[1] && fileURLToPath(import.meta.url) === resolve(process.argv[1])) {
  let result;
  try {
    if (process.argv.length !== 4) throw new Error('usage: node check.mjs <benchmark.stdout> <parity.stdout>');
    result = checkRequestCosts(readFileSync(process.argv[2], 'utf8'), readFileSync(process.argv[3], 'utf8'));
  } catch (error) { result = { pass: false, summary: {}, errors: [String(error.message ?? error)] }; }
  process.stdout.write(`${JSON.stringify(result)}\n`);
  if (!result.pass) process.exitCode = 1;
}
