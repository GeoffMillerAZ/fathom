// Flattened browser-level CDP containment guard. The caller owns Chrome and its
// fresh profile; this module never launches a process or changes cache behavior.
const TIMEOUT_MS = 5000;
const RELATED_TYPES = new Set(['page', 'iframe']);
const NETWORK_TYPES = new Set(['page', 'iframe', 'worker', 'service_worker', 'shared_worker']);
const FIXED_SHAPING = Object.freeze({latency: 100, downloadThroughput: 200000, uploadThroughput: 64000});

function exactBrowserWS(raw) {
  const url = new URL(raw);
  if (url.protocol !== 'ws:' || !['127.0.0.1', '[::1]'].includes(url.hostname) || !url.port ||
      url.username || url.password || url.search || url.hash || !/^\/devtools\/browser\/[^/]+$/.test(url.pathname) || raw !== url.href) {
    throw new Error('Browser CDP URL must be an exact loopback ws:// origin with /devtools/browser/<id>.');
  }
  return url.href;
}

function exactAllowedOrigin(raw) {
  const url = new URL(raw);
  if (url.protocol !== 'http:' || !['127.0.0.1', '[::1]'].includes(url.hostname) || !url.port ||
      url.username || url.password || url.pathname !== '/' || url.search || url.hash || raw !== url.origin) {
    throw new Error('allowedOrigin must be an exact HTTP loopback IP origin.');
  }
  return url.origin;
}

function selectedShaping(value) {
  if (!value) return null;
  if (value === true) return {...FIXED_SHAPING};
  if (typeof value !== 'object' || Object.keys(value).length !== Object.keys(FIXED_SHAPING).length ||
      Object.keys(FIXED_SHAPING).some(key => value[key] !== FIXED_SHAPING[key])) {
    throw new Error('shaping must be true or exactly {latency:100, downloadThroughput:200000, uploadThroughput:64000}.');
  }
  return {...FIXED_SHAPING};
}

function errorText(error) {
  return error instanceof Error ? `${error.name}: ${error.message}` : String(error);
}

