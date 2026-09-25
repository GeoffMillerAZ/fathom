import test from 'node:test';
import assert from 'node:assert/strict';
import {mountShellLive, validShellSnapshot} from './shell_live.js';

// DOM doubles test the adapter's state machine, not rendering, CSP or a browser.
class Element extends EventTarget {
  dataset = {};
  disabled = true;
  hidden = false;
  textContent = '';
  children = [];
  replaceChildren(...children) { this.children = children; }
}
function snapshot(overrides = {}) {
  return {version: '1', display_name: 'Alex Morgan', time_zone: 'America/Phoenix', start_page: 'home', digest: true, access: 'editor', availability: 'ready', activity: ['Synthetic account started'], ...overrides};
}
function fixture() {
  const names = ['shell-live', 'shell-live-start', 'shell-live-stop', 'shell-live-status', 'shell-live-content', 'shell-live-summary', 'shell-live-activity'];
  const nodes = Object.fromEntries(names.map((name) => [name, new Element()]));
  const doc = Object.assign(new EventTarget(), {
    hidden: false,
    getElementById(id) { assert.ok(Object.hasOwn(nodes, id), `unexpected DOM access ${id}`); return nodes[id]; },
    createElement(tag) { assert.equal(tag, 'li'); return new Element(); },
  });
  const win = Object.assign(new EventTarget(), {AbortController});
  let applied;
  let clears = 0;
  const api = mountShellLive(doc, win, () => applied, () => { applied = undefined; clears++; });
  const node = (suffix) => nodes[`shell-live${suffix ? '-' + suffix : ''}`];
  const emit = (type, argsRaw, el = node('start')) => doc.dispatchEvent(new CustomEvent('datastar-fetch', {detail: {type, argsRaw, el}}));
  const patch = (value, actual = value) => {
    applied = actual;
    emit('datastar-patch-signals', {signals: JSON.stringify({appkitShellProbe: value})});
  };
  const connect = () => {
    let resolve, reject, signal;
    const done = api.connect(() => {
      signal = api.controller.signal;
      return new Promise((yes, no) => { resolve = yes; reject = no; });
    });
    return {done, resolve, reject, signal};
  };
  return {doc, win, api, node, emit, patch, connect, get clears() { return clears; }};
}

test('mount is opt-in and does not touch any save forms; absent observer does nothing', () => {
  const f = fixture();
  assert.equal(f.api.controller, undefined);
  assert.equal(f.node('start').disabled, false);
  assert.equal(f.node('stop').disabled, true);
  assert.equal(mountShellLive({getElementById: () => null}, {}, () => {}), null);
});

test('one request at a time; stop cannot reconnect until aborted request has settled', async () => {
  const f = fixture(), first = f.connect();
  assert.equal(f.node().dataset.state, 'connecting');
  assert.equal(f.node('start').disabled, true);
  assert.equal(f.node('stop').disabled, false);
  await f.api.connect(() => assert.fail('second concurrent connection'));
  f.node('stop').dispatchEvent(new Event('click'));
  assert.equal(first.signal.aborted, true);
  assert.equal(f.node('start').disabled, true);
  await f.api.connect(() => assert.fail('overlapping cancellation'));
  first.resolve();
  await first.done;
  assert.equal(f.node('start').disabled, false);
  const second = f.connect();
  assert.notEqual(second.signal, first.signal);
  assert.equal(second.signal.aborted, false);
  second.resolve();
  await second.done;
});

test('injected signal-store snapshots render text only and preserve uint64 precision', async () => {
  const f = fixture(), run = f.connect();
  f.patch(snapshot({version: '18446744073709551615', display_name: '<img src=x onerror=bad()>', activity: ['<script>bad()</script>']}));
  assert.equal(f.node().dataset.state, 'watching');
  assert.match(f.node('summary').textContent, /18446744073709551615.*<img/);
  assert.equal(f.node('activity').children[0].textContent, '<script>bad()</script>');
  assert.equal(f.node('content').hidden, false);
  run.resolve();
  await run.done;
  assert.equal(f.node().dataset.state, 'closed');
  assert.match(f.node('status').textContent, /may be stale/);
});

test('events are filtered by initiating element, not the document target', async () => {
  const f = fixture(), run = f.connect();
  f.emit('error', {status: '403'}, new Element());
  assert.equal(run.signal.aborted, false);
  assert.equal(f.node().dataset.state, 'connecting');
  run.resolve();
  await run.done;
});

for (const code of ['401', '403', 'null']) {
  test(`authorization termination ${code} clears observed content and ignores late data`, async () => {
    const f = fixture(), run = f.connect();
    f.patch(snapshot());
    if (code === 'null') f.patch(null);
    else f.emit('error', {status: code});
    assert.equal(f.node().dataset.state, 'revoked');
    assert.equal(run.signal.aborted, true);
    assert.equal(f.node('content').hidden, true);
    assert.equal(f.node('summary').textContent, '');
    assert.equal(f.node('activity').children.length, 0);
    assert.equal(f.clears, 1, 'clear the underlying signal namespace too');
    f.patch(snapshot({version: '2'}));
    assert.equal(f.node('content').hidden, true);
    assert.equal(f.clears, 2, 'late buffered patch cannot restore the redacted signal namespace');
    run.resolve();
    await run.done;
    assert.equal(f.node().dataset.state, 'revoked');
  });
}

