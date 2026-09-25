import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const script = readFileSync(new URL('./appearance.js', import.meta.url), 'utf8');

// Deliberately small DOM/storage doubles. These test state transitions and
// event contracts; a real browser must separately prove native input and layout.
class Target {
  listeners = new Map();
  addEventListener(type, fn) {
    const list = this.listeners.get(type) || [];
    list.push(fn);
    this.listeners.set(type, list);
  }
  dispatchEvent(event) {
    event.preventDefault ||= () => { event.defaultPrevented = true; };
    for (const fn of this.listeners.get(event.type) || []) fn(event);
    return !event.defaultPrevented;
  }
}

function environment({ dark = false, stored = {}, keys = [], ready = 'loading', storageThrows = false } = {}) {
  const document = new Target();
  document.readyState = ready;
  document.activeElement = null;
  const attributes = new Map();
  document.documentElement = {
    setAttribute: (key, value) => attributes.set(key, value),
    removeAttribute: (key) => attributes.delete(key),
    getAttribute: (key) => attributes.get(key) ?? null,
  };
  const controls = [];
  document.querySelectorAll = () => controls;
  function control(key) {
    const attrs = new Map([['data-appearance-storage-key', key]]);
    const group = {
      getAttribute: (name) => attrs.get(name) ?? null,
      setAttribute: (name, value) => attrs.set(name, value),
      matches: (selector) => selector === '[data-appkit-appearance]',
      querySelectorAll: () => [],
      querySelector: (selector) => ({
        '[data-appkit-appearance-select]': group.select,
        '[data-appkit-appearance-status]': group.status,
        '[data-appkit-appearance-retry]': group.retry,
      })[selector],
    };
    group.select = {
      value: 'system', disabled: true,
      matches: (selector) => selector === '[data-appkit-appearance-select]',
      closest: (selector) => selector === '[data-appkit-appearance]' ? group : null,
      focus: () => { document.activeElement = group.select; },
    };
    group.status = { textContent: 'Appearance controls require JavaScript.' };
    group.retry = {
      hidden: true, textContent: 'Retry saving',
      closest: (selector) => selector === '[data-appkit-appearance-retry]' ? group.retry : selector === '[data-appkit-appearance]' ? group : null,
      focus: () => { document.activeElement = group.retry; },
    };
    controls.push(group);
    return group;
  }
  keys.forEach(control);
  const data = new Map(Object.entries(stored));
  const storage = {
    failRead: false, failWrite: false, reads: 0, writes: 0,
    getItem(key) {
      this.reads++;
      if (this.failRead) throw new Error('denied');
      return data.get(key) ?? null;
    },
    setItem(key, value) {
      this.writes++;
      if (this.failWrite) throw new Error('quota');
      data.set(key, value);
    },
    removeItem(key) {
      this.writes++;
      if (this.failWrite) throw new Error('denied');
      data.delete(key);
    },
  };
  const media = new Target();
  media.matches = dark;
  const window = new Target();
  window.storageThrows = storageThrows;
  window.storageAccesses = 0;
  Object.defineProperty(window, 'localStorage', { get() {
    window.storageAccesses++;
    if (window.storageThrows) throw new Error('storage is blocked');
    return storage;
  } });
  window.matchMedia = () => media;
  window.CustomEvent = class { constructor(type, options) { this.type = type; this.detail = options.detail; } };
  const events = [];
  window.addEventListener('appkit:appearance-change', (event) => events.push(JSON.parse(JSON.stringify(event.detail))));
  const context = vm.createContext({ window, document });
  const load = () => vm.runInContext(script, context);
  load();
  return {
    window, document, media, storage, data, controls, control, events, load,
    api: window.AppKitAppearance,
    theme: () => attributes.get('data-theme'),
    state: () => JSON.parse(JSON.stringify(window.AppKitAppearance.get())),
    ready: () => document.dispatchEvent({ type: 'DOMContentLoaded' }),
    choose(group, value) {
      group.select.value = value;
      group.select.focus();
      document.dispatchEvent({ type: 'change', target: group.select });
    },
    clickRetry(group) {
      group.retry.focus();
      const event = { type: 'click', target: group.retry };
      document.dispatchEvent(event);
      return event;
    },
    os(value) { media.matches = value; media.dispatchEvent({ type: 'change' }); },
    external(key, value, area = storage) {
      window.dispatchEvent({ type: 'storage', key, newValue: value, storageArea: area });
    },
  };
}

