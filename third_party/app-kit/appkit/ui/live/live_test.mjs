import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import http from 'node:http';
import test from 'node:test';
import vm from 'node:vm';

const script = readFileSync(new URL('./live.js', import.meta.url), 'utf8');

// Deliberately small DOM, clock, storage and fetch-stream doubles. They test the
// state machine and event contracts; a real browser must separately prove
// native input, layout, focus rendering and network behavior.
// Objects created inside the script's realm compare by value only.
const plain = (value) => JSON.parse(JSON.stringify(value));
const flush = async () => {
  for (let i = 0; i < 8; i++) await new Promise((resolve) => setImmediate(resolve));
};

class Target {
  listeners = new Map();
  addEventListener(type, fn) {
    const list = this.listeners.get(type) || [];
    list.push(fn);
    this.listeners.set(type, list);
  }
  removeEventListener(type, fn) {
    this.listeners.set(type, (this.listeners.get(type) || []).filter((f) => f !== fn));
  }
  dispatchEvent(event) {
    event.target ||= this;
    event.preventDefault ||= () => { event.defaultPrevented = true; };
    for (const fn of this.listeners.get(event.type) || []) fn(event);
    if (event.bubbles && this.parent) this.parent.dispatchEvent(event);
    return !event.defaultPrevented;
  }
}

function matcher(selector) {
  const m = /^([a-z]+)?((?:\[[a-z-]+\])*)$/.exec(selector);
  assert.ok(m, `unsupported selector ${selector}`);
  const attrs = [...m[2].matchAll(/\[([a-z-]+)\]/g)].map((x) => x[1]);
  return (el) => el instanceof El && (!m[1] || el.tag === m[1]) && attrs.every((a) => el.hasAttribute(a));
}

class El extends Target {
  constructor(doc, tag, attrs = {}) {
    super();
    this.doc = doc;
    this.tag = tag;
    this.attrs = new Map(Object.entries(attrs));
    this.children = [];
    this.style = {};
    this.hidden = this.attrs.has('hidden');
    this.textContent = '';
    this.parent = null;
  }
  get id() { return this.attrs.get('id') || ''; }
  getAttribute(key) { return this.attrs.has(key) ? this.attrs.get(key) : null; }
  setAttribute(key, value) { this.attrs.set(key, String(value)); }
  hasAttribute(key) { return this.attrs.has(key); }
  removeAttribute(key) { this.attrs.delete(key); }
  appendChild(child) { child.parent = this; this.children.push(child); return child; }
  add(tag, attrs, text) { const el = this.appendChild(new El(this.doc, tag, attrs)); if (text) el.textContent = text; return el; }
  matches(selector) { return matcher(selector)(this); }
  closest(selector) {
    const match = matcher(selector);
    for (let el = this; el instanceof El; el = el.parent) if (match(el)) return el;
    return null;
  }
  querySelectorAll(selector) {
    const match = matcher(selector);
    const out = [];
    const walk = (el) => { for (const c of el.children) { if (match(c)) out.push(c); walk(c); } };
    walk(this);
    return out;
  }
  querySelector(selector) { return this.querySelectorAll(selector)[0] || null; }
  focus(options) { this.doc.activeElement = this; this.focusOptions = options; }
}

class Clock {
  now = 0;
  seq = 0;
  timers = new Map();
  setTimeout = (fn, ms) => { const id = ++this.seq; this.timers.set(id, { at: this.now + (ms || 0), fn }); return id; };
  clearTimeout = (id) => { this.timers.delete(id); };
  next() {
    let best = null;
    for (const [id, t] of this.timers) if (!best || t.at < best[1].at || (t.at === best[1].at && id < best[0])) best = [id, t];
    return best;
  }
  async advance(ms) {
    const end = this.now + ms;
    await flush();
    for (;;) {
      const n = this.next();
      if (!n || n[1].at > end) break;
      this.now = n[1].at;
      this.timers.delete(n[0]);
      n[1].fn();
      await flush();
    }
    this.now = end;
  }
  delays() { return [...this.timers.values()].map((t) => t.at - this.now).sort((a, b) => a - b); }
}

function abortError() { const e = new Error('aborted'); e.name = 'AbortError'; return e; }

