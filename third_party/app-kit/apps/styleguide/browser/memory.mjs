// Memory-only acquisition hooks for the existing native theme task. These collect
// observations, not acceptance decisions. The launcher owns browser/server cleanup.
import {sampleOwnedProcesses} from './process_rss.mjs';

const pause = ms => new Promise(resolve => setTimeout(resolve, ms));

// Closing Playwright's startup page can return before Chrome tears down its
// targets. Wait before the workload, never exclude unknown targets from a sample.
export async function waitForMemoryStartup({guard, now, observations, timeoutMs = 5000, pauseFn = pause, wallNow = () => new Date().toISOString()}) {
  if (!Array.isArray(observations) || !Number.isFinite(timeoutMs) || timeoutMs <= 0) throw new Error('Invalid memory startup observation boundary');
  const deadline = now() + timeoutMs;
  for (;;) {
    const observation = {started_ms: now(), wall_started_at: wallNow()};
    observations.push(observation);
    try {
      const {targetInfos} = await guard.send('Target.getTargets');
      observation.raw_target_infos = structuredClone(targetInfos);
      if (!Array.isArray(targetInfos)) throw new Error('Missing startup target list');
      const remaining = targetInfos.filter(info => !['browser', 'tab', 'browser_ui'].includes(info?.type));
      observation.completed_ms = now(); observation.wall_completed_at = wallNow();
      if (!remaining.length) return;
      if (now() >= deadline) throw new Error('Memory startup targets did not finish teardown');
      await pauseFn(Math.min(100, Math.max(0, deadline - now())));
    } catch (error) {
      observation.error = String(error); observation.completed_ms ??= now(); observation.wall_completed_at ??= wallNow();
      throw error;
    }
  }
}