test('existing app keys retain plain preferences before CSS and mount later', () => {
  for (const key of ['ca-theme', 'gg-theme']) {
    const e = environment({ stored: { [key]: 'dark' }, keys: [key] });
    assert.equal(e.api.init({ storageKey: key }).preference, 'dark');
    assert.equal(e.theme(), 'dark');
    assert.equal(e.data.get(key), 'dark');
    assert.equal(e.storage.writes, 0, 'startup must not rewrite existing preferences');
    e.ready();
    assert.equal(e.controls[0].select.value, 'dark');
    assert.equal(e.controls[0].select.disabled, false);
    assert.equal(e.events[0].source, 'init');
  }
});

test('mount replaces the script fallback with actual persistence state', () => {
  const saved = environment({ stored: { 'ca-theme': 'dark' }, keys: ['ca-theme'] });
  assert.equal(saved.controls[0].status.textContent, 'Appearance controls require JavaScript.');
  saved.ready();
  assert.equal(saved.controls[0].status.textContent, 'Saved on this device.');

  const session = environment({ keys: [''] });
  assert.equal(session.controls[0].status.textContent, 'Appearance controls require JavaScript.');
  session.ready();
  assert.equal(session.controls[0].status.textContent, 'Applies to this page only.');

  const unavailable = environment({ keys: ['gg-theme'], storageThrows: true });
  assert.equal(unavailable.controls[0].status.textContent, 'Appearance controls require JavaScript.');
  unavailable.ready();
  assert.equal(unavailable.controls[0].status.textContent,
    'Your saved appearance could not be loaded. System appearance is active.');
});

test('empty-key state never accesses storage and system removes the explicit theme', () => {
  const e = environment({ dark: true, storageThrows: true, keys: [''] });
  e.ready();
  assert.equal(e.state().effective, 'dark');
  assert.equal(e.state().persistence, 'session');
  e.api.set('light');
  assert.equal(e.theme(), 'light');
  e.api.set('system');
  assert.equal(e.theme(), undefined);
  assert.equal(e.api.toggle().preference, 'light');
  assert.equal(e.window.storageAccesses, 0);
});

test('native select change preserves the chosen value and synchronizes same-key controls', () => {
  const e = environment({ keys: ['ca-theme', 'ca-theme'], ready: 'complete' });
  e.choose(e.controls[1], 'dark');
  assert.equal(e.theme(), 'dark');
  assert.equal(e.data.get('ca-theme'), 'dark');
  assert.deepEqual(e.controls.map(c => c.select.value), ['dark', 'dark']);
  assert.equal(e.document.activeElement, e.controls[1].select);
  assert.deepEqual(e.events.at(-1), {
    source: 'set', preference: 'dark', effective: 'dark', storageKey: 'ca-theme', persistence: 'saved', error: null,
  });
});

test('failed save still applies, reports unsaved state and retries without losing focus', () => {
  const e = environment({ stored: { 'gg-theme': 'light' }, keys: ['gg-theme', 'gg-theme'] });
  e.ready();
  e.storage.failWrite = true;
  e.choose(e.controls[0], 'dark');
  assert.equal(e.theme(), 'dark');
  assert.equal(e.data.get('gg-theme'), 'light');
  assert.equal(e.state().persistence, 'unsaved');
  assert.equal(e.state().error, 'storage-write');
  for (const c of e.controls) {
    assert.equal(c.retry.hidden, false);
    assert.match(c.status.textContent, /could not be saved/);
    assert.equal(c.select.value, 'dark');
  }
  e.storage.failWrite = false;
  assert.equal(e.clickRetry(e.controls[0]).defaultPrevented, true);
  assert.equal(e.state().persistence, 'saved');
  assert.equal(e.data.get('gg-theme'), 'dark');
  assert.equal(e.document.activeElement, e.controls[0].select);
  assert.equal(e.controls[0].retry.hidden, true);
  assert.equal(e.events.at(-1).source, 'retry');
});

test('system choice clears legacy overrides; failed removal retains only an unsaved page choice', () => {
  const e = environment({ dark: true, stored: { 'ca-theme': 'light' }, keys: ['ca-theme'] });
  e.ready();
  e.storage.failWrite = true;
  e.api.set('system');
  assert.equal(e.theme(), undefined);
  assert.equal(e.state().effective, 'dark');
  assert.equal(e.data.get('ca-theme'), 'light');
  assert.equal(e.state().persistence, 'unsaved');
  e.storage.failWrite = false;
  e.api.retry();
  assert.equal(e.data.has('ca-theme'), false);
  assert.equal(e.state().persistence, 'saved');
});