// One fetch call and its controllable server stream.
class Call {
  constructor(url, init) {
    this.url = url;
    this.init = init;
    this.queue = [];
    this.waiting = null;
    this.aborted = false;
    this.promise = new Promise((resolve, reject) => { this.resolve = resolve; this.reject = reject; });
    init.signal?.addEventListener('abort', () => {
      this.aborted = true;
      this.reject(abortError());
      if (this.waiting) { this.waiting.reject(abortError()); this.waiting = null; }
    });
  }
  header(name) { return this.init.headers[name]; }
  respond({ status = 200, type = 'text/event-stream', body = true } = {}) {
    const call = this;
    this.resolve({
      status, ok: status >= 200 && status < 300,
      headers: { get: (k) => (k.toLowerCase() === 'content-type' ? type : null) },
      body: body ? { getReader: () => ({
        read() {
          if (call.aborted) return Promise.reject(abortError());
          if (call.queue.length) return Promise.resolve(call.queue.shift());
          return new Promise((resolve, reject) => { call.waiting = { resolve, reject }; });
        },
        cancel() {},
      }) } : null,
    });
    return flush();
  }
  deliver(item) {
    if (this.waiting) { const w = this.waiting; this.waiting = null; w.resolve(item); } else this.queue.push(item);
  }
  push(text) { this.deliver({ done: false, value: new TextEncoder().encode(text) }); return flush(); }
  end() { this.deliver({ done: true }); return flush(); }
  fail() { if (this.waiting) { const w = this.waiting; this.waiting = null; w.reject(new Error('reset')); } return flush(); }
}

function environment({ ready = 'complete', autostart = true, url = '/events', events = 'change', hash = '', stored = {}, visibility = 'visible', cursor = null, random = 1, realTimers = false } = {}) {
  const document = new Target();
  document.readyState = ready;
  document.visibilityState = visibility;
  document.activeElement = null;
  const root = new El(document, 'html');
  document.body = root.add('body');
  document.querySelectorAll = (s) => root.querySelectorAll(s);
  document.querySelector = (s) => root.querySelector(s);
  document.getElementById = (id) => root.querySelectorAll('[id]').find((el) => el.id === id) || null;
  document.createElement = (tag) => new El(document, tag);
  root.parent = document;

  const pageAttrs = { 'data-live-page': '' };
  if (autostart) pageAttrs['data-live-url'] = url;
  if (events) pageAttrs['data-live-events'] = events;
  if (cursor !== null) pageAttrs['data-live-last-event-id'] = cursor;
  const page = document.body.add('div', pageAttrs);
  const connection = page.add('p', { 'data-live-connection': '' }, 'Live updates are off. Reload for newer data.');
  // RenderPageStatus markup: the control keeps its box (invisible, not hidden)
  // and its label is one layer of a stack that reserves the wider label.
  const toggle = page.add('button', { 'data-live-toggle': '', class: 'invisible' });
  const toggleLabel = toggle.add('span', {}).add('span', { 'data-live-toggle-label': '' }, 'Stop live updates');
  const announcer = page.add('div', { 'data-live-announcer': '', role: 'status' });

  function region(id, sources, label) {
    const el = document.body.add('section', { id, tabindex: '-1', 'data-live-source': sources, 'data-live-label': label });
    const body = el.add('p', {}, `${label} body`);
    const status = el.add('p', { 'data-live-status': '' });
    const asof = status.add('span', { 'data-live-asof': '' }, 'As of 12:03');
    const updated = status.add('span', { 'data-live-updated': '' });
    const link = updated.add('a', { 'data-live-refresh': '', href: `#${id}` }, 'refresh this panel');
    return { el, body, asof, updated, link };
  }
  const plans = region('plans', 'plans:fathom verify:fathom', 'Plan status');
  const index = region('index', 'ingest:fathom', 'Index freshness');

  const clock = new Clock();
  const calls = [];
  const window = new Target();
  const location = {
    href: `http://127.0.0.1:8775/live${hash}`, origin: 'http://127.0.0.1:8775', pathname: '/live', search: '', hash,
    reloads: 0,
    reload() { this.reloads++; },
  };
  const history = {
    state: null,
    replaceState(state, _title, url) {
      const u = new URL(url, location.href);
      Object.assign(location, { href: u.href, pathname: u.pathname, search: u.search, hash: u.hash });
    },
  };
  const data = new Map(Object.entries(stored));
  const sessionStorage = {
    getItem: (k) => (data.has(k) ? data.get(k) : null),
    setItem: (k, v) => data.set(k, String(v)),
    removeItem: (k) => data.delete(k),
  };
  Object.assign(window, {
    location, history, sessionStorage,
    setTimeout: realTimers ? setTimeout : clock.setTimeout, clearTimeout: realTimers ? clearTimeout : clock.clearTimeout,
    fetch: (url, init) => { const call = new Call(url, init); calls.push(call); return call.promise; },
    AbortController, TextDecoder, URL,
    CustomEvent: class { constructor(type, options) { this.type = type; this.bubbles = !!options.bubbles; this.detail = options.detail; } },
  });
  const statuses = [];
  window.addEventListener('appkit:live-status', (e) => statuses.push(e.detail.status));
  const regionEvents = [];
  document.addEventListener('appkit:live-updated', (e) => regionEvents.push(e.detail));

  const context = vm.createContext({ window, document });
  context.__random = random;
  vm.runInContext('Math.random = function () { return __random; };', context);
  const load = () => vm.runInContext(script, context);
  load();
  return {
    window, document, clock, calls, page, connection, toggle, toggleLabel, announcer, plans, index, statuses, regionEvents, data, load, location,
    api: window.AppKitLive,
    // Shown means usable: not hidden and made visible by the script.
    shown: () => !toggle.hidden && toggle.style.visibility === 'visible',
    last: () => calls[calls.length - 1],
    state: () => JSON.parse(JSON.stringify(window.AppKitLive.state())),
    click(el, extra = {}) {
      const event = { type: 'click', target: el, button: 0, bubbles: true, ...extra };
      el.dispatchEvent(event);
      return event;
    },
    async setVisibility(value) {
      document.visibilityState = value;
      document.dispatchEvent({ type: 'visibilitychange' });
      await flush();
    },
  };
}

