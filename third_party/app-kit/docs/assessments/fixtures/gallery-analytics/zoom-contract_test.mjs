import test from 'node:test';
import assert from 'node:assert/strict';
import {zoomCaptureClip, zoomElementCaptureClip, prepareContext} from './zoom-contract.mjs';

const metrics = () => ({contentSize: {x: 0, y: 0, width: 1440, height: 4051},
  cssContentSize: {x: 0, y: 0, width: 720, height: 2025}});

test('capture keeps every reported DIP row despite independent CSS rounding', () => {
  assert.deepEqual(zoomCaptureClip(metrics()), {x: 0, y: 0, width: 1440, height: 4051, scale: 1});
  const exact = metrics(); exact.contentSize.height = 4050;
  assert.equal(zoomCaptureClip(exact).height, 4050);
});
test('missing, shifted, non-zoom, mismatched and unbounded geometry fail', () => {
  for (const mutate of [m => {delete m.contentSize;}, m => {m.contentSize.x = 1;},
    m => {m.cssContentSize.y = 1;}, m => {m.contentSize.width = 720;},
    m => {m.cssContentSize.width = 1440;}, m => {m.contentSize.height = 2025;},
    m => {m.contentSize.height = 4052;}, m => {m.contentSize.height = 4048;},
    m => {m.contentSize.height = Infinity;}, m => {m.cssContentSize.height = NaN;},
    m => {m.contentSize.height = 4050.5;}, m => {m.cssContentSize.height = 0;},
    m => {m.contentSize.height = 20002; m.cssContentSize.height = 10001;}]) {
    const value = metrics(); mutate(value); assert.throws(() => zoomCaptureClip(value));
  }
});

test('element capture converts bounded integer and fractional CSS boxes to DIP', () => {
  const actual = metrics(); actual.contentSize.height = 4730; actual.cssContentSize.height = 2365;
  assert.deepEqual(zoomElementCaptureClip(actual, {x: 16, y: 200, width: 688, height: 602}),
    {x: 32, y: 400, width: 1376, height: 1204, scale: 1});
  assert.deepEqual(zoomElementCaptureClip(metrics(), {x: 0.25, y: 10.75, width: 100.5, height: 20.125}),
    {x: 0.5, y: 21.5, width: 201, height: 40.25, scale: 1});
});

test('element capture retains the exact actual-zoom metrics contract', () => {
  for (const mutate of [m => {m.contentSize.width = 720;}, m => {m.cssContentSize.width = 719;},
    m => {m.contentSize.height = 4048;}, m => {m.cssContentSize.height = Infinity;},
    m => {m.contentSize.x = 1;}]) {
    const value = metrics(); mutate(value);
    assert.throws(() => zoomElementCaptureClip(value, {x: 0, y: 0, width: 688, height: 602}));
  }
});

test('element capture rejects invalid, outside and over-cap CSS boxes', () => {
  const invalid = [
    null,
    {x: NaN, y: 0, width: 1, height: 1},
    {x: 0, y: Infinity, width: 1, height: 1},
    {x: -0.25, y: 0, width: 1, height: 1},
    {x: 0, y: -0.25, width: 1, height: 1},
    {x: 0, y: 0, width: 0, height: 1},
    {x: 0, y: 0, width: 1, height: -1},
    {x: 719.5, y: 0, width: 1, height: 1},
    {x: 0, y: 2024.5, width: 1, height: 1},
    {x: 0, y: 0, width: 720.25, height: 1},
    {x: 0, y: 0, width: 1, height: 1000.25},
  ];
  for (const box of invalid) assert.throws(() => zoomElementCaptureClip(metrics(), box));
});

test('element capture accepts exact document and capture boundaries', () => {
  assert.deepEqual(zoomElementCaptureClip(metrics(), {x: 0, y: 1025, width: 720, height: 1000}),
    {x: 0, y: 2050, width: 1440, height: 2000, scale: 1});
});

test('successful browser setup leaves normal teardown with its caller', async () => {
  let closes = 0;
  assert.equal(await prepareContext({close: async () => {closes++;}}, async () => 'ready'), 'ready');
  assert.equal(closes, 0);
});
test('every setup failure closes its owned context before rejecting', async () => {
  for (const stage of ['settings', 'route', 'websocket', 'page', 'cdp', 'disable scripts']) {
    const error = new Error(stage); const events = [];
    await assert.rejects(prepareContext({close: async () => {events.push('closed');}}, async () => {
      events.push(stage); throw error;
    }), actual => actual === error && events.at(-1) === 'closed');
    assert.deepEqual(events, [stage, 'closed']);
  }
});
test('cleanup failure retains both the setup error and teardown error', async () => {
  const setupError = new Error('setup'), cleanupError = new Error('close');
  await assert.rejects(prepareContext({close: async () => {throw cleanupError;}}, async () => {throw setupError;}),
    error => error instanceof AggregateError && error.errors[0] === setupError && error.errors[1] === cleanupError);
});