test('non-auth HTTP/network failures retain explicitly stale snapshot without retries', async () => {
  for (const outcome of ['429', '500', 'network']) {
    const f = fixture(), run = f.connect();
    f.patch(snapshot());
    if (outcome === 'network') run.reject(new Error('offline'));
    else { f.emit('error', {status: outcome}); run.resolve(); }
    await run.done;
    assert.equal(f.node().dataset.state, 'interrupted');
    assert.equal(f.node('content').hidden, false);
    assert.match(f.node('status').textContent, /may be stale/);
    assert.equal(f.api.controller, undefined);
  }
});

test('hidden/pagehide pause; show and bfcache restoration never reconnect', async () => {
  for (const reason of ['hidden', 'pagehide']) {
    const f = fixture(), run = f.connect();
    f.patch(snapshot());
    if (reason === 'hidden') { f.doc.hidden = true; f.doc.dispatchEvent(new Event('visibilitychange')); }
    else f.win.dispatchEvent(new Event('pagehide'));
    assert.equal(run.signal.aborted, true);
    run.resolve();
    await run.done;
    assert.equal(f.node().dataset.state, 'paused');
    if (reason === 'hidden') {
      assert.equal(f.node('start').disabled, true);
      await f.api.connect(() => assert.fail('hidden connection'));
    }
    f.doc.hidden = false;
    f.doc.dispatchEvent(new Event('visibilitychange'));
    f.win.dispatchEvent(new Event('pageshow'));
    assert.equal(f.api.controller, undefined);
    assert.equal(f.node('start').disabled, false);
    assert.equal(f.node().dataset.state, 'paused');
  }
});

test('reconnect accepts a fresh session version but rejects ordering regressions within it', async () => {
  const f = fixture(), first = f.connect();
  f.patch(snapshot({version: '9007199254740993'}));
  first.resolve();
  await first.done;
  const second = f.connect();
  f.patch(snapshot());
  assert.equal(f.node().dataset.state, 'watching');
  f.patch(snapshot({version: '10'}));
  assert.equal(f.node().dataset.state, 'watching');
  f.patch(snapshot({version: '9'}));
  assert.equal(f.node().dataset.state, 'invalid');
  assert.equal(second.signal.aborted, true);
  second.resolve();
  await second.done;
});

test('invalid envelopes, oversized frames and signal-store mismatch fail closed', async () => {
  for (const cause of ['envelope', 'json', 'oversize', 'shape', 'mismatch']) {
    const f = fixture(), run = f.connect();
    f.patch(snapshot());
    if (cause === 'envelope') f.emit('datastar-patch-signals', {signals: '{"unknown":null}'});
    if (cause === 'json') f.emit('datastar-patch-signals', {signals: 'broken'});
    if (cause === 'oversize') f.emit('datastar-patch-signals', {signals: ' '.repeat(8193)});
    if (cause === 'shape') f.patch(snapshot({version: 42}));
    if (cause === 'mismatch') f.patch(snapshot({version: '2'}), snapshot());
    assert.equal(f.node().dataset.state, 'invalid', cause);
    assert.equal(f.node('content').hidden, true);
    assert.equal(run.signal.aborted, true);
    run.resolve();
    await run.done;
  }
});

test('snapshot validator is closed, bounded, Unicode-aware and never coerces versions', () => {
  assert.equal(validShellSnapshot(snapshot({display_name: '😀'.repeat(80)})), true);
  for (const version of ['0', '01', '-1', '1e3', '18446744073709551616', '9'.repeat(21), 1, null]) {
    assert.equal(validShellSnapshot(snapshot({version})), false, String(version));
  }
  for (const changes of [{extra: true}, {digest: 'true'}, {activity: Array(11).fill('x')}, {activity: ['x'.repeat(257)]}, {display_name: ''}, {display_name: '😀'.repeat(81)}, {access: 'owner'}, {availability: 'unknown'}, {time_zone: 'guess'}, {start_page: 'other'}]) {
    assert.equal(validShellSnapshot(snapshot(changes)), false, JSON.stringify(changes));
  }
  assert.equal(validShellSnapshot(null), false);
  assert.equal(validShellSnapshot([]), false);
});

test('dispose is idempotent, aborts, removes listeners and prevents future connections', async () => {
  const f = fixture(), run = f.connect();
  f.api.dispose();
  f.api.dispose();
  assert.equal(run.signal.aborted, true);
  run.resolve();
  await run.done;
  assert.equal(f.node('start').disabled, true);
  await f.api.connect(() => assert.fail('disposed observer'));
  const before = f.node().dataset.state;
  f.emit('error', {status: '403'});
  assert.equal(f.node().dataset.state, before);
});
