import assert from 'node:assert/strict';
import {EventEmitter} from 'node:events';
import test from 'node:test';
import {setTimeout as pause} from 'node:timers/promises';
import {terminateOwned,trackOwnedChild} from './owned_process.mjs';

class FakePipe extends EventEmitter {
  constructor() {
    super();
    this.destroyed = false;
  }
  finish(delay = 0) {
    setTimeout(() => {
      this.emit('end');
      this.emit('close');
    }, delay);
  }
  destroy() {
    this.destroyed = true;
  }
}

class FakeChild extends EventEmitter {
  constructor(onKill = () => {}, options = {}) {
    super();
    this.pid = Object.hasOwn(options, 'pid') ? options.pid : 4242;
    this.stdout = new FakePipe();
    this.stderr = new FakePipe();
    this.kills = [];
    this.unrefs = 0;
    this.unrefError = options.unrefError ?? null;
    this.onKill = onKill;
  }
  kill(name) {
    this.kills.push(name);
    return this.onKill(name, this) ?? true;
  }
  unref() {
    this.unrefs++;
    if (this.unrefError) throw this.unrefError;
  }
  complete({code = 0, signal = null, pipeDelay = 0, closeDelay = pipeDelay} = {}) {
    this.emit('exit', code, signal);
    this.stdout.finish(pipeDelay);
    this.stderr.finish(pipeDelay);
    setTimeout(() => this.emit('close', code, signal), closeDelay);
  }
}

const budgets = {graceMs: 20, killMs: 20, drainMs: 20};

test('normal owned termination records distinct exit close and drained pipes', async () => {
  const child = new FakeChild((name, cp) => queueMicrotask(() => cp.complete({signal: name})));
  const state = {}, result = await terminateOwned(trackOwnedChild(child, state), budgets);
  assert.equal(result.ok, true);
  assert.equal(result.forced, false);
  assert.equal(result.unresolved_child, null);
  assert.deepEqual(child.kills, ['SIGTERM']);
  assert.equal(child.unrefs, 0);
  assert.equal(state.exit.kind, 'exit');
  assert.equal(state.close.kind, 'close');
  assert.ok(state.pipes.stdout.end && state.pipes.stderr.close);
});

test('already exited child is never signaled', async () => {
  const child = new FakeChild(), owned = trackOwnedChild(child, {});
  child.complete();
  await pause(1);
  const result = await terminateOwned(owned, budgets);
  assert.equal(result.ok, true);
  assert.deepEqual(child.kills, []);
});

test('SIGKILL escalation remains a cleanup failure', async () => {
  const child = new FakeChild((name, cp) => {
    if (name === 'SIGKILL') queueMicrotask(() => cp.complete({code: null, signal: 'SIGKILL'}));
  });
  const result = await terminateOwned(trackOwnedChild(child, {}), {graceMs: 2, killMs: 20, drainMs: 20});
  assert.equal(result.forced, true);
  assert.equal(result.ok, false);
  assert.deepEqual(child.kills, ['SIGTERM', 'SIGKILL']);
});

test('missing exit after SIGKILL releases local handles, unrefs the exact child, and fails', async () => {
  const child = new FakeChild();
  const state = {};
  const result = await terminateOwned(trackOwnedChild(child, state), {graceMs: 2, killMs: 2, drainMs: 2});
  assert.equal(result.exit_observed, false);
  assert.equal(result.close_observed, false);
  assert.equal(result.capture_complete, false);
  assert.equal(result.ok, false);
  assert.deepEqual(child.kills, ['SIGTERM', 'SIGKILL']);
  assert.equal(child.stdout.destroyed, true);
  assert.equal(child.stderr.destroyed, true);
  assert.equal(child.unrefs, 1);
  assert.equal(result.unresolved_child.pid, child.pid);
  assert.equal(result.unresolved_child.last_state, 'no-exit-observed');
  assert.equal(result.unresolved_child.forced, true);
  assert.equal(result.unresolved_child.local_read_handles_released, true);
  assert.equal(result.unresolved_child.unref_attempted, true);
  assert.equal(result.unresolved_child.unref_error, null);
  assert.equal(state.unresolved_child, result.unresolved_child);
  assert.equal(state._ownedListeners.size, 0);
  assert.ok(state.waits.every(wait => wait.timer_cleared));
});

test('signal exceptions are retained without escaping the bounded result', async () => {
  const child = new FakeChild(() => { throw new Error('synthetic signal refusal'); });
  const state = {}, result = await terminateOwned(trackOwnedChild(child, state), {graceMs: 2, killMs: 2, drainMs: 2});
  assert.equal(result.ok, false);
  assert.equal(result.exit_observed, false);
  assert.equal(state.signals.length, 2);
  assert.ok(state.signals.every(item => /synthetic signal refusal/.test(item.exception)));
  assert.equal(result.unresolved_child.last_state, 'no-exit-observed');
  assert.equal(child.unrefs, 1);
});