test('blocked reads fall back visibly and retry restores the actual stored preference', () => {
  const e = environment({ dark: false, stored: { 'gg-theme': 'dark' }, keys: ['gg-theme'], storageThrows: true });
  e.ready();
  assert.equal(e.state().preference, 'system');
  assert.equal(e.state().persistence, 'unavailable');
  assert.equal(e.controls[0].retry.textContent, 'Retry loading');
  e.window.storageThrows = false;
  e.api.retry();
  assert.equal(e.theme(), 'dark');
  assert.equal(e.state().persistence, 'saved');
  assert.equal(e.storage.writes, 0);
});

test('invalid stored values never become theme attributes and can be replaced explicitly', () => {
  const e = environment({ stored: { 'ca-theme': 'sepia<script>' }, keys: ['ca-theme'] });
  e.ready();
  assert.equal(e.theme(), undefined);
  assert.equal(e.state().error, 'invalid-stored-value');
  assert.equal(e.data.get('ca-theme'), 'sepia<script>');
  assert.equal(e.controls[0].retry.hidden, false);
  e.api.retry();
  assert.equal(e.data.has('ca-theme'), false);
  assert.equal(e.state().persistence, 'saved');
});

test('device changes notify app-owned graphics only when the system preference is active', () => {
  const e = environment({ keys: ['gg-theme'] });
  e.ready();
  e.os(true);
  assert.equal(e.theme(), undefined);
  assert.equal(e.events.at(-1).source, 'system');
  assert.equal(e.events.at(-1).effective, 'dark');
  e.api.set('light');
  const count = e.events.length;
  e.os(false);
  assert.equal(e.events.length, count);
  assert.equal(e.theme(), 'light');
});

test('cross-tab updates filter storage and keys, handle clear, and replace local unsaved state', () => {
  const e = environment({ keys: ['gg-theme'], dark: true });
  e.ready();
  const count = e.events.length;
  e.external('unrelated', 'dark');
  e.external('gg-theme', 'dark', {});
  assert.equal(e.events.length, count);
  e.storage.failWrite = true;
  e.api.set('light');
  e.external('gg-theme', 'dark');
  assert.equal(e.theme(), 'dark');
  assert.equal(e.state().persistence, 'saved');
  assert.equal(e.controls[0].retry.hidden, true);
  assert.equal(e.events.at(-1).source, 'storage');
  e.external(null, null);
  assert.equal(e.theme(), undefined);
  assert.equal(e.state().effective, 'dark');
  e.external('gg-theme', 'invalid');
  assert.equal(e.state().persistence, 'invalid');
});

test('conflicting keys fail explicitly without changing the configured page or other storage', () => {
  const e = environment({ keys: ['ca-theme', 'gg-theme'], stored: { 'ca-theme': 'dark', 'gg-theme': 'light' } });
  e.ready();
  assert.throws(() => e.api.init({ storageKey: 'gg-theme' }), /conflicting storage keys/);
  assert.equal(e.controls[1].select.disabled, true);
  assert.match(e.controls[1].status.textContent, /configured differently/);
  assert.equal(e.controls[1].getAttribute('data-appearance-state'), 'configuration-error');
  e.choose(e.controls[1], 'light');
  assert.equal(e.theme(), 'dark');
  assert.equal(e.data.get('gg-theme'), 'light');
});

test('dynamic controls mount without duplicate listeners and returned state is a copy', () => {
  const e = environment({ keys: ['ca-theme'], ready: 'complete' });
  e.api.set('dark');
  const added = e.control('ca-theme');
  e.api.mount(added);
  assert.equal(added.select.value, 'dark');
  assert.equal(added.select.disabled, false);
  const events = e.events.length;
  e.load();
  e.choose(added, 'light');
  assert.equal(e.events.length, events + 1);
  assert.equal(e.document.listeners.get('change').length, 1);
  const state = e.api.get();
  state.preference = 'dark';
  assert.equal(e.state().preference, 'light');
});

test('invalid API choices fail before changing page state or persistence', () => {
  const e = environment();
  assert.throws(() => e.api.set('sepia'), /light, dark or system/);
  assert.throws(() => e.api.init({ storageKey: 7 }), /must be a string/);
  assert.equal(e.events.length, 0);
  assert.equal(e.window.storageAccesses, 0);
  e.api.init({ storageKey: 'ca-theme' });
  assert.equal(e.state().storageKey, 'ca-theme');
});
