// Assessment-only cleanup for one exact browser context already owned by a caller.
// This module does not launch, close, or signal a browser process.

const stamp = () => new Date().toISOString();

function requireHealthy(guard) {
  if (!guard.coverage || typeof guard.coverage !== 'object' ||
      guard.coverage.failed !== false || guard.coverage.closed !== false) {
    throw new Error('guard coverage must be healthy and open');
  }
}

function requireObject(value, label) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new TypeError(`${label} must be an object`);
  }
  return value;
}

function requireIDs(value, label) {
  if (!Array.isArray(value) || value.some(id => typeof id !== 'string' || id.length === 0) ||
      new Set(value).size !== value.length) {
    throw new Error(`${label} must be an array of unique nonempty strings`);
  }
  return value;
}

function requireContexts(response) {
  return requireIDs(requireObject(response, 'Target.getBrowserContexts response').browserContextIds,
    'Target.getBrowserContexts browserContextIds');
}

function requireTargets(response) {
  const {targetInfos} = requireObject(response, 'Target.getTargets response');
  if (!Array.isArray(targetInfos)) throw new Error('Target.getTargets targetInfos must be an array');
  const ids = new Set();
  for (const target of targetInfos) {
    requireObject(target, 'Target.getTargets targetInfo');
    if (typeof target.targetId !== 'string' || target.targetId.length === 0 || ids.has(target.targetId)) {
      throw new Error('Target.getTargets targetIds must be unique nonempty strings');
    }
    if (target.browserContextId !== undefined &&
        (typeof target.browserContextId !== 'string' || target.browserContextId.length === 0)) {
      throw new Error('Target.getTargets browserContextId must be a nonempty string when present');
    }
    ids.add(target.targetId);
  }
  return targetInfos;
}

async function command(guard, events, method, params = {}) {
  requireHealthy(guard);
  const item = {at: stamp(), method, params: structuredClone(params)};
  events.push({kind: 'command', ...item});
  try {
    const response = await guard.send(method, params);
    events.push({kind: 'response', ...item, at: stamp(), response: structuredClone(response)});
    requireHealthy(guard);
    return response;
  } catch (error) {
    events.push({kind: 'error', ...item, at: stamp(), error: String(error)});
    throw error;
  }
}

export async function disposeOwnedContext({guard, contextId, events} = {}) {
  if (!guard || typeof guard !== 'object' || typeof guard.send !== 'function') {
    throw new TypeError('guard.send must be callable');
  }
  requireHealthy(guard);
  if (typeof contextId !== 'string' || contextId.length === 0) {
    throw new TypeError('contextId must be an exact nonempty string');
  }
  if (!Array.isArray(events)) throw new TypeError('events must be a caller-owned array');

  const beforeContexts = requireContexts(await command(guard, events, 'Target.getBrowserContexts'));
  if (!beforeContexts.includes(contextId)) throw new Error('owned browser context was not found');
  const unrelatedContextIds = beforeContexts.filter(id => id !== contextId);

  const beforeTargets = requireTargets(await command(guard, events, 'Target.getTargets'));
  const ownedTargetIds = beforeTargets.filter(target => target.browserContextId === contextId).map(target => target.targetId);

  const disposed = await command(guard, events, 'Target.disposeBrowserContext', {browserContextId: contextId});
  requireObject(disposed, 'Target.disposeBrowserContext response');

  const afterContexts = requireContexts(await command(guard, events, 'Target.getBrowserContexts'));
  if (afterContexts.includes(contextId)) throw new Error('owned browser context remains after disposal');
  if (unrelatedContextIds.some(id => !afterContexts.includes(id))) {
    throw new Error('an unrelated browser context disappeared during disposal');
  }

  const afterTargets = requireTargets(await command(guard, events, 'Target.getTargets'));
  const afterTargetIds = new Set(afterTargets.map(target => target.targetId));
  if (afterTargets.some(target => target.browserContextId === contextId) ||
      ownedTargetIds.some(id => afterTargetIds.has(id))) {
    throw new Error('owned browser-context target remains after disposal');
  }

  return {
    contextId,
    contextClosed: true,
    targetIds: ownedTargetIds,
    targetsClosed: true,
  };
}