async function connected(opts) {
  const env = environment(opts);
  await flush();
  assert.equal(env.calls.length, 1, 'auto-start opens one stream');
  await env.last().respond();
  return env;
}

test('auto-starts from markup with an explicit event-stream request', async () => {
  const env = environment();
  await flush();
  const call = env.last();
  assert.equal(call.url, 'http://127.0.0.1:8775/events');
  assert.equal(call.header('Accept'), 'text/event-stream');
  assert.equal(call.header('Last-Event-ID'), undefined);
  assert.equal(call.init.credentials, 'same-origin');
  assert.equal(call.init.cache, 'no-store');
  assert.equal(env.connection.textContent, 'Connecting to live updates…');
  assert.equal(env.shown(), true);
  await call.respond();
  assert.equal(env.state().status, 'live');
  assert.equal(env.connection.textContent, 'Live updates on.');
  assert.equal(env.toggleLabel.textContent, 'Stop live updates');
  assert.equal(env.toggle.children.length, 1, 'the label stack survives relabelling');
  assert.equal(env.page.getAttribute('data-live-status'), 'live');
  assert.deepEqual(env.statuses, ['connecting', 'live']);
});

test('the first connection resumes from the server-rendered cursor', async () => {
  const env = environment({ cursor: '17' });
  await flush();
  assert.equal(env.last().header('Last-Event-ID'), '17');
  assert.equal(env.state().lastEventId, '17');
  await env.last().respond();
  await env.last().push('id: 18\nevent: change\ndata: {"source":"plans:fathom"}\n\n');
  assert.deepEqual(env.state().updated, ['plans'], 'an event published after rendering is replayed');
  await env.last().end();
  await env.clock.advance(1500);
  assert.equal(env.last().header('Last-Event-ID'), '18');
  assert.throws(() => env.api.start({ url: '/e', lastEventId: 'a\nb' }), { name: 'TypeError' });
});

test('does nothing without a configured page and refuses bad explicit configuration', async () => {
  const env = environment({ autostart: false });
  await flush();
  assert.equal(env.calls.length, 0);
  assert.equal(env.state().status, 'idle');
  assert.equal(env.shown(), false, 'no control appears for a stream that never started');
  assert.equal(env.connection.textContent, 'Live updates are off. Reload for newer data.');
  assert.throws(() => env.api.start({ url: '' }), { name: 'TypeError' });
  assert.throws(() => env.api.start({ url: '/e#x' }), { name: 'TypeError' });
  assert.throws(() => env.api.start({ url: '/e', events: ['has space'] }), { name: 'TypeError' });
  assert.throws(() => env.api.start({ url: '/e', maxAttempts: 0 }), { name: 'TypeError' });
  assert.throws(() => env.api.start({ url: '/e', baseDelay: 5000, maxDelay: 1000 }), { name: 'TypeError' });
  assert.throws(() => env.api.start({ url: '/e', sources: 'x' }), { name: 'TypeError' });
  assert.throws(() => env.api.start(), { name: 'TypeError' });
  assert.equal(env.calls.length, 0, 'invalid configuration opens no stream');
});

test('marks only matching regions, once, without touching their content', async () => {
  const env = await connected();
  await env.last().push('id: 7\nevent: change\ndata: {"sources":["plans:fathom"]}\n\n');
  const s = env.state();
  assert.deepEqual(s.updated, ['plans']);
  assert.equal(s.lastEventId, '7');
  assert.equal(env.plans.el.getAttribute('data-live-state'), 'updated');
  assert.equal(env.plans.asof.style.visibility, 'hidden');
  assert.equal(env.plans.updated.style.visibility, 'visible');
  assert.equal(env.plans.body.textContent, 'Plan status body', 'content is never re-rendered');
  assert.equal(env.index.el.getAttribute('data-live-state'), null);
  assert.equal(env.index.updated.style.visibility, undefined);
  assert.equal(env.connection.textContent, 'Live updates on.', 'marking never changes the connection line, so it cannot move content');
  assert.deepEqual(plain(env.regionEvents), [{ source: 'plans:fathom', id: 'plans', eventId: '7' }]);

  assert.equal(env.announcer.textContent, '', 'announcements are debounced');
  await env.clock.advance(1100);
  assert.equal(env.announcer.textContent, 'Plan status has newer data. Use its refresh link to see it.');

  // A repeat for an updated region and a second source of the same region stay quiet.
  await env.last().push('event: change\ndata: {"source":"verify:fathom"}\n\n');
  await env.clock.advance(2000);
  assert.equal(env.regionEvents.length, 1);
  assert.equal(env.announcer.textContent, 'Plan status has newer data. Use its refresh link to see it.');
  assert.equal(env.state().unmatched, 0);

  await env.last().push('event: change\ndata: {"source":"nobody"}\n\n');
  assert.equal(env.state().unmatched, 1);
});

