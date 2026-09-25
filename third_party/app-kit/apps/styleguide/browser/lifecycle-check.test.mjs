import assert from 'node:assert/strict';
import test from 'node:test';
import {joinGuideLifecycle} from './lifecycle-check.mjs';

const origin = 'http://127.0.0.1:18080', contextId = 'owned-context';

function fixture({shared = false} = {}) {
  const events = [], commands = new Map(); let seq = 0, commandId = 0;
  const push = row => events.push({seq: ++seq, monotonic_ms: seq, ...row});
  const event = (method, params, sessionId = null) => push({kind: 'event', direction: 'inbound', method, sessionId, params});
  const command = (method, params = {}, sessionId = null, result = {}) => {
    const id = ++commandId, commandSeq = seq + 1;
    push({kind: 'command', direction: 'outbound', id, method, sessionId, params});
    push({kind: 'response', direction: 'inbound', id, method, sessionId, result});
    commands.set(id, {id, commandSeq, responseSeq: seq, method, sessionId, params, result: structuredClone(result)});
    return commands.get(id);
  };
  command('Target.setDiscoverTargets', {discover: true});
  command('Target.setAutoAttach', {autoAttach: true});
  const definitions = [
    {type: 'page', targetId: 'guide', sessionId: 'guide-session', url: origin + '/theme-lab', frameId: 'guide-frame', parentId: undefined},
    {type: 'iframe', targetId: 'preview', sessionId: 'preview-session', url: origin + '/theme-lab/preview?scheme=light', frameId: 'preview-frame', parentId: 'guide-frame'},
  ];
  for (const target of definitions) {
    const info = {targetId: target.targetId, type: target.type, url: target.url, browserContextId: contextId};
    event('Target.targetCreated', {targetInfo: info});
    event('Target.attachedToTarget', {sessionId: target.sessionId, waitingForDebugger: true, targetInfo: info});
    command('Target.setAutoAttach', {autoAttach: true, waitForDebuggerOnStart: true, flatten: true}, target.sessionId);
    command('Network.enable', {}, target.sessionId);
    command('Network.setBlockedURLs', {urlPatterns: [{urlPattern: `${origin}/*`, block: false}], urls: ['*']}, target.sessionId);
    command('Page.enable', {}, target.sessionId);
    const frame = {id: target.frameId, loaderId: `${target.frameId}-loader`, url: target.url,
      securityOrigin: origin, mimeType: 'text/html', ...(target.parentId ? {parentId: target.parentId} : {})};
    event('Page.frameNavigated', {frame}, target.sessionId);
    command('Runtime.enable', {}, target.sessionId);
    event('Runtime.executionContextCreated', {context: {id: target.type === 'page' ? 1 : 2,
      uniqueId: `${target.frameId}-context`, auxData: {frameId: target.frameId, isDefault: true}}}, target.sessionId);
    command('Page.setLifecycleEventsEnabled', {enabled: true}, target.sessionId);
    command('Page.getFrameTree', {}, target.sessionId, {frameTree: {frame}});
    command('Runtime.getIsolateId', {}, target.sessionId, {id: shared ? 'shared-isolate' : `${target.type}-isolate`});
    command('Runtime.runIfWaitingForDebugger', {}, target.sessionId);
  }
  function checkpoint(label = 'baseline') {
    const start = events.length, reads = [];
    const infos = definitions.map(target => ({targetId: target.targetId, type: target.type, url: target.url, browserContextId: contextId}));
    infos.push({targetId: 'internal', type: 'browser_ui', url: 'chrome://settings'});
    const read = (method, sessionId, result) => {
      const joined = command(method, {}, sessionId, result);
      reads.push({method, session_id: sessionId, result: structuredClone(result), command_id: joined.id,
        command_seq: joined.commandSeq, response_seq: joined.responseSeq,
        started_ms: joined.commandSeq, completed_ms: joined.responseSeq});
    };
    read('Target.getTargets', null, {targetInfos: infos});
    for (const target of definitions) {
      read('Runtime.getIsolateId', target.sessionId, {id: shared ? 'shared-isolate' : `${target.type}-isolate`});
      const frame = {id: target.frameId, loaderId: `${target.frameId}-loader`, url: target.url,
        securityOrigin: origin, mimeType: 'text/html', ...(target.parentId ? {parentId: target.parentId} : {})};
      read('Page.getFrameTree', target.sessionId, {frameTree: {frame}});
    }
    for (const target of definitions) {
      read('Runtime.getIsolateId', target.sessionId, {id: shared ? 'shared-isolate' : `${target.type}-isolate`});
      const frame = {id: target.frameId, loaderId: `${target.frameId}-loader`, url: target.url,
        securityOrigin: origin, mimeType: 'text/html', ...(target.parentId ? {parentId: target.parentId} : {})};
      read('Page.getFrameTree', target.sessionId, {frameTree: {frame}});
    }
    read('Target.getTargets', null, {targetInfos: infos});
    return {label, trace_start_seq: start, trace_end_seq: events.length, reads,
      started_ms: start, completed_ms: events.length};
  }
  const checkpoints = [checkpoint()];
  const run = () => joinGuideLifecycle({events, checkpoints, contextId, origin});
  return {events, checkpoints, definitions, event, command, checkpoint, run};
}

