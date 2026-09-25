import test from 'node:test';
import assert from 'node:assert/strict';

import { consumerExpectation } from './probe.mjs';

const usageCase = { id: 'unknown-command', kind: 'usage' };

function output(text = '', limited = false) {
  return {
    bytes: Buffer.byteLength(text),
    observed_bytes_before_stop: Buffer.byteLength(text),
    limited,
    sha256: '',
    raw_text: text,
  };
}

function result(overrides = {}) {
  return {
    code: 2,
    signal: null,
    timed_out: false,
    spawn_error: null,
    stdout: output(),
    stderr: output('unknown command\n'),
    ...overrides,
  };
}

test('usage reporting and exit policy remain separate', () => {
  const silentOne = consumerExpectation(usageCase, result({ code: 1, stderr: output() }));
  assert.equal(silentOne.dimensions.reporting.passed, false);
  assert.equal(silentOne.dimensions.exit_policy.passed, false);
  assert.equal(silentOne.passed, false);

  const diagnosticOne = consumerExpectation(usageCase, result({ code: 1 }));
  assert.equal(diagnosticOne.dimensions.reporting.passed, true);
  assert.equal(diagnosticOne.dimensions.exit_policy.passed, false);
  assert.equal(diagnosticOne.passed, false, 'reporting repair must not satisfy the usage-2 oracle');

  const diagnosticTwo = consumerExpectation(usageCase, result());
  assert.equal(diagnosticTwo.dimensions.reporting.passed, true);
  assert.equal(diagnosticTwo.dimensions.exit_policy.passed, true);
  assert.equal(diagnosticTwo.passed, true);
});

test('unsafe or incomplete usage results cannot pass either combined oracle', () => {
  const cases = [
    ['timeout', { timed_out: true }],
    ['stdout truncation', { stdout: output('', true) }],
    ['stderr truncation', { stderr: output('unknown command\n', true) }],
    ['stdout ANSI', { stdout: output('\x1b[31mnoise\x1b[0m') }],
    ['stderr ANSI', { stderr: output('\x1b[31munknown command\x1b[0m\n') }],
    ['unexpected stdout', { stdout: output('noise\n') }],
    ['spawn failure', { spawn_error: 'synthetic spawn failure' }],
    ['signal', { signal: 'SIGKILL' }],
  ];
  for (const [name, override] of cases) {
    const got = consumerExpectation(usageCase, result(override));
    assert.equal(got.passed, false, name);
  }
});

test('help and version expectations are unchanged', () => {
  const help = consumerExpectation({ id: 'help', kind: 'success' }, result({
    code: 0,
    stdout: output('help text\n'),
    stderr: output(),
  }));
  assert.equal(help.passed, true);
  assert.equal(help.dimensions, undefined);

  const version = consumerExpectation({ id: 'version', kind: 'version' }, result({
    code: 0,
    stdout: output('bosun v0.1.0-dev\n'),
    stderr: output(),
  }));
  assert.equal(version.passed, true);
  assert.equal(version.dimensions, undefined);

  assert.equal(consumerExpectation({ id: 'version', kind: 'version' }, result({
    code: 0,
    stdout: output('bosun development\n'),
    stderr: output(),
  })).passed, false);
});