test('coalesces several regions into one announcement and supports the wildcard', async () => {
  const env = await connected();
  await env.last().push('event: change\ndata: {"sources":["*"]}\n\n');
  await env.clock.advance(1100);
  assert.deepEqual(env.state().updated, ['plans', 'index']);
  assert.equal(env.announcer.textContent, '2 panels have newer data: Plan status, Index freshness.');
  assert.equal(env.connection.textContent, 'Live updates on.');
});

test('follows only configured event names and ignores malformed or oversized data', async () => {
  const env = await connected();
  const call = env.last();
  await call.push('data: {"source":"plans:fathom"}\n\n'); // unnamed "message" is not followed here
  await call.push('event: other\ndata: {"source":"plans:fathom"}\n\n');
  assert.deepEqual(env.state().updated, []);
  await call.push('event: change\ndata: not json\n\n');
  await call.push('event: change\ndata: {"nothing":1}\n\n');
  assert.equal(env.state().ignored, 2);
  env.api.start({ url: '/events', events: ['change'], maxEventBytes: 256 });
  await call.push(`event: change\ndata: {"source":"plans:fathom","pad":"${'x'.repeat(300)}"}\n\n`);
  assert.equal(env.state().dropped, 1);
  assert.deepEqual(env.state().updated, []);
  assert.equal(env.calls.length, 1, 'reconfiguring the same stream does not reconnect');
});

test('parses WHATWG line endings, comments, BOM, split chunks and data joins', async () => {
  const env = environment({ events: '' });
  await flush();
  await env.last().respond();
  const call = env.last();
  await call.push('﻿: keepalive\r');
  await call.push('\nid: a1\r\ndata: {"sources":\r\ndata: ["ingest');
  await call.push(':fathom"]}\r\r');
  assert.deepEqual(env.state().updated, ['index']);
  assert.equal(env.state().lastEventId, 'a1');
  await call.push('id: x\u0000y\ndata: {"source":"plans:fathom"}\n\n');
  assert.equal(env.state().lastEventId, 'a1', 'ids containing NULL are ignored');
});

test('reconnects with bounded exponential backoff and resumes with Last-Event-ID', async () => {
  const env = await connected();
  let call = env.last();
  await call.push('id: 41\nevent: change\ndata: {"source":"nobody"}\n\n');
  await call.end();
  assert.equal(env.state().status, 'reconnecting');
  assert.equal(env.connection.textContent, 'Reconnecting (attempt 1 of 8)…');
  assert.deepEqual(env.clock.delays(), [1500], 'the first retry jitters upward from the 1 s floor');
  await env.clock.advance(1500);
  call = env.last();
  assert.equal(env.calls.length, 2);
  assert.equal(call.header('Last-Event-ID'), '41');
  const seen = [];
  for (let attempt = 2; attempt <= 8; attempt++) {
    await call.respond({ status: 503, type: 'text/plain' });
    assert.equal(env.state().status, 'reconnecting');
    assert.equal(env.state().error, 'http-503');
    seen.push(env.clock.delays()[0]);
    await env.clock.advance(env.clock.delays()[0]);
    call = env.last();
    assert.equal(call.header('Last-Event-ID'), '41');
  }
  assert.deepEqual(seen, [2000, 4000, 8000, 16000, 30000, 30000, 30000], 'doubling delay capped at maxDelay');
  const retries = env.statuses.filter((s) => s === 'reconnecting').length;
  assert.ok(retries >= 8, `every attempt is reported to listeners (${retries})`);
  await call.respond({ status: 503, type: 'text/plain' });
  const s = env.state();
  assert.equal(s.status, 'unavailable');
  assert.equal(s.attempt, 8);
  assert.equal(env.calls.length, 9, 'one initial stream plus eight bounded attempts');
  assert.deepEqual(env.clock.delays().filter((d) => d > 100), [], 'no reconnect is scheduled after giving up');
  assert.equal(env.connection.textContent, 'Live updates unavailable. Reload to retry.');
  assert.equal(env.toggleLabel.textContent, 'Resume live updates');
  assert.equal(env.shown(), true, 'a refused stream can be resumed');
  await env.clock.advance(100);
  assert.equal(env.announcer.textContent, 'Live updates unavailable. Reload to retry.');

  env.click(env.toggle);
  await flush();
  assert.equal(env.calls.length, 10, 'manual resume reconnects');
  assert.equal(env.last().header('Last-Event-ID'), '41');
  assert.equal(env.state().attempt, 0);
});

