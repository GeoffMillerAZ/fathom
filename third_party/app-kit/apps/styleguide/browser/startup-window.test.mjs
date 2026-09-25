import assert from 'node:assert/strict';
import test from 'node:test';
import {validateStartupWindow} from './startup-window.mjs';

function regularWindow() {
  const samples = [];
  for (let completed = 100; completed <= 5000; completed += 100) {
    samples.push({
      started_ms: completed - 100,
      completed_ms: completed,
      target_infos: [
        {targetId: 'browser-1', type: 'browser'},
        {targetId: 'tab-1', type: 'tab'},
        {targetId: 'ui-1', type: 'browser_ui'},
      ],
    });
  }
  return {started_ms: 0, duration_ms: 5000, max_gap_ms: 500, min_samples: 10, samples, completed_ms: 5000};
}

test('accepts regular sampled observations and describes their bounded scope', () => {
  assert.deepEqual(validateStartupWindow(regularWindow()), {
    started_ms: 0,
    completed_ms: 5000,
    span_ms: 5000,
    sample_count: 50,
    observed_target_ids: ['browser-1', 'tab-1', 'ui-1'],
    observed_target_types: ['browser', 'browser_ui', 'tab'],
    evidence_scope: 'bounded-sampled-observations',
  });
});

test('rejects a slow command or excessive observation gap', () => {
  const slow = regularWindow();
  slow.samples[4].completed_ms = 901;
  assert.throws(() => validateStartupWindow(slow), /exceeds max_gap_ms/);

  const gap = regularWindow();
  gap.samples[4].started_ms = 901;
  gap.samples[4].completed_ms = 901;
  assert.throws(() => validateStartupWindow(gap), /exceeds max_gap_ms/);
});

test('rejects short spans and sparse observations', () => {
  const short = regularWindow();
  short.samples = short.samples.slice(0, -1);
  short.completed_ms = short.samples.at(-1).completed_ms;
  assert.throws(() => validateStartupWindow(short), /shorter than duration_ms/);

  const sparse = regularWindow();
  sparse.samples = sparse.samples.slice(0, 9);
  sparse.completed_ms = sparse.samples.at(-1).completed_ms;
  assert.throws(() => validateStartupWindow(sparse), /at least 10 observations/);
});

test('rejects a forbidden target appearing in any later sample', () => {
  for (const type of ['page', 'background_page']) {
    const window = regularWindow();
    window.samples[37].target_infos.push({targetId: `foreign-${type}`, type});
    assert.throws(() => validateStartupWindow(window), /forbidden target type/);
  }
});

test('rejects nonfinite and reordered timing', () => {
  const nonfinite = regularWindow();
  nonfinite.samples[2].completed_ms = Number.NaN;
  assert.throws(() => validateStartupWindow(nonfinite), /must be finite/);

  const reordered = regularWindow();
  reordered.samples[3].started_ms = reordered.samples[2].completed_ms - 1;
  assert.throws(() => validateStartupWindow(reordered), /started before the previous sample completed/);
});

test('rejects changed contract limits', () => {
  for (const [field, value] of [['duration_ms', 4999], ['max_gap_ms', 501], ['min_samples', 9]]) {
    const window = regularWindow();
    window[field] = value;
    assert.throws(() => validateStartupWindow(window), new RegExp(`${field} must be`));
  }
});

test('rejects a forged end time', () => {
  const window = regularWindow();
  window.completed_ms = 5001;
  assert.throws(() => validateStartupWindow(window), /must equal the last sample completion/);
});
