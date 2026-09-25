import assert from 'node:assert/strict';
import test from 'node:test';
import {memoryControlFlags, memoryControlModes} from './memory-control-profile.mjs';

const baseline = [
  '--headless=new',
  '--remote-debugging-address=127.0.0.1',
  '--remote-debugging-port=0',
  '--user-data-dir={PROFILE}',
  '--disk-cache-dir={PROFILE}/cache',
  '--window-size=1440,1000',
  '--disable-background-networking',
  '--no-proxy-server',
  '--no-first-run',
  '--no-default-browser-check',
  'about:blank',
];
const candidateFlag = '--disable-component-extensions-with-background-pages';

test('baseline exactly matches the frozen memory-control launch flags', () => {
  assert.deepEqual(memoryControlFlags(), baseline);
  assert.deepEqual(memoryControlFlags(undefined), baseline);
  assert.deepEqual(memoryControlFlags('baseline'), baseline);
});

test('candidate adds exactly one flag immediately before the final URL', () => {
  const candidate = memoryControlFlags('no-background-components-v1');
  assert.equal(candidate.length, baseline.length + 1);
  assert.equal(candidate.at(-2), candidateFlag);
  assert.equal(candidate.at(-1), 'about:blank');
  assert.equal(candidate.filter(flag => flag === candidateFlag).length, 1);
  assert.deepEqual(candidate.filter(flag => flag !== candidateFlag), baseline);
  assert.deepEqual(memoryControlFlags('no-background-components-controls-v1'), candidate);
  assert.deepEqual(memoryControlFlags('no-background-components-shutdown-v1'), candidate);
});

test('profiles retain loopback debugging, owned profile paths, and sandbox policy', () => {
  for (const profile of ['baseline', 'no-background-components-v1', 'no-background-components-controls-v1',
    'no-background-components-shutdown-v1']) {
    const flags = memoryControlFlags(profile);
    assert.ok(flags.includes('--remote-debugging-address=127.0.0.1'));
    assert.ok(flags.includes('--remote-debugging-port=0'));
    assert.ok(flags.includes('--user-data-dir={PROFILE}'));
    assert.ok(flags.includes('--disk-cache-dir={PROFILE}/cache'));
    for (const weaker of [
      '--no-sandbox',
      '--disable-web-security',
      '--disable-site-isolation-trials',
      '--remote-debugging-address=0.0.0.0',
      '--remote-allow-origins=*',
    ]) assert.ok(!flags.includes(weaker));
  }
});

test('modes isolate the startup diagnostic from the two control acquisitions', () => {
  assert.deepEqual(memoryControlModes(), ['positive', 'retention']);
  assert.deepEqual(memoryControlModes('baseline'), ['positive', 'retention']);
  assert.deepEqual(memoryControlModes('no-background-components-v1'), ['startup']);
  assert.deepEqual(memoryControlModes('no-background-components-controls-v1'), ['positive', 'retention']);
  assert.deepEqual(memoryControlModes('no-background-components-shutdown-v1'), ['shutdown']);
});

test('shutdown-only selection cannot admit memory or startup acquisitions', () => {
  const modes = memoryControlModes('no-background-components-shutdown-v1');
  modes.push('positive', 'retention', 'startup');
  assert.deepEqual(memoryControlModes('no-background-components-shutdown-v1'), ['shutdown']);
  for (const profile of ['baseline', 'no-background-components-v1', 'no-background-components-controls-v1'])
    assert.ok(!memoryControlModes(profile).includes('shutdown'));
});

test('mode selections are fresh arrays that cannot affect later calls', () => {
  const baselineModes = memoryControlModes();
  const controlsModes = memoryControlModes('no-background-components-controls-v1');
  assert.notStrictEqual(baselineModes, controlsModes);
  baselineModes.splice(0, baselineModes.length, 'startup');
  controlsModes.push('startup');
  assert.deepEqual(memoryControlModes(), ['positive', 'retention']);
  assert.deepEqual(memoryControlModes('no-background-components-controls-v1'), ['positive', 'retention']);
});

test('returns fresh arrays that cannot mutate later selections', () => {
  const first = memoryControlFlags();
  const second = memoryControlFlags();
  assert.notStrictEqual(first, second);
  first.splice(0, first.length, '--no-sandbox');
  second.push('--disable-web-security');
  assert.deepEqual(memoryControlFlags(), baseline);
  assert.deepEqual(
    memoryControlFlags('no-background-components-v1').filter(flag => flag !== candidateFlag),
    baseline,
  );
});

test('rejects unknown and non-string profiles without flag passthrough', () => {
  for (const value of [null, 0, false, {}, [], ['--no-sandbox']]) {
    assert.throws(() => memoryControlFlags(value), /profile must be a supported string/);
    assert.throws(() => memoryControlModes(value), /profile must be a supported string/);
  }
  for (const value of ['', 'candidate', '--no-sandbox', 'no-background-components-v2']) {
    assert.throws(() => memoryControlFlags(value), /unsupported memory control profile/);
    assert.throws(() => memoryControlModes(value), /unsupported memory control profile/);
  }
});