test('honors a larger server retry as the floor of its jittered delay', async () => {
  const env = await connected();
  await env.last().push('retry: 5000\n: ok\n\n');
  await env.last().end();
  assert.deepEqual(env.clock.delays(), [7500], 'server retry 5 s spreads over 5–7.5 s');
  await env.clock.advance(7500);
  await env.last().respond({ status: 502, type: 'text/html' });
  assert.equal(env.state().attempt, 2);
  assert.deepEqual(env.clock.delays(), [10000]);
});

// A server that accepts and then closes (a crashing handler, a proxy with a
// short timeout, an auth layer that flushes a preamble) must not reset the
// budget: lines alone prove nothing, so the retries stay bounded.
test('a stream that only sends a preamble and closes still exhausts the retry budget', async () => {
  const env = await connected();
  for (let attempt = 1; attempt <= 8; attempt++) {
    await env.last().push('retry: 1000\n: connected\n\n');
    await env.last().end();
    assert.equal(env.state().status, 'reconnecting');
    assert.equal(env.state().attempt, attempt, 'a preamble earns no fresh budget');
    await env.clock.advance(env.clock.delays().filter((d) => d > 100)[0]);
    await env.last().respond();
  }
  await env.last().push(': connected\n\n');
  await env.last().end();
  assert.equal(env.state().status, 'unavailable');
  assert.equal(env.state().error, 'closed');
  assert.equal(env.calls.length, 9, 'one stream plus eight bounded attempts');
  await env.clock.advance(600000);
  assert.equal(env.calls.length, 9);
});

test('a connection earns a fresh budget by staying open or advancing the cursor', async () => {
  const env = await connected();
  await env.last().end();
  await env.clock.advance(env.clock.delays()[0]);
  await env.last().respond({ status: 503, type: 'text/plain' });
  await env.clock.advance(env.clock.delays()[0]);
  await env.last().respond();
  assert.equal(env.state().attempt, 2);
  await env.last().push(': keepalive\n');
  await env.clock.advance(9000);
  assert.equal(env.state().attempt, 2, 'lines and a short stay earn nothing');
  await env.clock.advance(1000);
  assert.equal(env.state().attempt, 0, 'ten seconds open is a working stream');
  await env.last().end();
  assert.equal(env.state().attempt, 1);

  await env.clock.advance(env.clock.delays()[0]);
  await env.last().respond();
  await env.last().push('id: 5\nevent: change\ndata: {"source":"nobody"}\n\n');
  assert.equal(env.state().attempt, 0, 'an event that advances the cursor proves the stream');
  await env.last().end();
  await env.clock.advance(env.clock.delays()[0]);
  await env.last().respond();
  await env.last().push('id: 5\nevent: change\ndata: {"source":"nobody"}\n\n');
  assert.equal(env.state().attempt, 1, 'replaying the same id is not progress');
  await env.last().push('event: change\ndata: {"source":"nobody"}\n\n');
  assert.equal(env.state().attempt, 1, 'an event without an id is not progress');
});

test('the first reconnect is jittered and never sooner than the floor', async () => {
  const firsts = [];
  for (const random of [0, 0.25, 0.5, 0.75, 0.999]) {
    const env = await connected({ random });
    await env.last().end();
    const [first] = env.clock.delays();
    assert.ok(first >= 1000 && first <= 1500, `first delay ${first} within 1–1.5 s`);
    firsts.push(first);
    await env.clock.advance(first);
    await env.last().respond({ status: 503, type: 'text/plain' });
    const [second] = env.clock.delays();
    assert.ok(second >= 1000 && second <= 2000, `second delay ${second} within 1–2 s`);
    await env.clock.advance(second);
    await env.last().respond({ status: 503, type: 'text/plain' });
    const [third] = env.clock.delays();
    assert.ok(third >= 2000 && third <= 4000, `third delay ${third} within 2–4 s`);
  }
  assert.equal(new Set(firsts).size, firsts.length, `clients spread their first reconnect: ${firsts}`);
});

