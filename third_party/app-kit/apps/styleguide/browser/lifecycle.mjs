// Raw diagnostic checkpoints for the opt-in guard lifecycle trace. Recording one
// is NOT lifecycle attribution, heap acceptance or proof of detached reclamation.
const INTERNAL_TYPES = new Set(['browser', 'tab', 'browser_ui']);
const PATHS = {page: '/theme-lab', iframe: '/theme-lab/preview'};
const id = value => typeof value === 'string' && value.length > 0;
const ordinal = (a, b) => a < b ? -1 : a > b ? 1 : 0;

function selectedTargets(raw, guard, contextId) {
  if (!Array.isArray(raw)) throw Error('Missing raw target inventory');
  const ids = new Set(), targets = [];
  for (const info of raw) {
    if (!info || !id(info.targetId) || ids.has(info.targetId)) throw Error('Invalid or duplicate target identity');
    ids.add(info.targetId);
    if (INTERNAL_TYPES.has(info.type)) continue; // Still retained in raw inventory.
    if (!Object.hasOwn(PATHS, info.type)) throw Error(`Unknown application target type: ${info.type}`);
    if (info.browserContextId !== contextId) throw Error('Application target is outside the owned browser context');
    const url = new URL(info.url);
    if (url.origin !== guard.coverage.allowedOrigin || url.pathname !== PATHS[info.type] || url.hash) throw Error('Application target is outside the guide route');
    const sessions = guard.coverage.sessions.filter(s => s.targetId === info.targetId && !s.detached);
    if (sessions.length !== 1 || !id(sessions[0].sessionId) || !sessions[0].appliedPolicy || !sessions[0].resumed || sessions[0].error) throw Error('Target lacks exactly one live guarded session');
    const session = sessions[0];
    if (session.type !== info.type) throw Error('Target and guarded session types differ');
    targets.push({target_id: info.targetId, session_id: session.sessionId, type: info.type, url: info.url, browser_context_id: info.browserContextId});
  }
  if (targets.length !== 2 || targets.filter(t => t.type === 'page').length !== 1 || targets.filter(t => t.type === 'iframe').length !== 1) throw Error('Expected one guide page and one opaque preview target');
  if (new Set(targets.map(t => t.session_id)).size !== targets.length) throw Error('Session identity assigned to multiple targets');
  const live = guard.coverage.sessions.filter(s => !s.detached);
  if (live.length !== targets.length || live.some(s => !targets.some(t => t.target_id === s.targetId && t.session_id === s.sessionId))) throw Error('Live guarded sessions disagree with the target inventory');
  return targets.sort((a, b) => ordinal(a.target_id, b.target_id));
}

function traceBoundary(events) {
  if (!Array.isArray(events) || !events.length) throw Error('Missing opt-in lifecycle trace');
  let last = -1;
  for (const [i, event] of events.entries()) {
    if (event?.seq !== i + 1 || !Number.isFinite(event.monotonic_ms) || event.monotonic_ms < 0 || event.monotonic_ms < last) throw Error('Lifecycle sequence or clock is incomplete');
    last = event.monotonic_ms;
  }
  return events.length;
}

function frameIdentity(response) {
  const frame = response?.frameTree?.frame;
  if (!frame || !id(frame.id) || !id(frame.loaderId)) throw Error('Missing frame/loader identity');
  return {frame_id: frame.id, parent_frame_id: frame.parentId ?? null, loader_id: frame.loaderId};
}