test('joins distinct endpoint membership without promoting memory or attribution', () => {
  const f = fixture(), before = structuredClone({events: f.events, checkpoints: f.checkpoints});
  const result = f.run();
  assert.equal(result.evidence_valid, true, JSON.stringify(result.unqualified_reasons));
  assert.equal(result.live_membership_complete, true);
  assert.deepEqual(result.unqualified_reasons, []);
  assert.equal(result.observations.checkpoints[0].members.length, 2);
  assert.deepEqual(result.observations.shared_isolates, []);
  assert.equal(result.attribution, 'not-verified');
  assert.equal(result.memory_complete, false);
  assert.equal(result.full_contract_complete, false);
  assert.deepEqual({events: f.events, checkpoints: f.checkpoints}, before);
});

test('shared isolate is retained and explicitly unqualified', () => {
  const result = fixture({shared: true}).run();
  assert.equal(result.evidence_valid, false);
  assert.equal(result.live_membership_complete, true);
  assert.equal(result.observations.shared_isolates.length, 1);
  assert.ok(result.unqualified_reasons.some(reason => reason.code === 'shared_isolate'));
});

test('retired detached member is not treated as destroyed or reclaimed', () => {
  const f = fixture();
  f.event('Target.detachedFromTarget', {sessionId: 'preview-session', targetId: 'preview'});
  f.checkpoints[0].trace_end_seq = f.events.length;
  const result = f.run();
  assert.equal(result.evidence_valid, false);
  assert.equal(result.observations.retired_members[0].target_destroyed, false);
  assert.ok(result.unqualified_reasons.some(reason => reason.code === 'detached_without_target_destruction'));
});

test('causally explained target replacement across checkpoints is retained', () => {
  const f = fixture();
  f.event('Target.detachedFromTarget', {sessionId: 'preview-session'});
  f.event('Target.targetDestroyed', {targetId: 'preview'});
  const target = f.definitions[1];
  Object.assign(target, {targetId: 'preview-2', sessionId: 'preview-session-2', frameId: 'preview-frame-2'});
  const info = {targetId: target.targetId, type: target.type, url: target.url, browserContextId: contextId};
  f.event('Target.targetCreated', {targetInfo: info});
  f.event('Target.attachedToTarget', {sessionId: target.sessionId, waitingForDebugger: true, targetInfo: info});
  f.command('Target.setAutoAttach', {autoAttach: true, waitForDebuggerOnStart: true, flatten: true}, target.sessionId);
  f.command('Network.enable', {}, target.sessionId);
  f.command('Network.setBlockedURLs', {urlPatterns: [{urlPattern: `${origin}/*`, block: false}], urls: ['*']}, target.sessionId);
  f.command('Page.enable', {}, target.sessionId);
  const frame = {id: target.frameId, parentId: 'guide-frame', loaderId: 'preview-frame-2-loader', url: target.url};
  f.event('Page.frameNavigated', {frame}, target.sessionId);
  f.command('Runtime.enable', {}, target.sessionId);
  f.event('Runtime.executionContextCreated', {context: {uniqueId: 'preview-2-context', auxData: {frameId: target.frameId}}}, target.sessionId);
  f.command('Page.setLifecycleEventsEnabled', {enabled: true}, target.sessionId);
  f.command('Page.getFrameTree', {}, target.sessionId, {frameTree: {frame}});
  f.command('Runtime.getIsolateId', {}, target.sessionId, {id: 'iframe-isolate-2'});
  f.command('Runtime.runIfWaitingForDebugger', {}, target.sessionId);
  f.checkpoints.push(f.checkpoint('final'));
  const result = f.run();
  assert.equal(result.observations.replacements.length, 1);
  assert.equal(result.observations.replacements[0].explained, true);
  assert.equal(result.live_membership_complete, true);
  assert.equal(result.evidence_valid, false);
  assert.ok(result.unqualified_reasons.some(reason => reason.code === 'retired_member_unqualified'));
});