test('refuses permanently on client errors and stops on 204', async () => {
  const env = environment();
  await flush();
  await env.last().respond({ status: 404, type: 'text/plain' });
  assert.equal(env.state().status, 'unavailable');
  assert.equal(env.state().error, 'refused-404');
  await env.clock.advance(120000);
  assert.equal(env.calls.length, 1, 'no retries for a refused stream');

  const stopped = environment();
  await flush();
  await stopped.last().respond({ status: 204, type: '' });
  assert.equal(stopped.state().status, 'stopped');
  assert.equal(stopped.state().error, 'server-closed');
  await stopped.clock.advance(120000);
  assert.equal(stopped.calls.length, 1);

  const wrongType = environment();
  await flush();
  await wrongType.last().respond({ status: 200, type: 'text/html' });
  assert.equal(wrongType.state().status, 'reconnecting');
  assert.equal(wrongType.state().error, 'http-200');
});

test('pauses after the page stays hidden and resumes with Last-Event-ID', async () => {
  const env = await connected();
  const first = env.last();
  await first.push('id: 9\nevent: change\ndata: {"source":"nobody"}\n\n');
  await env.setVisibility('hidden');
  await env.clock.advance(4000);
  assert.equal(env.state().status, 'live', 'brief hides keep the stream');
  await env.setVisibility('visible');
  await env.clock.advance(10000);
  assert.equal(env.state().status, 'live');
  assert.equal(first.aborted, false);

  await env.setVisibility('hidden');
  await env.clock.advance(5000);
  assert.equal(env.state().status, 'paused');
  assert.equal(first.aborted, true, 'paused streams are closed');
  assert.equal(env.connection.textContent, 'Paused while this page is hidden.');
  await env.clock.advance(600000);
  assert.equal(env.calls.length, 1, 'nothing reconnects while hidden');
  await env.setVisibility('visible');
  assert.equal(env.calls.length, 2);
  assert.equal(env.last().header('Last-Event-ID'), '9');
  await env.last().respond();
  await env.last().push('id: 10\nevent: change\ndata: {"source":"ingest:fathom"}\n\n');
  assert.deepEqual(env.state().updated, ['index'], 'replayed events mark regions after resume');
});

test('manual stop closes the stream and never reconnects until resumed', async () => {
  const env = await connected();
  const call = env.last();
  await call.push('id: 3\n: ok\n\n');
  env.toggle.focus();
  env.click(env.toggle);
  await flush();
  assert.equal(call.aborted, true);
  assert.equal(env.state().status, 'stopped');
  assert.equal(env.toggleLabel.textContent, 'Resume live updates');
  assert.equal(env.document.activeElement, env.toggle, 'one control keeps focus across states');
  assert.equal(env.connection.textContent, 'Live updates stopped. Reload for newer data.');
  await env.clock.advance(100);
  assert.equal(env.announcer.textContent, 'Live updates stopped.');
  await env.clock.advance(600000);
  await env.setVisibility('hidden');
  await env.setVisibility('visible');
  assert.equal(env.calls.length, 1, 'stopped streams stay stopped');

  env.api.start();
  await flush();
  assert.equal(env.calls.length, 2);
  assert.equal(env.last().header('Last-Event-ID'), '3');
  await env.clock.advance(100);
  assert.equal(env.announcer.textContent, 'Live updates resumed.');
});

test('treats a silent stream as lost after idleTimeout', async () => {
  const env = environment({ autostart: false });
  env.api.start({ url: '/events', idleTimeout: 5000 });
  await flush();
  await env.last().respond();
  await env.clock.advance(4000);
  await env.last().push(': keepalive\n');
  await env.clock.advance(4000);
  assert.equal(env.state().status, 'live', 'keepalives hold the stream');
  await env.clock.advance(1000);
  assert.equal(env.state().status, 'reconnecting');
  assert.equal(env.state().error, 'idle');
});

test('custom source mapping reads app-owned payloads', async () => {
  const env = environment({ autostart: false });
  env.api.start({ url: '/events', events: ['ingest'], sources: (m) => [m.type + ':' + JSON.parse(m.data).service] });
  await flush();
  await env.last().respond();
  await env.last().push('event: ingest\ndata: {"service":"fathom","commit":"abc"}\n\n');
  assert.deepEqual(env.state().updated, ['index']);
});

test('event data never becomes markup', async () => {
  const env = await connected();
  await env.last().push('event: change\ndata: {"sources":["<img src=x onerror=alert(1)>","plans:fathom"]}\n\n');
  await env.clock.advance(1100);
  assert.equal(env.announcer.textContent, 'Plan status has newer data. Use its refresh link to see it.');
  assert.ok(!JSON.stringify(env.document.body.querySelectorAll('[data-live-source]').map((e) => e.textContent)).includes('<img'));
});

