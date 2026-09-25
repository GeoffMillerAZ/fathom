import assert from 'node:assert/strict';
import test from 'node:test';
import { checkRequestCosts } from './check.mjs';

const previews = ['dashboard', 'components', 'forms'].flatMap((specimen) =>
  ['light', 'dark'].map((scheme) => `preview-${specimen}-${scheme}`));
const benchmarkNames = ['baseline-page', 'candidate-preview', 'source-comparison', 'candidate-export', ...previews];
const parityNames = [...benchmarkNames, 'invalid-field', 'stale-digest'];
const hash = 'a'.repeat(64);

function bodyBytes(name) {
  if (name === 'candidate-export') return 16036;
  if (name.startsWith('preview-components')) return name.endsWith('light') ? 57929 : 57928;
  if (name.startsWith('preview-dashboard')) return name.endsWith('light') ? 15439 : 15438;
  if (name.startsWith('preview-forms')) return name.endsWith('light') ? 10887 : 10886;
  return 20000;
}

function validBenchmark() {
  return benchmarkNames.flatMap((name) => Array.from({ length: 5 }, (_, sample) =>
    `BenchmarkThemeLabRequest/${name}-2  10  ${500000 + sample} ns/op  ${bodyBytes(name)} response-bytes/op  ${800000 + sample} B/op  6000 allocs/op`)).join('\n');
}

function validParity() {
  const observations = parityNames.map((name) => {
    const bytes = bodyBytes(name);
    const type = name === 'candidate-export' ? 'application/zip' : 'text/html; charset=utf-8';
    const headers = [['Content-Length', String(bytes)], ['Content-Type', type]];
    if (name === 'candidate-export') headers.push(['X-Content-Sha256', hash]);
    if (name.startsWith('preview-')) headers.push(['X-Theme-Candidate-Sha256', hash]);
    return { name, status: name === 'invalid-field' ? 422 : name === 'stale-digest' ? 409 : 200,
      headers, body_sha256: hash, body_bytes: bytes };
  });
  return `=== RUN TestThemeLabRequestParitySnapshot\n    theme_lab_performance_test.go:337: theme-lab-parity-json=${JSON.stringify({ schema_version: 'appkit-theme-lab-parity-v1', observations })}\nPASS\n`;
}

test('accepts the fixed request-cost workload', () => {
  const result = checkRequestCosts(validBenchmark(), validParity());
  assert.equal(result.pass, true, result.errors.join('\n'));
  assert.deepEqual(result.summary, { benchmark_cases: 10, benchmark_samples: 50, parity_cases: 12,
    max_ns_per_op: 500004, max_allocated_bytes_per_op: 800004 });
});

for (const [name, corrupt, pattern] of [
  ['missing benchmark sample', (bench, parity) => [bench.split('\n').slice(1).join('\n'), parity], /samples 4, want 5/],
  ['extra benchmark case', (bench, parity) => [`${bench}\nBenchmarkThemeLabRequest/other-2 10 1 ns/op 1 response-bytes\/op 1 B\/op 1 allocs\/op`, parity], /unexpected benchmark case/],
  ['wrong operation count', (bench, parity) => [bench.replace('  10  ', '  9  '), parity], /operations 9/],
  ['time budget', (bench, parity) => [bench.replace('500000 ns/op', '50000001 ns/op'), parity], /exceeds 50000000/],
  ['allocation budget', (bench, parity) => [bench.replace('800000 B/op', '16777217 B/op'), parity], /exceeds 16 MiB/],
  ['response budget', (bench, parity) => [bench.replace('20000 response-bytes/op', '200000 response-bytes/op'), parity], /response 200000 exceeds/],
  ['wrong concurrency', (bench, parity) => [bench.replace('baseline-page-2', 'baseline-page-8'), parity], /malformed benchmark row/],
  ['nonfinite timing', (bench, parity) => [bench.replace('500000 ns/op', 'NaN ns/op'), parity], /non-finite benchmark value/],
  ['malformed row', (bench, parity) => [bench.replace('500000 ns/op', 'bad timing'), parity], /malformed benchmark row/],
  ['null parity', (bench) => [bench, 'null'], /schema or observations are invalid/],
  ['duplicate parity', (bench, parity) => [bench, parity.replace('"observations":[', '"observations":[' + parity.match(/\{"name":"baseline-page"[^}]+\}/)[0] + ',')], /duplicate or malformed parity case/],
  ['bad status', (bench, parity) => [bench, parity.replace('"name":"invalid-field","status":422', '"name":"invalid-field","status":200')], /status 200, want 422/],
  ['bad body hash', (bench, parity) => [bench, parity.replace(`"body_sha256":"${hash}"`, '"body_sha256":"bad"')], /invalid body SHA-256/],
  ['bad content type', (bench, parity) => [bench, parity.replace('"Content-Type","application/zip"', '"Content-Type","text/html"')], /Content-Type is not application\/zip/],
  ['mismatched body bytes', (bench, parity) => [bench, parity.replace('"body_bytes":16036', '"body_bytes":16035')], /Content-Length does not match|benchmark\/parity response bytes differ/],
]) test(`rejects ${name}`, () => {
  const [benchmark, parity] = corrupt(validBenchmark(), validParity());
  const result = checkRequestCosts(benchmark, parity);
  assert.equal(result.pass, false);
  assert.match(result.errors.join('\n'), pattern);
});
