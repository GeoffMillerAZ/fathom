import assert from 'node:assert/strict';
import test from 'node:test';
import {
  installStreamCancelObserver,
  restoreStreamCancelObserver,
  streamCancelObserverSnapshot,
} from './stream-cancel-observer.mjs';

const key = '__appkitStreamCancelObserverV1__';

function saveGlobals() {
  return {
    stream: Object.getOwnPropertyDescriptor(globalThis, 'ReadableStream'),
    reader: Object.getOwnPropertyDescriptor(globalThis, 'ReadableStreamDefaultReader'),
    observer: Object.getOwnPropertyDescriptor(globalThis, key),
  };
}

function restoreGlobals(saved) {
  restoreStreamCancelObserver();
  for (const [property, descriptor] of [
    ['ReadableStream', saved.stream], ['ReadableStreamDefaultReader', saved.reader], [key, saved.observer],
  ]) {
    if (descriptor) Object.defineProperty(globalThis, property, descriptor);
    else delete globalThis[property];
  }
}

function installMocks() {
  class MockStream {
    cancel(...args) { this.calls.push({receiver: this, args}); return this.result; }
  }
  class MockReader {
    cancel(...args) { this.calls.push({receiver: this, args}); if (this.error) throw this.error; return this.result; }
  }
  Object.defineProperty(globalThis, 'ReadableStream', {value: MockStream, configurable: true, writable: true});
  Object.defineProperty(globalThis, 'ReadableStreamDefaultReader', {value: MockReader, configurable: true, writable: true});
  return {MockStream, MockReader};
}

test('delegates exact receiver, arguments, return value, and throw while recording only bounded metadata', () => {
  const saved = saveGlobals();
  try {
    const {MockStream, MockReader} = installMocks();
    const streamDescriptor = Object.getOwnPropertyDescriptor(MockStream.prototype, 'cancel');
    const readerDescriptor = Object.getOwnPropertyDescriptor(MockReader.prototype, 'cancel');
    assert.deepEqual(installStreamCancelObserver({capacity: 2, stackLimit: 1}), {
      available: true, status: 'installed', version: 1, capacity: 2, stackLimit: 1,
    });

    const stream = new MockStream();
    stream.calls = []; stream.result = {exact: 'stream'};
    const first = {opaque: 'argument'}, second = Symbol('second');
    assert.strictEqual(stream.cancel(first, second), stream.result);
    assert.strictEqual(stream.calls[0].receiver, stream);
    assert.deepEqual(stream.calls[0].args, [first, second]);

    const reader = new MockReader();
    reader.calls = []; reader.error = new Error('exact throw');
    assert.throws(() => reader.cancel('not recorded'), error => error === reader.error);
    assert.strictEqual(reader.calls[0].receiver, reader);
    assert.deepEqual(reader.calls[0].args, ['not recorded']);
    stream.cancel('dropped');

    const snapshot = streamCancelObserverSnapshot();
    assert.equal(snapshot.number_calls, 3);
    assert.equal(snapshot.dropped_count, 1);
    assert.equal(snapshot.entries.length, 2);
    assert.deepEqual(snapshot.entries.map(entry => entry.method), [
      'ReadableStream.prototype.cancel', 'ReadableStreamDefaultReader.prototype.cancel',
    ]);
    assert.ok(snapshot.entries.every(entry => Number.isFinite(entry.timestamp_ms)));
    assert.ok(snapshot.entries.every(entry => entry.stack.value.length <= 1 && entry.stack.truncated));
    assert.match(snapshot.limitations, /changes function identity and adds overhead/);
    assert.equal(JSON.stringify(snapshot).includes('not recorded'), false, 'arguments are never captured');

    assert.deepEqual(restoreStreamCancelObserver(), {available: true, status: 'restored', version: 1});
    assert.strictEqual(Object.getOwnPropertyDescriptor(MockStream.prototype, 'cancel').value, streamDescriptor.value);
    assert.strictEqual(Object.getOwnPropertyDescriptor(MockReader.prototype, 'cancel').value, readerDescriptor.value);
    assert.equal(Object.hasOwn(globalThis, key), false);
  } finally {
    restoreGlobals(saved);
  }
});

test('refuses unsupported, already-installed, and conflicting versioned globals without mutation', () => {
  const saved = saveGlobals();
  try {
    Object.defineProperty(globalThis, 'ReadableStream', {value: class {}, configurable: true, writable: true});
    Object.defineProperty(globalThis, 'ReadableStreamDefaultReader', {value: class {}, configurable: true, writable: true});
    assert.deepEqual(installStreamCancelObserver(), {available: false, status: 'refused-unsupported', version: 1});

    installMocks();
    assert.equal(installStreamCancelObserver().status, 'installed');
    assert.deepEqual(installStreamCancelObserver(), {available: false, status: 'refused-already-installed', version: 1});
    assert.deepEqual(restoreStreamCancelObserver(), {available: true, status: 'restored', version: 1});

    Object.defineProperty(globalThis, key, {value: {other: true}, configurable: true});
    assert.deepEqual(installStreamCancelObserver(), {available: false, status: 'refused-global-conflict', version: 1});
  } finally {
    restoreGlobals(saved);
  }
});