const corruptions = [
  ['mismatched checkpoint read', f => { f.checkpoints[0].reads[0].result = {targetInfos: []}; }, 'checkpoint_read_mismatch'],
  ['premature resume', f => {
    const commandIndex = f.events.findIndex(e => e.kind === 'command' && e.method === 'Runtime.runIfWaitingForDebugger');
    const pair = f.events.splice(commandIndex, 2); const attachment = f.events.findIndex(e => e.method === 'Target.attachedToTarget');
    f.events.splice(attachment + 1, 0, ...pair); f.events.forEach((e, i) => { e.seq = i + 1; e.monotonic_ms = i + 1; });
    for (const checkpoint of f.checkpoints) for (const read of checkpoint.reads) {
      read.command_seq = f.events.find(e => e.kind === 'command' && e.id === read.command_id).seq;
      read.response_seq = f.events.find(e => e.kind === 'response' && e.id === read.command_id).seq;
    }
  }, 'unguarded_live_session'],
  ['unknown target', f => { f.checkpoints[0].reads[0].result.targetInfos.push({targetId: 'worker', type: 'worker', url: origin + '/worker', browserContextId: contextId});
    f.checkpoints[0].reads.at(-1).result.targetInfos.push({targetId: 'worker', type: 'worker', url: origin + '/worker', browserContextId: contextId});
    const commands = f.events.filter(e => e.kind === 'response' && e.method === 'Target.getTargets');
    commands[0].result = structuredClone(f.checkpoints[0].reads[0].result); commands[1].result = structuredClone(f.checkpoints[0].reads.at(-1).result); }, 'incomplete_or_unknown_membership'],
  ['missing execution context', f => { const row = f.events.find(e => e.method === 'Runtime.executionContextCreated' && e.sessionId === 'preview-session'); row.method = 'Page.lifecycleEvent'; row.params = {name: 'load'}; }, 'missing_execution_context_observation'],
  ['duplicate command id', f => { const rows = f.events.filter(e => e.kind === 'command'); rows[1].id = rows[0].id; }, 'ambiguous_command'],
  ['duplicate checkpoint read', f => { f.checkpoints[0].reads[2] = structuredClone(f.checkpoints[0].reads[1]); }, 'ambiguous_checkpoint_reads'],
  ['out of order trace', f => { f.events[2].monotonic_ms = 0; }, 'invalid_trace_order'],
];
for (const [name, mutate, code] of corruptions) test(`fails closed: ${name}`, () => {
  const f = fixture(); mutate(f); const result = f.run();
  assert.equal(result.evidence_valid, false);
  assert.ok(result.unqualified_reasons.some(reason => reason.code === code), JSON.stringify(result.unqualified_reasons));
  assert.equal(result.attribution, 'not-verified');
});

test('missing boundary context and oversized inputs refuse before analysis', () => {
  const f = fixture();
  assert.throws(() => joinGuideLifecycle({events: f.events, checkpoints: f.checkpoints, contextId: '', origin}), /contextId/);
  assert.throws(() => joinGuideLifecycle({events: Array(50001), checkpoints: f.checkpoints, contextId, origin}), /bounded/);
});

test('post-final cleanup does not retroactively invalidate measured membership', () => {
  const f = fixture();
  f.event('Target.detachedFromTarget', {sessionId: 'preview-session'});
  f.event('Target.targetDestroyed', {targetId: 'preview'});
  const result = f.run();
  assert.equal(result.evidence_valid, true, JSON.stringify(result.unqualified_reasons));
  assert.equal(result.live_membership_complete, true);
  assert.equal(result.observations.retired_members[0].after_measurement, true);
});

test('void execution-context clear after measurement is retained without rewriting the endpoint', () => {
  const f = fixture();
  f.event('Runtime.executionContextsCleared', undefined, 'preview-session');
  const result = f.run();
  assert.equal(result.evidence_valid, true, JSON.stringify(result.unqualified_reasons));
  assert.equal(result.observations.checkpoints[0].members.length, 2);
});

test('malformed target destruction is a fail-closed observation, not an exception', () => {
  const f = fixture(); f.event('Target.targetDestroyed', {});
  const result = f.run();
  assert.equal(result.evidence_valid, false);
  assert.ok(result.unqualified_reasons.some(reason => reason.code === 'unknown_or_duplicate_target_destruction'));
});

test('change-away-or-back during a checkpoint cannot hide behind equal inventories', () => {
  const f = fixture();
  const info = {targetId: 'guide', type: 'page', url: origin + '/theme-lab', browserContextId: contextId};
  f.event('Target.targetInfoChanged', {targetInfo: info});
  f.checkpoints[0].trace_end_seq = f.events.length;
  const result = f.run();
  assert.equal(result.evidence_valid, false);
  assert.ok(result.unqualified_reasons.some(reason => reason.code === 'identity_changed_during_checkpoint'));
});