test('refresh reloads the same document at the region and restores focus there', async () => {
  const env = await connected();
  const event = env.click(env.plans.link);
  assert.equal(event.defaultPrevented, true, 'fragment-only navigation would not re-render');
  assert.equal(env.location.hash, '#plans');
  assert.equal(env.location.reloads, 1);
  assert.equal(env.data.get('appkit-live-focus'), 'plans');

  const modified = env.click(env.index.link, { metaKey: true });
  assert.equal(modified.defaultPrevented, undefined, 'modified clicks keep native behavior');

  const other = environment();
  await flush();
  other.plans.link.setAttribute('href', '/elsewhere#plans');
  const away = other.click(other.plans.link);
  assert.equal(away.defaultPrevented, undefined, 'other destinations navigate normally');
  assert.equal(other.location.reloads, 0);

  const reloaded = environment({ hash: '#plans', stored: { 'appkit-live-focus': 'plans' } });
  assert.equal(reloaded.document.activeElement, reloaded.plans.el);
  assert.deepEqual(plain(reloaded.plans.el.focusOptions), { preventScroll: true });
  assert.equal(reloaded.data.has('appkit-live-focus'), false, 'focus return is one-shot');

  const unrelated = environment({ hash: '#index', stored: { 'appkit-live-focus': 'plans' } });
  assert.equal(unrelated.document.activeElement, null);
});

test('waits for DOM readiness and loading twice keeps one controller', async () => {
  const env = environment({ ready: 'loading' });
  await flush();
  assert.equal(env.calls.length, 0);
  env.document.dispatchEvent({ type: 'DOMContentLoaded' });
  await flush();
  assert.equal(env.calls.length, 1);
  const api = env.window.AppKitLive;
  env.load();
  assert.equal(env.window.AppKitLive, api);
  await env.last().respond();
  await env.last().push('event: change\ndata: {"source":"plans:fathom"}\n\n');
  assert.equal(env.regionEvents.length, 1, 'a second load adds no duplicate listeners');
});

test('reports unsupported environments instead of claiming live behavior', async () => {
  const env = environment({ autostart: false });
  env.window.fetch = undefined;
  env.api.start({ url: '/events' });
  await flush();
  assert.equal(env.state().status, 'unavailable');
  assert.equal(env.state().error, 'unsupported');
  assert.equal(env.connection.textContent, 'Live updates unavailable. Reload for newer data.', 'retrying cannot help here');
  assert.equal(env.shown(), false, 'no resume control that cannot work');
  env.click(env.toggle);
  env.api.start();
  await env.clock.advance(200);
  assert.equal(env.state().status, 'unavailable');
  assert.notEqual(env.announcer.textContent, 'Live updates resumed.', 'a resume that did not connect is not announced');
});

test('a configuration the browser refuses shows no dead control', async () => {
  const env = environment({ url: 'http://127.0.0.1:99999/events' });
  await flush();
  assert.equal(env.calls.length, 0);
  assert.equal(env.state().status, 'unavailable');
  assert.equal(env.state().error, 'configuration');
  assert.equal(env.connection.textContent, 'Live updates unavailable. Reload for newer data.');
  assert.equal(env.shown(), false, 'a control that cannot act is not offered');
  for (let i = 0; i < 3; i++) env.click(env.toggle);
  await env.clock.advance(1000);
  assert.equal(env.calls.length, 0);
  assert.equal(env.state().error, 'configuration');
});

// fetch header values are byte strings: a code point above U+00FF makes fetch
// reject before any request, and U+0080–U+00FF would go out as Latin-1 bytes.
test('sends a non-Latin-1 cursor as the UTF-8 bytes of the id', async () => {
  const env = environment({ cursor: 'plan-ω1' });
  await flush();
  const header = env.last().header('Last-Event-ID');
  assert.ok([...header].every((c) => c.charCodeAt(0) <= 0xff), 'every header character is one byte');
  assert.equal(Buffer.from(header, 'latin1').toString('utf8'), 'plan-ω1');
  assert.equal(env.state().lastEventId, 'plan-ω1', 'state keeps the id as text');
  await env.last().respond();
  await env.last().push('id: é\nevent: change\ndata: {"source":"nobody"}\n\n');
  await env.last().end();
  await env.clock.advance(env.clock.delays()[0]);
  assert.equal(env.last().header('Last-Event-ID'), '\xc3\xa9', 'é is sent as UTF-8, not the Latin-1 byte 0xE9');
  assert.throws(() => env.api.start({ url: '/e', lastEventId: 'ω'.repeat(129) }), { name: 'TypeError' }, 'the bound is 256 UTF-8 bytes');
  assert.throws(() => env.api.start({ url: '/e', lastEventId: 'a\ud800' }), { name: 'TypeError' }, 'a lone surrogate has no UTF-8 form');
  assert.throws(() => env.api.start({ url: '/e', lastEventId: ' 42' }), { name: 'TypeError' }, 'fetch would strip the padding');
});

