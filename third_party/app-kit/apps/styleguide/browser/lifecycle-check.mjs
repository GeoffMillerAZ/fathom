// Pure join of the guard's raw CDP lifecycle trace and bounded guide
// checkpoints. This qualifies the observation, never memory attribution.
const INTERNAL_TYPES = new Set(['browser', 'tab', 'browser_ui']);
const MAX_EVENTS = 50000, MAX_CHECKPOINTS = 16, MAX_READS = 64;
const id = value => typeof value === 'string' && value.length > 0 && value.length <= 1024;
const same = (a, b) => JSON.stringify(a) === JSON.stringify(b);

function boundary(events, checkpoints, contextId, origin) {
  if (!Array.isArray(events) || events.length === 0 || events.length > MAX_EVENTS) throw Error('events must be a non-empty bounded array');
  if (!Array.isArray(checkpoints) || checkpoints.length === 0 || checkpoints.length > MAX_CHECKPOINTS) throw Error('checkpoints must be a non-empty bounded array');
  if (!id(contextId)) throw Error('contextId must be a non-empty string');
  const url = new URL(origin);
  if (url.origin !== origin || url.protocol !== 'http:' || !['127.0.0.1', '[::1]'].includes(url.hostname) || !url.port) throw Error('origin must be an exact loopback HTTP origin');
  let serialized;
  try { serialized = JSON.stringify({events, checkpoints}); } catch { throw Error('lifecycle input must be serializable'); }
  if (new TextEncoder().encode(serialized).length > 16 * 1024 * 1024) throw Error('lifecycle input exceeds byte bound');
}

function memberKey(member) { return `${member.type}:${member.target_id}:${member.session_id}`; }

