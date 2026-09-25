// Diagnostic-only, local browser instrumentation for a bounded cancellation trace.
//
// This deliberately observes just the two stream cancellation entry points.  It does
// not wrap fetch(), getReader(), clone(), or tee(); it captures no request identity,
// body, reason, cookies, or source data.  Installing it changes native function
// identity and adds overhead, so its observations are never a performance sample or
// causal per-request proof.

// This function is self-contained so callers may serialize it for
// browserContext.addInitScript({content: `(${installStreamCancelObserver})()`}).
export function installStreamCancelObserver(options = {}) {
  const key = '__appkitStreamCancelObserverV1__';
  const version = 1;
  const root = globalThis;
  const hasOwn = Object.prototype.hasOwnProperty.call(root, key);
  if (hasOwn) {
    const current = root[key];
    return {
      available: false,
      status: current && current.version === version ? 'refused-already-installed' : 'refused-global-conflict',
      version,
    };
  }

  const streamPrototype = root.ReadableStream?.prototype;
  const readerPrototype = root.ReadableStreamDefaultReader?.prototype;
  const streamDescriptor = streamPrototype && Object.getOwnPropertyDescriptor(streamPrototype, 'cancel');
  const readerDescriptor = readerPrototype && Object.getOwnPropertyDescriptor(readerPrototype, 'cancel');
  if (!streamDescriptor || typeof streamDescriptor.value !== 'function' || !streamDescriptor.configurable ||
      !readerDescriptor || typeof readerDescriptor.value !== 'function' || !readerDescriptor.configurable) {
    return {available: false, status: 'refused-unsupported', version};
  }
  if (!Object.isExtensible(root)) return {available: false, status: 'refused-global-not-installable', version};

  const capacity = Number.isInteger(options.capacity) && options.capacity > 0 ? Math.min(options.capacity, 256) : 64;
  const stackLimit = Number.isInteger(options.stackLimit) && options.stackLimit > 0 ? Math.min(options.stackLimit, 1024) : 320;
  const record = {
    version,
    number_calls: 0,
    dropped_count: 0,
    entries: [],
    capacity,
    stackLimit,
    streamPrototype,
    readerPrototype,
    streamDescriptor,
    readerDescriptor,
    globalDescriptor: undefined,
  };
  const capture = method => {
    // Diagnostics must never affect cancellation, including if performance or stack
    // capture is unavailable in an unusual browser realm.
    try {
      record.number_calls += 1;
      if (record.entries.length >= record.capacity) {
        record.dropped_count += 1;
        return;
      }
      let timestamp = null;
      try { timestamp = typeof performance?.now === 'function' ? performance.now() : null; } catch {}
      let stack = '';
      try { stack = String(new Error().stack ?? ''); } catch {}
      record.entries.push({
        method,
        timestamp_ms: Number.isFinite(timestamp) ? timestamp : null,
        stack: {value: stack.slice(0, record.stackLimit), truncated: stack.length > record.stackLimit},
      });
    } catch {}
  };
  const originalStreamCancel = streamDescriptor.value;
  const originalReaderCancel = readerDescriptor.value;
  const observedStreamCancel = function (...args) {
    capture('ReadableStream.prototype.cancel');
    return Reflect.apply(originalStreamCancel, this, args);
  };
  const observedReaderCancel = function (...args) {
    capture('ReadableStreamDefaultReader.prototype.cancel');
    return Reflect.apply(originalReaderCancel, this, args);
  };

  try {
    Object.defineProperty(streamPrototype, 'cancel', {...streamDescriptor, value: observedStreamCancel});
    Object.defineProperty(readerPrototype, 'cancel', {...readerDescriptor, value: observedReaderCancel});
    Object.defineProperty(root, key, {value: record, configurable: true, enumerable: false, writable: false});
    record.globalDescriptor = Object.getOwnPropertyDescriptor(root, key);
  } catch {
    // Best-effort rollback keeps a partial failed install from altering stream behavior.
    try { Object.defineProperty(streamPrototype, 'cancel', streamDescriptor); } catch {}
    try { Object.defineProperty(readerPrototype, 'cancel', readerDescriptor); } catch {}
    try { delete root[key]; } catch {}
    return {available: false, status: 'refused-install-failed', version};
  }
  return {available: true, status: 'installed', version, capacity, stackLimit};
}

// Also self-contained so it can be serialized independently from installation.
export function streamCancelObserverSnapshot() {
  const key = '__appkitStreamCancelObserverV1__';
  const version = 1;
  const record = globalThis[key];
  if (!record || record.version !== version) return {available: false, status: 'not-installed', version};
  return {
    available: true,
    status: 'installed',
    version,
    number_calls: record.number_calls,
    dropped_count: record.dropped_count,
    entries: record.entries.map(entry => ({
      method: entry.method,
      timestamp_ms: entry.timestamp_ms,
      stack: {value: entry.stack.value, truncated: entry.stack.truncated},
    })),
    limitations: 'Diagnostic-only: installation changes function identity and adds overhead; not a performance sample or causal per-request proof.',
  };
}

// Restoration is intended for fixture teardown only. It restores both method
// descriptors and removes the versioned global, without observing any new calls.
export function restoreStreamCancelObserver() {
  const key = '__appkitStreamCancelObserverV1__';
  const version = 1;
  const record = globalThis[key];
  if (!record || record.version !== version) return {available: false, status: 'not-installed', version};
  try {
    Object.defineProperty(record.streamPrototype, 'cancel', record.streamDescriptor);
    Object.defineProperty(record.readerPrototype, 'cancel', record.readerDescriptor);
    if (!delete globalThis[key]) throw new Error('global removal refused');
    return {available: true, status: 'restored', version};
  } catch {
    return {available: false, status: 'refused-restore-failed', version};
  }
}