test('unref error is retained and cannot turn unresolved termination clean', async () => {
  const child = new FakeChild(() => false, {unrefError: new Error('synthetic unref refusal')});
  const state = {}, result = await terminateOwned(trackOwnedChild(child, state), {graceMs: 2, killMs: 2, drainMs: 2});
  assert.equal(result.ok, false);
  assert.equal(result.capture_complete, false);
  assert.equal(result.unresolved_child.pid, child.pid);
  assert.match(result.unresolved_child.unref_error, /synthetic unref refusal/);
  assert.ok(state.signals.every(item => item.returned === false));
  assert.equal(child.stdout.destroyed, true);
  assert.equal(child.stderr.destroyed, true);
  assert.equal(state._ownedListeners.size, 0);
});

test('delayed close within drain budget is accepted', async () => {
  const child = new FakeChild((name, cp) => queueMicrotask(() => cp.complete({signal: name, pipeDelay: 4, closeDelay: 5})));
  const result = await terminateOwned(trackOwnedChild(child, {}), {graceMs: 10, killMs: 10, drainMs: 20});
  assert.equal(result.ok, true);
  assert.equal(result.capture_complete, true);
});

test('missing close destroys only local pipe reads and never reports clean', async () => {
  const child = new FakeChild((name, cp) => queueMicrotask(() => cp.emit('exit', 0, name)));
  const state = {}, result = await terminateOwned(trackOwnedChild(child, state), {graceMs: 10, killMs: 10, drainMs: 2});
  assert.equal(result.exit_observed, true);
  assert.equal(result.close_observed, false);
  assert.equal(result.capture_complete, false);
  assert.equal(result.ok, false);
  assert.equal(child.stdout.destroyed, true);
  assert.equal(child.stderr.destroyed, true);
  assert.equal(state.pipes.stdout.destroyed_by_owner, true);
});

test('pipe error remains incomplete even when end and close follow', async () => {
  const child = new FakeChild((name, cp) => queueMicrotask(() => {
    cp.emit('exit', 0, name);
    cp.stdout.emit('error', new Error('synthetic truncated stdout'));
    cp.stdout.finish();
    cp.stderr.finish();
    setTimeout(() => cp.emit('close', 0, name), 0);
  }));
  const state = {}, result = await terminateOwned(trackOwnedChild(child, state), budgets);
  assert.equal(result.capture_complete, false);
  assert.equal(result.ok, false);
  assert.match(state.pipes.stdout.error.error, /truncated stdout/);
  assert.ok(state.pipes.stdout.end && state.pipes.stdout.close);
});

test('pipe close without EOF is incomplete and an already closed read end is not destroyed', async () => {
  const child = new FakeChild((name, cp) => queueMicrotask(() => {
    cp.emit('exit', 0, name);
    cp.stdout.emit('close');
    cp.stderr.emit('close');
    cp.emit('close', 0, name);
  }));
  const state = {}, result = await terminateOwned(trackOwnedChild(child, state), budgets);
  assert.equal(result.close_observed, true);
  assert.equal(result.capture_complete, false);
  assert.equal(result.ok, false);
  assert.equal(state.pipes.stdout.end, null);
  assert.equal(child.stdout.destroyed, false);
  assert.equal(child.stderr.destroyed, false);
});

test('spawn error is retained separately and is never clean', async () => {
  const child = new FakeChild(() => {}, {pid: undefined}), owned = trackOwnedChild(child, {});
  child.emit('error', new Error('synthetic spawn failure'));
  child.emit('close', -1, null);
  const result = await terminateOwned(owned, {graceMs: 2, killMs: 2, drainMs: 2});
  assert.match(owned.state.error.error, /synthetic spawn failure/);
  assert.equal(owned.state.exit, null);
  assert.equal(result.ok, false);
  assert.equal(result.capture_complete, false);
  assert.equal(result.unresolved_child, null);
  assert.equal(owned.state.unresolved_child, null);
  assert.equal(child.unrefs, 0);
  assert.deepEqual(child.kills, []);
});

test('event completion clears every armed timer', async () => {
  const child = new FakeChild((name, cp) => queueMicrotask(() => cp.complete({signal: name})));
  const state = {};
  await terminateOwned(trackOwnedChild(child, state), {graceMs: 1000, killMs: 1000, drainMs: 1000});
  assert.ok(state.waits.length > 0);
  assert.ok(state.waits.every(wait => wait.timer_cleared));
  assert.ok(state.waits.every(wait => wait.outcome !== 'timeout'));
});

test('invalid budgets fail before signaling', async () => {
  for (const options of [
    {graceMs: 0, killMs: 1, drainMs: 1},
    {graceMs: 1.5, killMs: 1, drainMs: 1},
    {graceMs: 1, killMs: -1, drainMs: 1},
    {graceMs: 1, killMs: 1, drainMs: 60_001},
  ]) {
    const child = new FakeChild(), owned = trackOwnedChild(child, {});
    await assert.rejects(terminateOwned(owned, options), /milliseconds/);
    assert.deepEqual(child.kills, []);
  }
});