export async function connectGuard(wsURL, {allowedOrigin, events, shaping, protocolEvents} = {}) {
  const endpoint = exactBrowserWS(wsURL), origin = exactAllowedOrigin(allowedOrigin);
  if (!Array.isArray(events)) throw new Error('events must be a caller-owned array.');
  if (protocolEvents !== undefined && !Array.isArray(protocolEvents)) throw new Error('protocolEvents must be a caller-owned array when supplied.');
  const conditions = selectedShaping(shaping);
  const socket = new WebSocket(endpoint);
  const pending = new Map(), setupTasks = new Set(), sessions = new Map();
  let commandId = 0, protocolSequence = 0, disconnected = false, closeRequested = false;
  const coverage = {
    endpoint, allowedOrigin: origin, shaping: conditions, rootAutoAttach: false,
    ready: false, failed: false, closed: false, sessions: [], errors: [], detach: [],
    rawNetworkEvents: [], ruleIds: [], appliedRuleEvents: [], unpausedAttachments: [],
    excludedTargetTypes: ['browser', 'tab', 'browser_ui'],
    targetScope: 'App documents, iframes and workers; browser-internal UI is explicitly excluded, not OS-wide containment.',
  };

  const recordError = (scope, error, detail = {}) => {
    const item = {at: new Date().toISOString(), scope, error: errorText(error), ...detail};
    coverage.failed = true; coverage.errors.push(item); events.push({kind: 'guard-error', ...item});
    return item;
  };
  const rejectPending = error => {
    for (const entry of pending.values()) { clearTimeout(entry.timer); entry.reject(error); }
    pending.clear();
  };
  const retainProtocol = item => {
    if (protocolEvents) protocolEvents.push({seq: ++protocolSequence, ...structuredClone(item)});
  };
  await new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('Browser CDP WebSocket open timeout.')), TIMEOUT_MS);
    socket.addEventListener('open', () => { clearTimeout(timer); resolve(); }, {once: true});
    socket.addEventListener('error', () => { clearTimeout(timer); reject(new Error('Browser CDP WebSocket failed before open.')); }, {once: true});
  });

  const send = (method, params = {}, sessionId) => {
    if (disconnected || socket.readyState !== WebSocket.OPEN) return Promise.reject(new Error(`CDP disconnected before ${method}.`));
    const id = ++commandId, message = {id, method, params};
    if (sessionId) message.sessionId = sessionId;
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        pending.delete(id); const error = new Error(`CDP command timeout: ${method}${sessionId ? ` session=${sessionId}` : ''}`);
        recordError('command-timeout', error, {method, sessionId: sessionId ?? null}); reject(error);
      }, TIMEOUT_MS);
      pending.set(id, {method, sessionId, resolve, reject, timer});
      try {
        socket.send(JSON.stringify(message));
        if (method.startsWith('Target.')) retainProtocol({kind: 'command', direction: 'outbound', sent_at: new Date().toISOString(), id, method, sessionId: sessionId ?? null, params});
      }
      catch (error) { clearTimeout(timer); pending.delete(id); reject(error); }
    });
  };

  const configure = async (params, parentSessionId = null) => {
    const {sessionId, targetInfo, waitingForDebugger} = params;
    const target = {
      sessionId, parentSessionId, targetId: targetInfo.targetId, type: targetInfo.type,
      url: targetInfo.url, waitingForDebugger, recursiveAutoAttach: false,
      networkEnabled: false, blockedURLs: false, appliedPolicy: false,
      ruleIds: [], resumed: false, detached: false, error: null,
    };
    sessions.set(sessionId, target); coverage.sessions.push(target);
    events.push({kind: 'attached', at: new Date().toISOString(), ...target});
    if (!waitingForDebugger) coverage.unpausedAttachments.push({sessionId, targetId: target.targetId, type: target.type, url: target.url});
    try {
      if (!NETWORK_TYPES.has(target.type)) throw new Error(`Unsupported auto-attached target type: ${target.type}`);
      if (RELATED_TYPES.has(target.type)) {
        await send('Target.setAutoAttach', {
          autoAttach: true, waitForDebuggerOnStart: true, flatten: true,
          filter: [{type: 'browser', exclude: true}, {type: 'tab', exclude: true}, {type: 'browser_ui', exclude: true}, {}],
        }, sessionId);
        target.recursiveAutoAttach = true;
      }
      await send('Network.enable', {}, sessionId); target.networkEnabled = true;
      await send('Network.setBlockedURLs', {
        urlPatterns: [{urlPattern: `${origin}/*`, block: false}], urls: ['*'],
      }, sessionId); target.blockedURLs = true;
      if (conditions) {
        const result = await send('Network.emulateNetworkConditionsByRule', {
          offline: false,
          matchedNetworkConditions: [{urlPattern: '', offline: false, ...conditions}],
        }, sessionId);
        if (!Array.isArray(result?.ruleIds) || result.ruleIds.length !== 1 || typeof result.ruleIds[0] !== 'string') {
          throw new Error('Network shaping did not return exactly one rule id.');
        }
        target.ruleIds = [...result.ruleIds]; coverage.ruleIds.push(...result.ruleIds.map(ruleId => ({sessionId, ruleId})));
      }
      target.appliedPolicy = true;
      await send('Runtime.runIfWaitingForDebugger', {}, sessionId); target.resumed = true;
      events.push({kind: 'policy-applied', at: new Date().toISOString(), sessionId, targetId: target.targetId, ruleIds: target.ruleIds});
    } catch (error) {
      target.error = errorText(error); recordError('target-setup', error, {sessionId, targetId: target.targetId, type: target.type});
      throw error; // Never resume a target whose complete policy was not applied.
    }
  };

  const onAttached = (params, parentSessionId) => {
    const task = configure(params, parentSessionId);
    setupTasks.add(task);
    task.catch(() => {}).finally(() => setupTasks.delete(task));
  };
  const receive = async event => {
    try {
      const raw = typeof event.data === 'string' ? event.data : await event.data.text();
      const message = JSON.parse(raw);
      if (message.id) {
        const entry = pending.get(message.id); if (!entry) return;
        clearTimeout(entry.timer); pending.delete(message.id);
        if (entry.method.startsWith('Target.')) retainProtocol({kind: 'response', direction: 'inbound', received_at: new Date().toISOString(), id: message.id, method: entry.method, sessionId: entry.sessionId ?? null, ...(message.error ? {error: message.error} : {result: message.result})});
        if (message.error) {
          const error = new Error(`CDP ${entry.method}: ${JSON.stringify(message.error)}`);
          recordError('command', error, {method: entry.method, sessionId: entry.sessionId ?? null}); entry.reject(error);
        }
        else entry.resolve(message.result ?? {});
        return;
      }
      if (message.method?.startsWith('Target.')) retainProtocol({kind: 'event', direction: 'inbound', received_at: new Date().toISOString(), method: message.method, sessionId: message.sessionId ?? null, params: message.params});
      if (message.method === 'Target.attachedToTarget') onAttached(message.params, message.sessionId ?? null);
      else if (message.method === 'Target.detachedFromTarget') {
        const item = {at: new Date().toISOString(), parentSessionId: message.sessionId ?? null, ...message.params};
        coverage.detach.push(item); const target = sessions.get(message.params.sessionId); if (target) target.detached = true;
        events.push({kind: 'detached', ...item});
      }
      if (message.method?.startsWith('Network.')) {
        const item = {at: new Date().toISOString(), sessionId: message.sessionId ?? null, method: message.method, params: message.params};
        coverage.rawNetworkEvents.push(item); events.push({kind: 'network', ...item});
        if (message.params?.appliedNetworkConditionsId) coverage.appliedRuleEvents.push({
          sessionId: item.sessionId, method: item.method, requestId: message.params.requestId,
          ruleId: message.params.appliedNetworkConditionsId,
        });
      }
    } catch (error) { recordError('receive', error); }
  };
  socket.addEventListener('message', receive);
  socket.addEventListener('error', () => recordError('websocket', new Error('Browser CDP WebSocket error.')));
  socket.addEventListener('close', event => {
    disconnected = true; coverage.closed = true;
    const error = new Error(`Browser CDP WebSocket closed (${event.code} ${event.reason || 'no reason'}).`);
    if (!closeRequested) recordError('disconnect', error);
    rejectPending(error); events.push({kind: 'closed', at: new Date().toISOString(), code: event.code, reason: event.reason});
  });

  const drainSetup = async () => {
    for (;;) {
      if (setupTasks.size) await Promise.allSettled([...setupTasks]);
      await new Promise(resolve => setTimeout(resolve, 0));
      if (!setupTasks.size) return;
    }
  };
  const ready = (async () => {
    try {
      await send('Target.setAutoAttach', {
        autoAttach: true, waitForDebuggerOnStart: true, flatten: true,
        filter: [{type: 'browser', exclude: true}, {type: 'tab', exclude: true}, {type: 'browser_ui', exclude: true}, {}],
      });
      coverage.rootAutoAttach = true; await drainSetup();
      if (coverage.errors.length) throw new AggregateError(coverage.errors.map(item => new Error(item.error)), 'CDP guard setup failed.');
      coverage.ready = true; return coverage;
    } catch (error) {
      if (!coverage.errors.some(item => item.error === errorText(error))) recordError('root-setup', error);
      throw error;
    }
  })();
  ready.catch(() => {});

  const close = async () => {
    if (coverage.closed || disconnected) return;
    const unsafe = coverage.sessions.filter(target => target.waitingForDebugger && !target.resumed && !target.detached);
    if (unsafe.length) throw new Error(`Refusing to detach with ${unsafe.length} paused unguarded target(s); terminate the owned browser.`);
    closeRequested = true;
    await new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error('Browser CDP WebSocket close timeout.')), TIMEOUT_MS);
      socket.addEventListener('close', () => { clearTimeout(timer); resolve(); }, {once: true}); socket.close(1000, 'guard close');
    });
  };
  return {send, ready, close, coverage};
}