// Caller freezes source/tools and owns browser/server cleanup. The exact expected
// context ID must come from that caller's context-creation receipt, never a URL.
// guard and lifecycleEvents must be the same pair supplied to connectGuard.
export async function captureGuideLifecycle({guard, lifecycleEvents, contextId, label, checkpoints, timeoutMs = 5000, now = () => performance.now()}) {
  if (!Array.isArray(checkpoints)) throw Error('checkpoints must be a caller-owned array');
  const item = {
    schema: 'appkit-guide-lifecycle-checkpoint-v1', label, browser_context_id: contextId,
    started_ms: now(), completed_ms: null, status: 'failed', errors: [],
    trace_start_seq: null, trace_end_seq: null, raw_targets_before: null, raw_targets_after: null,
    targets: [], reads: [], attribution: 'not-verified', memory_complete: false, full_contract_complete: false,
  };
  checkpoints.push(item); // Failed/partial attempts remain available to the owner.
  const deadline = item.started_ms + timeoutMs;
  const checkDeadline = () => {
    const current = now();
    if (!Number.isFinite(current) || current < item.started_ms || current >= deadline) throw Error('Lifecycle checkpoint deadline exceeded or clock invalid');
  };
  const read = async (method, params = {}, sessionId) => {
    checkDeadline();
    if (guard.coverage.failed || guard.coverage.closed) throw Error('Guard failed or closed before lifecycle read');
    const row = {method, session_id: sessionId ?? null, started_ms: now(), completed_ms: null};
    item.reads.push(row);
    let timer;
    try {
      const traceStart = lifecycleEvents.length;
      const remaining = deadline - now();
      if (remaining <= 0) throw Error('Lifecycle checkpoint deadline exceeded');
      row.result = structuredClone(await Promise.race([
        guard.send(method, params, sessionId),
        new Promise((_, reject) => { timer = setTimeout(() => reject(Error('Lifecycle checkpoint deadline exceeded')), remaining); }),
      ]));
      checkDeadline();
      if (guard.coverage.failed || guard.coverage.closed) throw Error('Guard failed during lifecycle read or closed');
      const observed = lifecycleEvents.slice(traceStart);
      const commands = observed.filter(e => e.kind === 'command' && e.method === method && (e.sessionId ?? null) === row.session_id && JSON.stringify(e.params) === JSON.stringify(params));
      if (commands.length !== 1 || !Number.isSafeInteger(commands[0].id) || commands[0].id <= 0) throw Error('Checkpoint command missing or ambiguous in lifecycle trace');
      const command = commands[0];
      const responses = observed.filter(e => e.kind === 'response' && e.id === command.id);
      if (responses.length !== 1 || responses[0].seq <= command.seq || responses[0].method !== method || (responses[0].sessionId ?? null) !== row.session_id || responses[0].error || JSON.stringify(responses[0].result) !== JSON.stringify(row.result)) throw Error('Checkpoint response missing or mismatched in lifecycle trace');
      Object.assign(row, {command_id: command.id, command_seq: command.seq, response_seq: responses[0].seq});
      checkDeadline();
      return row.result;
    } catch (error) {
      row.error = String(error); throw error;
    } finally {
      clearTimeout(timer); row.completed_ms = now();
    }
  };
  try {
    if (!id(contextId) || !id(label) || !Number.isFinite(timeoutMs) || timeoutMs <= 0 || timeoutMs > 5000 || !Number.isFinite(item.started_ms) || item.started_ms < 0) throw Error('Invalid lifecycle checkpoint boundary');
    if (!guard?.coverage?.ready || guard.coverage.failed || guard.coverage.closed) throw Error('Lifecycle checkpoint requires a live ready guard');
    // Presence of an arbitrary array alone must not masquerade as opt-in tracing.
    if (guard.lifecycleEvents !== lifecycleEvents) throw Error('Lifecycle trace does not belong to this guard');
    checkDeadline();
    item.trace_start_seq = traceBoundary(lifecycleEvents);
    checkDeadline();
    const before = await read('Target.getTargets');
    item.raw_targets_before = before?.targetInfos;
    item.targets = selectedTargets(item.raw_targets_before, guard, contextId);
    checkDeadline();
    for (const target of item.targets) {
      const isolate = await read('Runtime.getIsolateId', {}, target.session_id);
      if (!id(isolate?.id)) throw Error('Missing isolate identity');
      const frame = await read('Page.getFrameTree', {}, target.session_id);
      Object.assign(target, {isolate_id: isolate.id}, frameIdentity(frame));
    }
    // A same-target native navigation can replace its loader or isolate. Re-read
    // both, rather than assuming stable IDs from the two target inventories.
    for (const target of item.targets) {
      const isolate = await read('Runtime.getIsolateId', {}, target.session_id);
      const frame = frameIdentity(await read('Page.getFrameTree', {}, target.session_id));
      if (isolate?.id !== target.isolate_id || frame.frame_id !== target.frame_id || frame.loader_id !== target.loader_id || frame.parent_frame_id !== target.parent_frame_id) throw Error('Frame/loader/isolate identity changed during checkpoint');
    }
    const after = await read('Target.getTargets');
    item.raw_targets_after = after?.targetInfos;
    const afterTargets = selectedTargets(item.raw_targets_after, guard, contextId);
    const membership = target => ({target_id: target.target_id, session_id: target.session_id, type: target.type, url: target.url, browser_context_id: target.browser_context_id});
    if (JSON.stringify(item.targets.map(membership)) !== JSON.stringify(afterTargets)) throw Error('Target/session membership changed during checkpoint');
    checkDeadline();
    item.trace_end_seq = traceBoundary(lifecycleEvents);
    if (item.trace_end_seq <= item.trace_start_seq) throw Error('Lifecycle commands were not retained');
    checkDeadline();
    item.status = 'recorded';
  } catch (error) {
    item.errors.push(String(error));
    // Retain the raw trace separately, even when it is malformed or incomplete.
    item.trace_end_seq = Array.isArray(lifecycleEvents) ? lifecycleEvents.length : null;
    throw error;
  } finally {
    item.completed_ms = now();
  }
  return item;
}