export function joinGuideLifecycle({events, checkpoints, contextId, origin} = {}) {
  boundary(events, checkpoints, contextId, origin);
  const reasons = [], reasonKeys = new Set();
  const add = (code, detail = '') => {
    const key = `${code}\0${detail}`;
    if (!reasonKeys.has(key)) { reasonKeys.add(key); reasons.push({code, ...(detail ? {detail} : {})}); }
  };
  const observations = {
    event_count: events.length, checkpoint_count: checkpoints.length, checkpoints: [],
    shared_isolates: [], retired_members: [], replacements: [],
  };
  const result = {
    schema: 'appkit-guide-lifecycle-join-v1', evidence_valid: false,
    live_membership_complete: false, unqualified_reasons: reasons, observations,
    attribution: 'not-verified', memory_complete: false, full_contract_complete: false,
  };

  const commands = new Map(), responses = new Map();
  let lastClock = -1;
  for (let index = 0; index < events.length; index++) {
    const event = events[index];
    if (!event || event.seq !== index + 1 || !Number.isFinite(event.monotonic_ms) || event.monotonic_ms < 0 || event.monotonic_ms < lastClock) {
      add('invalid_trace_order', `event ${index + 1}`); continue;
    }
    lastClock = event.monotonic_ms;
    if (event.kind === 'command') {
      if (!Number.isSafeInteger(event.id) || event.id <= 0 || !id(event.method) || commands.has(event.id)) add('ambiguous_command', String(event.id));
      else commands.set(event.id, event);
    } else if (event.kind === 'response') {
      if (!Number.isSafeInteger(event.id) || event.id <= 0 || responses.has(event.id)) add('ambiguous_response', String(event.id));
      else responses.set(event.id, event);
      if (event.error) add('protocol_error', `${event.method ?? 'unknown'}#${event.id}`);
    } else if (event.kind === 'local-error') add('protocol_error', `${event.method ?? 'unknown'}#${event.id ?? 'unknown'}`);
    else if (event.kind !== 'event') add('malformed_event_kind', String(event.kind));
  }
  for (const [commandId, command] of commands) {
    const response = responses.get(commandId);
    if (!response || response.seq <= command.seq || response.method !== command.method ||
        (response.sessionId ?? null) !== (command.sessionId ?? null)) add('missing_or_mismatched_response', String(commandId));
  }
  for (const responseId of responses.keys()) if (!commands.has(responseId)) add('response_without_command', String(responseId));

  const eventRows = events.filter(event => event?.kind === 'event');
  const attachments = new Map(), targetSessions = new Map(), createdTargets = new Map(), observedTargets = new Map();
  const destroyedTargets = new Map(), frameEvents = new Map(), contexts = new Map();
  for (const event of eventRows) {
    const params = event.params ?? (event.method === 'Runtime.executionContextsCleared' ? {} : null);
    if (!params || typeof params !== 'object') { add('malformed_lifecycle_event', event.method ?? 'unknown'); continue; }
    if (event.method === 'Target.targetCreated' || event.method === 'Target.targetInfoChanged') {
      const info = params.targetInfo;
      if (!info || !id(info.targetId) || !id(info.type) || typeof info.url !== 'string') add('malformed_target_identity', event.method);
      else {
        if (event.method === 'Target.targetCreated') {
          if (createdTargets.has(info.targetId)) add('duplicate_target_creation', info.targetId);
          else createdTargets.set(info.targetId, {seq: event.seq, info: structuredClone(info)});
        }
        const history = observedTargets.get(info.targetId) ?? [];
        history.push({seq: event.seq, info: structuredClone(info)}); observedTargets.set(info.targetId, history);
      }
    } else if (event.method === 'Target.attachedToTarget') {
      const info = params.targetInfo, sessionId = params.sessionId;
      if (!id(sessionId) || !info || !id(info.targetId) || !id(info.type) || typeof info.url !== 'string' || typeof params.waitingForDebugger !== 'boolean') {
        add('malformed_attachment', String(sessionId)); continue;
      }
      if (attachments.has(sessionId)) { add('duplicate_session_attachment', sessionId); continue; }
      const row = {session_id: sessionId, target_id: info.targetId, type: info.type, url: info.url,
        context_id: info.browserContextId ?? null, attach_seq: event.seq, parent_session_id: event.sessionId ?? null,
        waiting: params.waitingForDebugger, detach_seq: null, destroyed_seq: null};
      attachments.set(sessionId, row);
      const sessions = targetSessions.get(info.targetId) ?? [];
      sessions.push(row); targetSessions.set(info.targetId, sessions);
    } else if (event.method === 'Target.detachedFromTarget') {
      if (!id(params.sessionId) || !attachments.has(params.sessionId)) add('unknown_detachment', String(params.sessionId));
      else {
        const row = attachments.get(params.sessionId);
        if (!row.detach_seq) row.detach_seq = event.seq; // CDP may repeat detach notifications.
      }
    } else if (event.method === 'Target.targetDestroyed') {
      if (!id(params.targetId) || destroyedTargets.has(params.targetId) ||
          (!targetSessions.has(params.targetId) && !createdTargets.has(params.targetId))) add('unknown_or_duplicate_target_destruction', String(params.targetId));
      else {
        destroyedTargets.set(params.targetId, event.seq);
        for (const row of targetSessions.get(params.targetId) ?? []) row.destroyed_seq = event.seq;
      }
    } else if (event.method === 'Page.frameAttached' || event.method === 'Page.frameNavigated' || event.method === 'Page.frameDetached') {
      if (!id(event.sessionId)) { add('frame_event_without_session', event.method); continue; }
      const rows = frameEvents.get(event.sessionId) ?? [];
      rows.push(event); frameEvents.set(event.sessionId, rows);
    } else if (event.method === 'Runtime.executionContextCreated') {
      const context = params.context, unique = context?.uniqueId, numeric = context?.id;
      if (!id(event.sessionId) || !context || (!id(unique) && !Number.isSafeInteger(numeric)) || !id(context.auxData?.frameId)) add('malformed_execution_context', String(event.sessionId));
      else {
        const rows = contexts.get(event.sessionId) ?? [];
        if (rows.some(row => !row.retired_seq && ((id(unique) && row.unique_id === unique) || (Number.isSafeInteger(numeric) && row.numeric_id === numeric)))) {
          add('duplicate_execution_context', event.sessionId);
        }
        rows.push({unique_id: id(unique) ? unique : null, numeric_id: Number.isSafeInteger(numeric) ? numeric : null,
          frame_id: context.auxData.frameId, created_seq: event.seq, retired_seq: null});
        contexts.set(event.sessionId, rows);
      }
    } else if (event.method === 'Runtime.executionContextDestroyed') {
      if (!id(event.sessionId)) add('context_event_without_session', event.method);
      else {
        const unique = id(params.executionContextUniqueId) ? params.executionContextUniqueId : null;
        const numeric = Number.isSafeInteger(params.executionContextId) ? params.executionContextId : null;
        const candidates = (contexts.get(event.sessionId) ?? []).filter(item => !item.retired_seq &&
          (!unique || item.unique_id === unique) && (numeric === null || item.numeric_id === numeric));
        if ((unique === null && numeric === null) || candidates.length !== 1) add('unknown_or_ambiguous_context_destruction', event.sessionId);
        else candidates[0].retired_seq = event.seq;
      }
    } else if (event.method === 'Runtime.executionContextsCleared') {
      if (!id(event.sessionId)) add('context_event_without_session', event.method);
      else for (const row of contexts.get(event.sessionId) ?? []) if (!row.retired_seq) row.retired_seq = event.seq;
    }
  }

  function successfulCommand(method, sessionId, beforeSeq) {
    return [...commands.values()].filter(command => command.method === method && (command.sessionId ?? null) === sessionId && command.seq < beforeSeq)
      .filter(command => { const response = responses.get(command.id); return response && !response.error && response.seq < beforeSeq; })
      .sort((a, b) => a.seq - b.seq).at(-1);
  }
  function policyQualified(session, boundarySeq) {
    const required = ['Target.setAutoAttach', 'Network.enable', 'Network.setBlockedURLs', 'Page.enable', 'Runtime.enable',
      'Page.setLifecycleEventsEnabled', 'Page.getFrameTree', 'Runtime.getIsolateId'];
    const resumes = [...commands.values()].filter(command => command.method === 'Runtime.runIfWaitingForDebugger' &&
      command.sessionId === session.session_id && command.seq < boundarySeq && responses.get(command.id) && !responses.get(command.id).error)
      .sort((a, b) => a.seq - b.seq);
    if (!session.waiting || resumes.length !== 1) return false;
    const resume = resumes[0], rows = required.map(method => successfulCommand(method, session.session_id, resume.seq));
    if (resume.seq <= session.attach_seq || responses.get(resume.id).seq >= boundarySeq ||
        rows.some(row => !row) || rows.some(row => row.seq <= session.attach_seq || responses.get(row.id).seq >= resume.seq)) return false;
    const blocked = rows[2], lifecycle = rows[5];
    const attach = rows[0];
    return attach.params?.autoAttach === true && attach.params?.waitForDebuggerOnStart === true && attach.params?.flatten === true &&
      same(blocked.params, {urlPatterns: [{urlPattern: `${origin}/*`, block: false}], urls: ['*']}) &&
      same(lifecycle.params, {enabled: true});
  }
  function readJoin(checkpoint, row) {
    if (!row || !id(row.method) || !Number.isSafeInteger(row.command_id) || !Number.isSafeInteger(row.command_seq) || !Number.isSafeInteger(row.response_seq)) return null;
    const command = commands.get(row.command_id), response = responses.get(row.command_id);
    if (!command || !response || command.seq !== row.command_seq || response.seq !== row.response_seq ||
        command.method !== row.method || response.method !== row.method ||
        (command.sessionId ?? null) !== (row.session_id ?? null) || !same(command.params, {}) || response.error || !same(response.result, row.result) ||
        command.seq <= checkpoint.trace_start_seq || response.seq > checkpoint.trace_end_seq) return null;
    return {command, response};
  }
  function exactTarget(info, type) {
    if (!info || !id(info.targetId) || info.type !== type || info.browserContextId !== contextId || typeof info.url !== 'string') return false;
    try {
      const url = new URL(info.url);
      return url.origin === origin && !url.hash && (type === 'page' ? url.pathname === '/theme-lab' : url.pathname === '/theme-lab/preview');
    } catch { return false; }
  }

  let previous = null, previousCheckpoint = null;
  const checkpointReadIDs = new Set();
  const identityMethods = new Set(['Target.targetCreated', 'Target.targetInfoChanged', 'Target.attachedToTarget',
    'Target.detachedFromTarget', 'Target.targetDestroyed', 'Page.frameAttached', 'Page.frameNavigated',
    'Page.frameDetached', 'Runtime.executionContextCreated', 'Runtime.executionContextDestroyed',
    'Runtime.executionContextsCleared']);
  for (const [checkpointIndex, checkpoint] of checkpoints.entries()) {
    const observed = {label: checkpoint?.label ?? null, members: []};
    observations.checkpoints.push(observed);
    if (!checkpoint || !Array.isArray(checkpoint.reads) || checkpoint.reads.length > MAX_READS ||
        !Number.isSafeInteger(checkpoint.trace_start_seq) || !Number.isSafeInteger(checkpoint.trace_end_seq) ||
        checkpoint.trace_start_seq < 1 || checkpoint.trace_end_seq > events.length || checkpoint.trace_start_seq >= checkpoint.trace_end_seq) {
      add('malformed_checkpoint', String(checkpointIndex)); continue;
    }
    if (previousCheckpoint && (checkpoint.trace_start_seq < previousCheckpoint.trace_end_seq ||
        checkpoint.started_ms < previousCheckpoint.completed_ms)) {
      add('overlapping_or_replayed_checkpoint', String(checkpointIndex)); continue;
    }
    previousCheckpoint = checkpoint;
    if (eventRows.some(event => identityMethods.has(event.method) && event.seq > checkpoint.trace_start_seq && event.seq <= checkpoint.trace_end_seq)) {
      add('identity_changed_during_checkpoint', String(checkpointIndex)); continue;
    }
    const joined = checkpoint.reads.map(row => readJoin(checkpoint, row));
    if (joined.some(row => !row)) { add('checkpoint_read_mismatch', String(checkpointIndex)); continue; }
    const readIDs = checkpoint.reads.map(row => row.command_id);
    if (new Set(readIDs).size !== readIDs.length || readIDs.some(id => checkpointReadIDs.has(id)) ||
        checkpoint.reads.some((row, index) => index && row.command_seq <= checkpoint.reads[index - 1].response_seq) ||
        checkpoint.reads[0]?.method !== 'Target.getTargets' || checkpoint.reads.at(-1)?.method !== 'Target.getTargets') {
      add('ambiguous_checkpoint_reads', String(checkpointIndex)); continue;
    }
    for (const id of readIDs) checkpointReadIDs.add(id);
    if (!Number.isFinite(checkpoint.started_ms) || !Number.isFinite(checkpoint.completed_ms) || checkpoint.started_ms < 0 || checkpoint.completed_ms < checkpoint.started_ms ||
        checkpoint.reads.some(row => !Number.isFinite(row.started_ms) || !Number.isFinite(row.completed_ms) || row.completed_ms < row.started_ms)) {
      add('invalid_checkpoint_clock', String(checkpointIndex)); continue;
    }
    const targetReads = checkpoint.reads.filter(row => row.method === 'Target.getTargets' && row.session_id == null);
    if (checkpoint.reads.length !== 10 || targetReads.length !== 2) { add('incomplete_checkpoint_reads', String(checkpointIndex)); continue; }
    const before = targetReads[0].result?.targetInfos, after = targetReads[1].result?.targetInfos;
    if (!Array.isArray(before) || !Array.isArray(after) || !same(before, after)) { add('unstable_target_inventory', String(checkpointIndex)); continue; }
    const app = before.filter(info => !INTERNAL_TYPES.has(info?.type));
    const page = app.filter(info => exactTarget(info, 'page'));
    const iframe = app.filter(info => exactTarget(info, 'iframe'));
    if (app.length !== 2 || page.length !== 1 || iframe.length !== 1) { add('incomplete_or_unknown_membership', String(checkpointIndex)); continue; }
    const discovered = [...observedTargets.entries()].map(([targetId, history]) => [targetId,
      history.filter(row => row.seq <= checkpoint.trace_end_seq).at(-1)]).filter(([targetId, row]) => row &&
      !INTERNAL_TYPES.has(row.info.type) && (!destroyedTargets.has(targetId) || destroyedTargets.get(targetId) > checkpoint.trace_end_seq));
    if (discovered.length !== app.length || discovered.some(([targetId, row]) => !app.some(info => info.targetId === targetId &&
      info.type === row.info.type && info.url === row.info.url && info.browserContextId === row.info.browserContextId))) {
      add('discovered_target_inventory_mismatch', String(checkpointIndex)); continue;
    }
    const members = [], memberTrees = [];
    for (const info of [page[0], iframe[0]]) {
      const sessions = targetSessions.get(info.targetId) ?? [];
      const live = sessions.filter(session => session.attach_seq <= checkpoint.trace_end_seq &&
        (!session.detach_seq || session.detach_seq > checkpoint.trace_end_seq) && (!session.destroyed_seq || session.destroyed_seq > checkpoint.trace_end_seq));
      if (live.length !== 1) { add('ambiguous_live_session', info.targetId); continue; }
      const session = live[0];
      if (session.type !== info.type || session.context_id !== contextId || !policyQualified(session, checkpoint.trace_end_seq)) {
        add('unguarded_live_session', info.targetId); continue;
      }
      const isolateReads = checkpoint.reads.filter(row => row.method === 'Runtime.getIsolateId' && row.session_id === session.session_id);
      const frameReads = checkpoint.reads.filter(row => row.method === 'Page.getFrameTree' && row.session_id === session.session_id);
      const isolates = isolateReads.map(row => row.result?.id), frames = frameReads.map(row => row.result?.frameTree?.frame);
      if (isolates.length !== 2 || !id(isolates[0]) || isolates[0] !== isolates[1] || frames.length !== 2 || !same(frames[0], frames[1]) ||
          !same(frameReads[0].result?.frameTree, frameReads[1].result?.frameTree) ||
          !id(frames[0]?.id) || !id(frames[0]?.loaderId) || typeof frames[0]?.url !== 'string') {
        add('missing_or_unstable_identity', info.targetId); continue;
      }
      let frameURL;
      try { frameURL = new URL(frames[0].url); } catch { frameURL = null; }
      if (!frameURL || frames[0].url !== info.url || frameURL.origin !== origin) { add('frame_target_url_mismatch', info.targetId); continue; }
      const navigations = (frameEvents.get(session.session_id) ?? []).filter(event => event.method === 'Page.frameNavigated' &&
        event.seq > session.attach_seq && event.seq <= checkpoint.trace_end_seq && event.params?.frame?.id === frames[0].id && event.params.frame.loaderId === frames[0].loaderId);
      const liveContexts = (contexts.get(session.session_id) ?? []).filter(row => row.created_seq <= checkpoint.trace_end_seq &&
        row.created_seq > session.attach_seq && (!row.retired_seq || row.retired_seq > checkpoint.trace_end_seq) && row.frame_id === frames[0].id);
      if (createdTargets.get(info.targetId)?.seq > checkpoint.trace_end_seq) add('future_target_creation_observation', info.targetId);
      if (!navigations.length) add('missing_frame_navigation_observation', info.targetId);
      else {
        const lastNavigation = navigations.at(-1);
        const detachedAfter = (frameEvents.get(session.session_id) ?? []).some(event => event.method === 'Page.frameDetached' &&
          event.seq > lastNavigation.seq && event.seq <= checkpoint.trace_end_seq && event.params?.frameId === frames[0].id);
        if (detachedAfter) add('current_frame_detached', info.targetId);
      }
      if (!liveContexts.length) add('missing_execution_context_observation', info.targetId);
      memberTrees.push(frameReads[0].result.frameTree);
      members.push({target_id: info.targetId, session_id: session.session_id, type: info.type, url: info.url,
        browser_context_id: contextId, frame_id: frames[0].id, parent_frame_id: frames[0].parentId ?? null,
        target_parent_id: info.parentId ?? null, target_parent_frame_id: info.parentFrameId ?? null,
        loader_id: frames[0].loaderId, isolate_id: isolates[0], execution_contexts: liveContexts.map(row =>
          ({unique_id: row.unique_id, numeric_id: row.numeric_id})).sort((a, b) => JSON.stringify(a) < JSON.stringify(b) ? -1 : 1)});
    }
    if (members.length !== 2) continue;
    // A root-only join can hide child frames and contexts sharing its session.
    // Account for the complete observed tree and every live context, not just the
    // rows that happen to match the expected two members.
    for (const tree of memberTrees) {
      const pending = [tree], seen = new Set();
      let visited = 0;
      while (pending.length) {
        if (++visited > 64) { add('frame_tree_exceeds_bound'); break; }
        const node = pending.pop(), frame = node?.frame;
        const member = members.find(candidate => candidate.frame_id === frame?.id);
        if (!member || seen.has(frame?.id) || frame.loaderId !== member.loader_id || frame.url !== member.url ||
            (frame.parentId ?? null) !== member.parent_frame_id) add('unaccounted_or_contradictory_frame', frame?.id ?? 'missing');
        seen.add(frame?.id);
        if (node?.childFrames !== undefined) {
          if (!Array.isArray(node.childFrames)) add('malformed_frame_children');
          else pending.push(...node.childFrames);
        }
      }
    }
    const uniqueContexts = new Set();
    for (const [sessionId, rows] of contexts) for (const row of rows) {
      if (row.created_seq > checkpoint.trace_end_seq || (row.retired_seq && row.retired_seq <= checkpoint.trace_end_seq)) continue;
      const session = attachments.get(sessionId);
      // Retired session contexts remain separately unqualified below.
      if (session?.detach_seq && session.detach_seq <= checkpoint.trace_end_seq) continue;
      if (session?.destroyed_seq && session.destroyed_seq <= checkpoint.trace_end_seq) continue;
      if (!session || row.created_seq <= session.attach_seq ||
          !members.some(member => member.session_id === sessionId && member.frame_id === row.frame_id)) {
        add('unaccounted_execution_context', `${sessionId}:${row.frame_id}`);
      }
      if (row.unique_id) {
        if (uniqueContexts.has(row.unique_id)) add('duplicate_global_execution_context', row.unique_id);
        uniqueContexts.add(row.unique_id);
      }
    }
    const pageMember = members.find(member => member.type === 'page'), iframeMember = members.find(member => member.type === 'iframe');
    if (pageMember.parent_frame_id !== null || iframeMember.parent_frame_id !== pageMember.frame_id ||
        (iframeMember.target_parent_id !== null && iframeMember.target_parent_id !== pageMember.target_id) ||
        (iframeMember.target_parent_frame_id !== null && iframeMember.target_parent_frame_id !== pageMember.frame_id)) add('invalid_frame_topology', String(checkpointIndex));
    const isolateGroups = Map.groupBy(members, member => member.isolate_id);
    for (const [isolateId, group] of isolateGroups) if (group.length > 1) {
      observations.shared_isolates.push({checkpoint: checkpointIndex, isolate_id: isolateId, members: group.map(memberKey).sort()});
      add('shared_isolate', isolateId);
    }
    observed.members = members.sort((a, b) => a.type.localeCompare(b.type));
    if (previous) for (const member of observed.members) {
      const old = previous.members.find(candidate => candidate.type === member.type);
      if (old && !same(old, member)) {
        const targetReplaced = old.target_id !== member.target_id;
        const destroyed = destroyedTargets.get(old.target_id);
        let explained;
        if (targetReplaced) explained = destroyed > previous.end_seq && destroyed <= checkpoint.trace_end_seq &&
          createdTargets.get(member.target_id)?.seq > previous.end_seq;
        else if (old.session_id !== member.session_id) {
          const oldSession = attachments.get(old.session_id), newSession = attachments.get(member.session_id);
          explained = oldSession?.detach_seq > previous.end_seq && oldSession.detach_seq <= checkpoint.trace_end_seq &&
            newSession?.attach_seq > previous.end_seq && newSession.attach_seq <= checkpoint.trace_end_seq;
        } else {
          explained = true;
          if (old.frame_id !== member.frame_id || old.loader_id !== member.loader_id) {
            const navigated = (frameEvents.get(member.session_id) ?? []).some(event => event.method === 'Page.frameNavigated' &&
              event.seq > previous.end_seq && event.seq <= checkpoint.trace_end_seq &&
              event.params?.frame?.id === member.frame_id && event.params.frame.loaderId === member.loader_id);
            const retired = old.execution_contexts.every(identity => (contexts.get(member.session_id) ?? []).some(row =>
              row.unique_id === identity.unique_id && row.numeric_id === identity.numeric_id &&
              row.retired_seq > previous.end_seq && row.retired_seq <= checkpoint.trace_end_seq));
            const created = member.execution_contexts.some(identity => (contexts.get(member.session_id) ?? []).some(row =>
              row.unique_id === identity.unique_id && row.numeric_id === identity.numeric_id &&
              row.created_seq > previous.end_seq && row.created_seq <= checkpoint.trace_end_seq));
            explained &&= navigated && retired && created;
          } else if (old.isolate_id !== member.isolate_id) {
            const retired = old.execution_contexts.every(identity => (contexts.get(member.session_id) ?? []).some(row =>
              row.unique_id === identity.unique_id && row.numeric_id === identity.numeric_id && row.retired_seq > previous.end_seq && row.retired_seq <= checkpoint.trace_end_seq));
            const created = member.execution_contexts.some(identity => (contexts.get(member.session_id) ?? []).some(row =>
              row.unique_id === identity.unique_id && row.numeric_id === identity.numeric_id && row.created_seq > previous.end_seq && row.created_seq <= checkpoint.trace_end_seq));
            explained &&= retired && created;
          }
        }
        observations.replacements.push({type: member.type, from: old, to: member, explained: Boolean(explained)});
        if (!explained) add('unexplained_replacement', member.type);
      }
    }
    previous = {members: observed.members, end_seq: checkpoint.trace_end_seq};
  }

  const finalBoundary = checkpoints.at(-1)?.trace_end_seq ?? events.length;
  for (const session of attachments.values()) if (session.detach_seq || session.destroyed_seq) {
    const destroyed = session.destroyed_seq && session.destroyed_seq >= session.detach_seq;
    const retiredSeq = session.detach_seq ?? session.destroyed_seq;
    const afterMeasurement = retiredSeq > finalBoundary;
    observations.retired_members.push({...session, target_destroyed: Boolean(destroyed), after_measurement: afterMeasurement});
    if (!afterMeasurement) add(destroyed ? 'retired_member_unqualified' : 'detached_without_target_destruction', session.session_id);
  }
  const lastMembers = observations.checkpoints.at(-1)?.members ?? [];
  const liveApplication = [...attachments.values()].filter(session => !INTERNAL_TYPES.has(session.type) &&
    session.attach_seq <= finalBoundary && (!session.detach_seq || session.detach_seq > finalBoundary) &&
    (!session.destroyed_seq || session.destroyed_seq > finalBoundary));
  const complete = lastMembers.length === 2 && liveApplication.length === 2 &&
    liveApplication.every(session => lastMembers.some(member => member.target_id === session.target_id && member.session_id === session.session_id));
  if (!complete) add('live_membership_incomplete');
  result.live_membership_complete = complete;
  result.evidence_valid = reasons.length === 0;
  return result;
}