async function withServer(handler, fn) {
  const hits = [];
  const server = http.createServer((req, res) => { hits.push(req); handler(req, res, hits.length); });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  try {
    return await fn(`http://127.0.0.1:${server.address().port}/events`, hits);
  } finally {
    server.closeAllConnections();
    await new Promise((resolve) => server.close(resolve));
  }
}

async function until(predicate, ms) {
  const end = Date.now() + ms;
  while (!predicate() && Date.now() < end) await new Promise((resolve) => setTimeout(resolve, 10));
}

test('real fetch: a UTF-8 cursor reaches the server and the stream resumes from the next id', async () => {
  await withServer((req, res, n) => {
    res.writeHead(200, { 'Content-Type': 'text/event-stream' });
    res.end(n === 1 ? 'id: plan-ω2\nevent: change\ndata: {"source":"plans:fathom"}\n\n' : ': connected\n\n');
  }, async (url, hits) => {
    const env = environment({ autostart: false, realTimers: true });
    env.window.fetch = fetch;
    env.api.start({ url, events: ['change'], lastEventId: 'plan-ω1', baseDelay: 100, maxDelay: 200, maxAttempts: 2 });
    await until(() => hits.length >= 2, 3000);
    const ids = hits.map((req) => Buffer.from(req.headers['last-event-id'], 'latin1').toString('utf8'));
    assert.deepEqual(ids, ['plan-ω1', 'plan-ω2']);
    assert.deepEqual(env.state().updated, ['plans']);
    env.api.stop();
  });
});

test('real fetch: a server that accepts and closes is retried a bounded number of times', async () => {
  await withServer((req, res) => {
    res.writeHead(200, { 'Content-Type': 'text/event-stream' });
    res.end('retry: 100\n: connected\n\n');
  }, async (url, hits) => {
    const env = environment({ autostart: false, realTimers: true });
    env.window.fetch = fetch;
    env.api.start({ url, events: ['change'], baseDelay: 100, maxDelay: 30000, maxAttempts: 3 });
    await until(() => env.state().status === 'unavailable', 5000);
    assert.equal(env.state().status, 'unavailable');
    assert.equal(hits.length, 4, 'one stream plus three attempts');
    await new Promise((resolve) => setTimeout(resolve, 400));
    assert.equal(hits.length, 4, 'nothing reconnects after giving up');
  });
});

test('an explicit start adopts the rendered cursor of its page status', async () => {
  const env = environment({ ready: 'loading', cursor: '17' });
  env.api.start({ url: '/events', events: ['change'], sources: (m) => [JSON.parse(m.data).key] });
  await flush();
  assert.equal(env.calls.length, 0, 'regions and the cursor are not parsed yet; connect when they are');
  env.document.dispatchEvent({ type: 'DOMContentLoaded' });
  await flush();
  assert.equal(env.calls.length, 1, 'the explicit start wins over auto-start and connects once');
  assert.equal(env.last().header('Last-Event-ID'), '17');
  assert.equal(env.state().lastEventId, '17');
  await env.last().respond();
  await env.last().push('id: 18\nevent: change\ndata: {"key":"plans:fathom"}\n\n');
  assert.deepEqual(env.state().updated, ['plans'], 'custom mapping still applies');

  const ready = environment({ ready: 'complete', autostart: false });
  ready.page.setAttribute('data-live-url', '/events');
  ready.page.setAttribute('data-live-last-event-id', '30');
  ready.api.start({ url: '/events' });
  await flush();
  assert.equal(ready.last().header('Last-Event-ID'), '30', 'a start after parsing adopts it at once');

  const explicit = environment({ ready: 'loading', cursor: '17' });
  explicit.api.start({ url: '/events', lastEventId: '5' });
  explicit.document.dispatchEvent({ type: 'DOMContentLoaded' });
  await flush();
  assert.equal(explicit.last().header('Last-Event-ID'), '5', 'an explicit lastEventId wins');

  const other = environment({ ready: 'loading', cursor: '17' });
  other.api.start({ url: '/other' });
  other.document.dispatchEvent({ type: 'DOMContentLoaded' });
  await flush();
  assert.equal(other.last().header('Last-Event-ID'), undefined, 'a cursor belongs to its own stream URL');

  const stopped = environment({ ready: 'loading', cursor: '17' });
  stopped.api.start({ url: '/events' });
  stopped.api.stop();
  stopped.document.dispatchEvent({ type: 'DOMContentLoaded' });
  await flush();
  assert.equal(stopped.calls.length, 0, 'stop before the document is ready cancels the pending start');
});

test('switching to another stream does not send the previous stream cursor', async () => {
  const env = await connected({ cursor: '17' });
  await env.last().push('id: 18\nevent: change\ndata: {"source":"nobody"}\n\n');
  env.api.start({ url: '/elsewhere' });
  await flush();
  assert.equal(env.last().url, 'http://127.0.0.1:8775/elsewhere');
  assert.equal(env.last().header('Last-Event-ID'), undefined);
});
