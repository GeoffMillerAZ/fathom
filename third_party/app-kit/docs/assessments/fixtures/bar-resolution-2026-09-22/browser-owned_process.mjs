// Assessment-only bounded lifecycle tracking for an exactly owned ChildProcess.
// This is evidence machinery, not a public process-management API.
const tracked = new WeakSet();

const stamp = () => new Date().toISOString();
const finiteBudget = (name, value) => {
  if (!Number.isSafeInteger(value) || value <= 0 || value > 60_000) {
    throw new Error(`${name} must be an integer from 1 through 60000 milliseconds`);
  }
};

function notify(state) {
  for (const listener of state._ownedListeners) listener();
}

function observe(state, kind, detail = {}) {
  const event = {kind, at: stamp(), ...detail};
  state.events.push(event);
  if (kind === 'exit' && state.exit === null) state.exit = event;
  if (kind === 'error' && state.error === null) state.error = event;
  if (kind === 'close' && state.close === null) state.close = event;
  notify(state);
}

function trackPipe(stream, name, state) {
  const pipe = state.pipes[name] = {
    present: stream != null,
    end: null,
    close: null,
    error: null,
    destroyed_by_owner: false,
    destroy_error: null,
  };
  if (!stream) return;
  stream.once('end', () => {
    pipe.end = {at: stamp()};
    notify(state);
  });
  stream.once('close', () => {
    pipe.close = {at: stamp()};
    notify(state);
  });
  stream.once('error', error => {
    pipe.error = {at: stamp(), error: String(error)};
    state.events.push({kind: `${name}-error`, ...pipe.error});
    notify(state);
  });
}

export function trackOwnedChild(cp, state = {}) {
  if (!cp || typeof cp.once !== 'function' || typeof cp.kill !== 'function') {
    throw new TypeError('cp must be one exact ChildProcess-like object');
  }
  if (!state || typeof state !== 'object' || Array.isArray(state)) {
    throw new TypeError('state must be a caller-owned object');
  }
  if (tracked.has(cp)) throw new Error('ChildProcess is already tracked');
  tracked.add(cp);
  state.events = [];
  state.signals = [];
  state.waits = [];
  state.exit = null;
  state.error = null;
  state.close = null;
  state.unresolved_child = null;
  state.pipes = {};
  state.tracked_at = stamp();
  Object.defineProperty(state, '_ownedListeners', {value: new Set(), enumerable: false});
  cp.once('exit', (code, signal) => observe(state, 'exit', {code: code ?? null, signal: signal ?? null}));
  cp.once('error', error => observe(state, 'error', {error: String(error)}));
  cp.once('close', (code, signal) => observe(state, 'close', {code: code ?? null, signal: signal ?? null}));
  trackPipe(cp.stdout, 'stdout', state);
  trackPipe(cp.stderr, 'stderr', state);
  return Object.freeze({child: cp, state});
}

function pipesDrained(state) {
  return Object.values(state.pipes).every(pipe => !pipe.present || (pipe.end !== null && pipe.error === null));
}

async function boundedWait(state, name, timeoutMs, predicate) {
  const record = {name, started_at: stamp(), timeout_ms: timeoutMs, outcome: null, completed_at: null, timer_cleared: false};
  state.waits.push(record);
  if (predicate()) {
    record.outcome = 'already-observed';
    record.completed_at = stamp();
    record.timer_cleared = true;
    return true;
  }
  let timer;
  let listener;
  try {
    return await new Promise(resolve => {
      const finish = (outcome, value) => {
        if (record.outcome !== null) return;
        record.outcome = outcome;
        record.completed_at = stamp();
        resolve(value);
      };
      listener = () => {
        if (predicate()) finish('observed', true);
      };
      state._ownedListeners.add(listener);
      timer = setTimeout(() => finish('timeout', false), timeoutMs);
    });
  } finally {
    clearTimeout(timer);
    state._ownedListeners.delete(listener);
    record.timer_cleared = true;
  }
}

function signal(owned, name) {
  const record = {signal: name, attempted_at: stamp(), returned: null, exception: null};
  owned.state.signals.push(record);
  try {
    record.returned = owned.child.kill(name);
  } catch (error) {
    record.exception = String(error);
  }
  record.completed_at = stamp();
  return record;
}

function destroyUndrainedPipes(owned) {
  for (const name of ['stdout', 'stderr']) {
    const stream = owned.child[name], pipe = owned.state.pipes[name];
    if (!stream || pipe.close || typeof stream.destroy !== 'function') continue;
    try {
      stream.destroy();
      pipe.destroyed_by_owner = true;
      pipe.destroyed_at = stamp();
    } catch (error) {
      pipe.destroy_error = String(error);
    }
  }
}

function releaseUnresolvedChild(owned, result) {
  const {child, state} = owned;
  if (state.exit !== null || !Number.isSafeInteger(child.pid) || child.pid <= 0) return;
  const unresolved = state.unresolved_child = result.unresolved_child = {
    pid: child.pid,
    observed_at: stamp(),
    last_state: state.error !== null ? 'error-observed-without-exit' :
      state.close !== null ? 'close-observed-without-exit' : 'no-exit-observed',
    forced: result.forced,
    local_read_handles_released: false,
    unref_attempted: false,
    unref_error: null,
  };
  destroyUndrainedPipes(owned);
  unresolved.local_read_handles_released = Object.values(state.pipes).every(pipe =>
    !pipe.present || pipe.close !== null || pipe.destroyed_by_owner === true);
  unresolved.unref_attempted = true;
  try {
    if (typeof child.unref !== 'function') throw new TypeError('owned ChildProcess has no unref method');
    child.unref();
  } catch (error) {
    unresolved.unref_error = String(error);
  }
}

// result.ok means only that lifecycle and local output capture ended cleanly.
// The caller separately decides whether the recorded workload exit code passed.
export async function terminateOwned(owned, {graceMs, killMs, drainMs} = {}) {
  if (!owned || !tracked.has(owned.child) || owned.state?._ownedListeners === undefined) {
    throw new TypeError('owned must be returned by trackOwnedChild');
  }
  finiteBudget('graceMs', graceMs);
  finiteBudget('killMs', killMs);
  finiteBudget('drainMs', drainMs);
  const {state} = owned;
  if (state.termination) throw new Error('Termination was already requested');
  const result = state.termination = {
    started_at: stamp(),
    exit_observed: state.exit !== null,
    close_observed: state.close !== null,
    capture_complete: false,
    forced: false,
    unresolved_child: null,
    ok: false,
    completed_at: null,
  };

  if (!state.exit && !state.error) signal(owned, 'SIGTERM');
  if (!state.exit && !state.error) await boundedWait(state, 'grace-exit', graceMs, () => state.exit !== null || state.error !== null);
  if (!state.exit && !state.error) {
    result.forced = true;
    signal(owned, 'SIGKILL');
    await boundedWait(state, 'kill-exit', killMs, () => state.exit !== null || state.error !== null);
  }

  await boundedWait(state, 'pipe-drain-and-close', drainMs, () => state.close !== null && pipesDrained(state));
  result.exit_observed = state.exit !== null;
  result.close_observed = state.close !== null;
  result.capture_complete = result.exit_observed && state.close !== null && pipesDrained(state);
  if (result.exit_observed && !result.capture_complete) destroyUndrainedPipes(owned);
  if (!result.exit_observed) releaseUnresolvedChild(owned, result);
  result.ok = result.exit_observed && result.close_observed && result.capture_complete && !result.forced && state.error === null && state.signals.every(item => item.exception === null && item.returned === true);
  result.completed_at = stamp();
  return result;
}
