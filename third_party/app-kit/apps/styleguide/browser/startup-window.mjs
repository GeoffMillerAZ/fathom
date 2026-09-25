import assert from 'node:assert/strict';

const finite = (value, name) => assert.ok(Number.isFinite(value), `${name} must be finite`);

export function validateStartupWindow(window) {
  assert.ok(window && typeof window === 'object' && !Array.isArray(window), 'window must be an object');
  assert.equal(window.duration_ms, 5000, 'duration_ms must be 5000');
  assert.equal(window.max_gap_ms, 500, 'max_gap_ms must be 500');
  assert.equal(window.min_samples, 10, 'min_samples must be 10');
  finite(window.started_ms, 'started_ms');
  finite(window.completed_ms, 'completed_ms');
  assert.ok(Array.isArray(window.samples), 'samples must be an array');
  assert.ok(window.samples.length >= 10, 'samples must contain at least 10 observations');

  let previousCompleted = window.started_ms;
  const targetIDs = new Set();
  const targetTypes = new Set();
  for (const [index, sample] of window.samples.entries()) {
    assert.ok(sample && typeof sample === 'object' && !Array.isArray(sample), `samples[${index}] must be an object`);
    finite(sample.started_ms, `samples[${index}].started_ms`);
    finite(sample.completed_ms, `samples[${index}].completed_ms`);
    assert.ok(sample.started_ms >= previousCompleted, `samples[${index}] started before the previous sample completed`);
    assert.ok(sample.completed_ms >= sample.started_ms, `samples[${index}] completed before it started`);
    assert.ok(sample.completed_ms - previousCompleted <= 500, `samples[${index}] exceeds max_gap_ms`);
    assert.ok(Array.isArray(sample.target_infos), `samples[${index}].target_infos must be an array`);

    for (const [targetIndex, target] of sample.target_infos.entries()) {
      assert.ok(target && typeof target === 'object' && !Array.isArray(target), `samples[${index}].target_infos[${targetIndex}] must be an object`);
      assert.ok(typeof target.targetId === 'string' && target.targetId.length > 0,
        `samples[${index}].target_infos[${targetIndex}].targetId must be nonempty`);
      assert.ok(['browser', 'tab', 'browser_ui'].includes(target.type),
        `samples[${index}].target_infos[${targetIndex}] has forbidden target type`);
      targetIDs.add(target.targetId);
      targetTypes.add(target.type);
    }
    previousCompleted = sample.completed_ms;
  }

  const span = previousCompleted - window.started_ms;
  assert.ok(span >= 5000, 'sampled window is shorter than duration_ms');
  assert.ok(span <= 5500, 'sampled window exceeds duration_ms plus max_gap_ms');
  assert.equal(window.completed_ms, previousCompleted, 'completed_ms must equal the last sample completion');

  return {
    started_ms: window.started_ms,
    completed_ms: window.completed_ms,
    span_ms: span,
    sample_count: window.samples.length,
    observed_target_ids: [...targetIDs].sort(),
    observed_target_types: [...targetTypes].sort(),
    evidence_scope: 'bounded-sampled-observations',
  };
}