export async function createMemoryMeasurements({guard, now, browserPid, serverPid, sampleFn = sampleOwnedProcesses, pauseFn = pause, wallNow = () => new Date().toISOString()}) {
  const sample = () => sampleFn({send: guard.send, browserPid, serverPid, now});
  const initial = await sample();
  if (initial.errors.length) throw new Error('Memory owner sample failed: ' + JSON.stringify(initial));
  const owner = pid => {
    const row = initial.rows.find(row => row.pid === pid);
    if (!row) throw new Error('Missing memory owner PID ' + pid);
    return {pid, start_identity: row.start_identity};
  };
  const owners = {browser: owner(browserPid), server: owner(serverPid)};
  const lanes = [];
  let active;

  async function topology(lane) {
    const observation = {started_ms: now(), wall_started_at: wallNow(), raw_target_infos: null, sessions: guard.coverage.sessions.map(session => ({...session}))};
    lane.topology_observations.push(observation);
    try {
      const {targetInfos} = await guard.send('Target.getTargets');
      observation.raw_target_infos = targetInfos;
      const targets = [];
      for (const info of targetInfos) {
        if (['browser', 'tab', 'browser_ui'].includes(info.type)) continue;
        if (!['page', 'iframe'].includes(info.type)) throw new Error('Unexpected live memory target: ' + info.type);
        let pathname;
        try { pathname = new URL(info.url).pathname; } catch { throw new Error('Invalid memory target URL: ' + info.url); }
        const expectedPath = info.type === 'page' ? '/theme-lab' : '/theme-lab/preview';
        if (new URL(info.url).origin !== guard.coverage.allowedOrigin || pathname !== expectedPath) throw new Error('Out-of-scope memory target: ' + info.url);
        const sessions = guard.coverage.sessions.filter(s => s.targetId === info.targetId && !s.detached);
        if (sessions.length !== 1 || !sessions[0].appliedPolicy || !sessions[0].resumed) throw new Error('Memory target lacks unique guarded session');
        const session = sessions[0];
        const isolate = await guard.send('Runtime.getIsolateId', {}, session.sessionId);
        targets.push({target_id: info.targetId, session_id: session.sessionId, type: info.type, url: info.url, isolate_id: isolate.id});
      }
      if (targets.length !== 2 || targets.filter(t => t.type === 'page').length !== 1 || targets.filter(t => t.type === 'iframe').length !== 1) throw new Error('Expected one guide document and one opaque preview');
      observation.completed_ms = now(); observation.wall_completed_at = wallNow();
      return targets.sort((a, b) => a.target_id.localeCompare(b.target_id));
    } catch (error) {
      observation.error = String(error); observation.completed_ms = now(); observation.wall_completed_at = wallNow(); throw error;
    }
  }

  async function begin(id, contextIndex) {
    if (active) throw new Error('Memory lane already active');
    const lane = {id, context_index: contextIndex, task_indexes: [], cycles: [], samples: [], errors: [], endpoint_attempts: [], topology_observations: []};
    const state = {lane, stopped: false, worker: null};
    active = state; lanes.push(lane);
    state.worker = (async () => {
      while (!state.stopped) {
        const started = now();
        lane.samples.push(await sample());
        if (!state.stopped) await pauseFn(Math.max(0, 100 - (now() - started)));
      }
    })().catch(error => lane.errors.push('sampler: ' + error));
    return lane;
  }

  async function settledEndpoint(lane) {
    if (active?.lane !== lane) throw new Error('Wrong active memory lane');
    const endpoint = {started_ms: now(), wall_started_at: wallNow(), gc: [], isolates: [], dom: []};
    lane.endpoint_attempts.push(endpoint);
    try {
    const before = await topology(lane);
    const groups = Map.groupBy(before, target => target.isolate_id);
    for (const [id, targets] of groups) {
      const gc = {isolate_id: id, session_id: targets[0].session_id, started_ms: now(), ok: false};
      endpoint.gc.push(gc);
      await guard.send('HeapProfiler.collectGarbage', {}, gc.session_id);
      gc.completed_ms = now(); gc.ok = true;
    }
    endpoint.idle = {started_ms: now(), network_before: guard.coverage.rawNetworkEvents.length, targets_before: before.map(t => t.target_id)};
    // A requested timer delay is not proof that the minimum interval elapsed.
    const idleDeadline = endpoint.idle.started_ms + 5000;
    for (let remaining = idleDeadline - now(); remaining > 0; remaining = idleDeadline - now()) {
      await pauseFn(remaining);
    }
    endpoint.idle.completed_ms = now();
    const after = await topology(lane);
    endpoint.idle.network_after = guard.coverage.rawNetworkEvents.length;
    endpoint.idle.targets_after = after.map(t => t.target_id);
    endpoint.targets = after;
    if (JSON.stringify(after) !== JSON.stringify(before)) throw new Error('Memory topology/isolate changed during idle');
    for (const [id, targets] of Map.groupBy(after, target => target.isolate_id)) {
      // Runtime heap usage is isolate-wide. Read one canonical session, never
      // sum the same isolate once per target. DOM counters remain per session.
      const heap = await guard.send('Runtime.getHeapUsage', {}, targets[0].session_id);
      endpoint.isolates.push({id, session_ids: targets.map(t => t.session_id), used_bytes: heap.usedSize, total_bytes: heap.totalSize, raw_heap: heap});
    }
    for (const target of after) {
      const counts = await guard.send('Memory.getDOMCounters', {}, target.session_id);
      endpoint.dom.push({session_id: target.session_id, ...counts});
    }
    const deadline = now() + 1500;
    while (!lane.samples.length || lane.samples.at(-1).completed_ms < endpoint.idle.completed_ms) {
      if (now() >= deadline) throw new Error('No complete endpoint RSS sample');
      await pauseFn(10);
    }
    endpoint.sample_index = lane.samples.length - 1;
    endpoint.completed_ms = now(); endpoint.wall_completed_at = wallNow();
    return endpoint;
    } catch (error) {
      endpoint.error = String(error); endpoint.completed_ms ??= now(); endpoint.wall_completed_at ??= wallNow(); throw error;
    }
  }

  async function end(lane) {
    if (!active) return;
    if (active.lane !== lane) throw new Error('Wrong memory lane cleanup');
    active.stopped = true;
    await active.worker;
    active = undefined;
  }

  return {owners, initial, lanes, begin, settledEndpoint, end};
}
