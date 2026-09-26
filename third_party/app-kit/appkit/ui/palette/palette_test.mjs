import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const script = readFileSync(new URL('./palette.js', import.meta.url), 'utf8');

// Deliberately small DOM, timer and network doubles. They pin the script's state
// machine (debounce, cancellation, stale responses, selection, focus return and
// refusals); a real browser must separately prove native input, parsing and layout.

class Target {
  listeners = new Map();
  addEventListener(type, fn) {
    const list = this.listeners.get(type) || [];
    list.push(fn);
    this.listeners.set(type, list);
  }
  fire(event) {
    for (const fn of this.listeners.get(event.type) || []) fn(event);
  }
}

function makeEvent(type, props = {}) {
  return {
    type, button: 0, metaKey: false, ctrlKey: false, shiftKey: false, altKey: false,
    defaultPrevented: false, preventDefault() { this.defaultPrevented = true; }, ...props,
  };
}

function matchesOne(el, selector) {
  const m = /^([a-z0-9]*)(?:\[([a-z-]+)\])?$/.exec(selector.trim());
  if (!m) throw new Error(`unsupported selector ${selector}`);
  if (m[1] && el.tagName !== m[1].toUpperCase()) return false;
  if (m[2] && !el.attrs.has(m[2])) return false;
  return true;
}

let env; // the current environment, for focus and connection

class Text {
  constructor(data, nodeType = 3) {
    this.nodeType = nodeType; // 3 text, 8 comment
    this.nodeValue = data;
    this.parent = null;
  }
  cloneNode() { return new Text(this.nodeValue, this.nodeType); }
}

class El extends Target {
  constructor(tag, attrs = {}, children = []) {
    super();
    this.nodeType = 1;
    this.tagName = tag.toUpperCase();
    this.attrs = new Map(Object.entries(attrs));
    this.children = [];
    this.parent = null;
    this.hidden = 'hidden' in attrs;
    this.value = '';
    this.text = '';
    this.clicks = 0;
    for (const c of children) this.appendChild(c);
  }
  getAttribute(k) { return this.attrs.has(k) ? this.attrs.get(k) : null; }
  hasAttribute(k) { return this.attrs.has(k); }
  setAttribute(k, v) { this.attrs.set(k, String(v)); }
  removeAttribute(k) { this.attrs.delete(k); }
  get id() { return this.getAttribute('id') || ''; }
  set id(v) { this.setAttribute('id', v); }
  get elements() { return this.children.filter((c) => c.nodeType === 1); }
  get childNodes() { return this.children; }
  get firstElementChild() { return this.elements[0] || null; }
  get firstChild() { return this.children[0] || null; }
  get textContent() { return this.text; }
  set textContent(v) { this.text = String(v); }
  appendChild(c) {
    if (c.parent) c.parent.removeChild(c);
    c.parent = this;
    this.children.push(c);
    return c;
  }
  removeChild(c) {
    this.children = this.children.filter((x) => x !== c);
    c.parent = null;
    return c;
  }
  cloneNode() {
    const copy = new El(this.tagName.toLowerCase(), Object.fromEntries(this.attrs), this.children.map((c) => c.cloneNode(true)));
    copy.hidden = this.hidden;
    copy.text = this.text;
    if (this.content) copy.content = this.content.cloneNode(true);
    return copy;
  }
  matches(selector) { return selector.split(',').some((s) => matchesOne(this, s)); }
  querySelectorAll(selector) {
    const out = [];
    const walk = (el) => {
      for (const c of el.elements) {
        if (c.matches(selector)) out.push(c);
        walk(c);
      }
    };
    walk(this);
    return out;
  }
  querySelector(selector) { return this.querySelectorAll(selector)[0] || null; }
  closest(selector) {
    for (let el = this; el; el = el.parent) if (el.matches && el.matches(selector)) return el;
    return null;
  }
  contains(other) {
    for (let el = other; el; el = el.parent) if (el === this) return true;
    return false;
  }
  get isConnected() {
    let el = this;
    while (el.parent) el = el.parent;
    return el === env.document.body;
  }
  focus() { env.document.activeElement = this; }
  select() {}
  scrollIntoView() { this.scrolled = true; }
  getBoundingClientRect() { return { left: 100, right: 700, top: 100, bottom: 500 }; }
  click() {
    this.clicks++;
    dispatch(this, makeEvent('click'));
  }
}

// dispatch bubbles an event from el through its ancestors to the document.
function dispatch(el, event) {
  event.target = el;
  for (let node = el; node; node = node.parent) node.fire(event);
  env.document.fire(event);
  return event;
}

// fragment builds the fixed markup shape RenderResults emits, from a JSON spec.
function fragment({ state = 'results', query = '', options = 0, announce = '' }) {
  const frag = new El('div', {
    'data-palette-fragment': '', 'data-palette-state': state, 'data-palette-query': query,
    'data-palette-count': String(options), 'data-palette-announce': announce,
  });
  if (options) {
    const ul = new El('ul');
    for (let i = 0; i < options; i++) {
      ul.appendChild(new El('li', {}, [new El('a', { href: `/r/${i}`, 'data-palette-option': '' })]));
    }
    const group = new El('section', { 'data-palette-group': '' }, [new El('h3'), ul]);
    frag.appendChild(new El('div', { 'data-palette-list': '' }, [group]));
  }
  return frag;
}

// build turns one response-parsing spec node into a DOM double.
function build(node) {
  if (typeof node === 'string') return new Text(node);
  if ('comment' in node) return new Text(node.comment, 8);
  if (node.tag) return new El(node.tag, node.attrs || {}, (node.children || []).map(build));
  return fragment(node);
}

// A full app page that renders the palette: head content, the shell and the
// dialog with its idle fragment, as a template parses a whole document.
const fullPage = (inner = { state: 'idle' }) => [
  { tag: 'meta' }, { tag: 'title' }, '\n',
  { tag: 'header' },
  { tag: 'main', children: [{ tag: 'h1' }] },
  { tag: 'dialog', children: [{ tag: 'div', attrs: { 'data-palette': 'cmd' }, children: [{ tag: 'div', attrs: { 'data-palette-results': '' }, children: [inner] }] }] },
];

function environment({ platform = 'Linux x86_64', shortcut = 'mod+k', url = '/search', action = '/search', debounce = '150', timeout = '8000' } = {}) {
  const timers = new Map();
  let now = 0;
  let nextTimer = 1;
  const window = new Target();
  window.navigator = { platform };
  window.location = { origin: 'http://app.test' };
  window.URL = URL;
  window.AbortController = AbortController;
  window.setTimeout = (fn, ms) => { const id = nextTimer++; timers.set(id, { fn, at: now + ms }); return id; };
  window.clearTimeout = (id) => { timers.delete(id); };
  const requests = [];
  window.fetch = (href, init) => new Promise((resolve, reject) => {
    const req = { href, init, resolve, reject, aborted: false };
    init.signal?.addEventListener('abort', () => {
      req.aborted = true;
      reject(Object.assign(new Error('aborted'), { name: 'AbortError' }));
    });
    requests.push(req);
  });

  const document = new Target();
  document.readyState = 'complete';
  document.baseURI = 'http://app.test/page';
  document.body = new El('body');
  document.activeElement = document.body;
  document.createElement = (tag) => {
    const el = new El(tag);
    if (tag === 'template') {
      Object.defineProperty(el, 'innerHTML', {
        set(text) {
          el.content = new El('#fragment');
          let spec;
          try {
            spec = JSON.parse(text);
          } catch {
            el.content.appendChild(new El('main')); // e.g. a login page, not a fragment
            return;
          }
          for (const node of Array.isArray(spec) ? spec : [spec]) el.content.appendChild(build(node));
        },
      });
    }
    return el;
  };
  document.querySelectorAll = (sel) => document.body.querySelectorAll(sel);

  // makeTrigger mirrors RenderTrigger: a link with a hidden, empty hint.
  const makeTrigger = (id = 'cmd') => new El('a', { href: '/search', 'data-palette-open': id }, [new El('kbd', { 'data-palette-hint': '', hidden: '' })]);
  // mount mirrors RenderDialog and appends it to the body (unwired until init).
  const mount = ({ id = 'cmd', shortcut: sc = shortcut } = {}) => {
    const input = new El('input', { 'data-palette-input': '', 'aria-controls': `${id}-results`, 'aria-expanded': 'false' });
    const form = new El('form', { action: '/search', 'data-palette-form': '' }, [input]);
    const busy = new El('span', { 'data-palette-busy': '', hidden: '' });
    const status = new El('p', { 'data-palette-status': '' });
    const results = new El('div', { id: `${id}-results`, 'data-palette-results': '' }, [fragment({ state: 'idle' })]);
    const errorTemplate = new El('template', { 'data-palette-error': '' });
    errorTemplate.content = new El('#fragment', {}, [fragment({ state: 'error', announce: 'Search failed. Offline.' })]);
    const page = new El('a', { href: '/search', 'data-palette-page': '' });
    const root = new El('div', {
      'data-palette': id, 'data-palette-action': action, 'data-palette-url': url, 'data-palette-param': 'q', 'data-palette-shortcut': sc,
      'data-palette-debounce': debounce, 'data-palette-timeout': timeout, 'data-palette-label': 'Search',
    }, [form, busy, status, results, errorTemplate]);
    const close = new El('button', { 'data-overlay-dismiss': '' });
    const panel = new El('div', { 'data-overlay-panel': '' }, [close, root, page]);
    const dialog = new El('dialog', { id }, [panel]);
    dialog.open = false;
    dialog.showModal = () => {
      if (!dialog.isConnected) throw new Error("InvalidStateError: Failed to execute 'showModal' on 'HTMLDialogElement': The element is not in a Document.");
      dialog.open = true;
    };
    dialog.close = () => {
      if (!dialog.open) return;
      dialog.open = false;
      dialog.fire(makeEvent('close'));
    };
    document.body.appendChild(dialog);
    return { input, form, busy, status, results, dialog, page, close, root };
  };
  const trigger = makeTrigger();
  const other = new El('button');
  document.body.appendChild(trigger);
  document.body.appendChild(other);
  const { input, form, busy, status, results, dialog, page, close, root } = mount();

  env = { document };
  const context = vm.createContext({ window, document });
  const load = () => vm.runInContext(script, context);
  load();

  const flush = async () => { for (let i = 0; i < 10; i++) await new Promise((r) => setImmediate(r)); };
  return {
    window, document, requests, trigger, other, input, form, busy, status, results, dialog, page, close, root, load, flush, mount, makeTrigger,
    api: window.AppKitPalette,
    advance(ms) {
      const until = now + ms;
      for (;;) {
        const due = [...timers.entries()].filter(([, t]) => t.at <= until).sort((a, b) => a[1].at - b[1].at)[0];
        if (!due) break;
        timers.delete(due[0]);
        now = due[1].at;
        due[1].fn();
      }
      now = until;
    },
    type(value) {
      input.value = value;
      input.fire(makeEvent('input'));
    },
    key(target, key, props = {}) {
      return dispatch(target, makeEvent('keydown', { key, ...props }));
    },
    inputKey(key, props = {}) {
      const e = makeEvent('keydown', { key, ...props, target: input });
      input.fire(e);
      return e;
    },
    state: () => results.firstElementChild.getAttribute('data-palette-state'),
    options: () => results.querySelectorAll('[data-palette-option]'),
  };
}

async function respond(req, spec, { status = 200 } = {}) {
  req.resolve({ ok: status >= 200 && status < 300, status, text: async () => (typeof spec === 'string' ? spec : JSON.stringify(spec)) });
}

test('trigger is upgraded and localized per platform', () => {
  const pc = environment();
  assert.equal(pc.trigger.getAttribute('aria-haspopup'), 'dialog');
  assert.equal(pc.trigger.getAttribute('aria-keyshortcuts'), 'Control+K');
  assert.equal(pc.trigger.querySelector('[data-palette-hint]').textContent, 'Ctrl+K');
  assert.equal(pc.trigger.querySelector('[data-palette-hint]').hidden, false, 'the script reveals the server-hidden hint');
  assert.equal(pc.dialog.getAttribute('aria-label'), 'Search');
  const mac = environment({ platform: 'MacIntel' });
  assert.equal(mac.trigger.getAttribute('aria-keyshortcuts'), 'Meta+K');
  assert.equal(mac.trigger.querySelector('[data-palette-hint]').textContent, '⌘K');
  const custom = environment({ platform: 'MacIntel', shortcut: 'mod+shift+p' });
  assert.equal(custom.trigger.querySelector('[data-palette-hint]').textContent, '⇧⌘P');
});

test('Mod+K opens from anywhere and focuses the input; platform decides the modifier', () => {
  const pc = environment();
  pc.other.focus();
  const e = pc.key(pc.other, 'k', { ctrlKey: true, code: 'KeyK' });
  assert.ok(pc.dialog.open && e.defaultPrevented);
  assert.equal(pc.document.activeElement, pc.input);
  const mac = environment({ platform: 'MacIntel' });
  mac.key(mac.other, 'k', { ctrlKey: true });
  assert.equal(mac.dialog.open, false, 'Control+K stays a text-editing key on Apple platforms');
  mac.key(mac.other, 'k', { metaKey: true });
  assert.equal(mac.dialog.open, true);
  const extra = environment();
  extra.key(extra.other, 'k', { ctrlKey: true, shiftKey: true });
  assert.equal(extra.dialog.open, false, 'extra modifiers do not match');
});

test('an unmodified shortcut is ignored while typing elsewhere; none disables it', () => {
  const slash = environment({ shortcut: '/' });
  const field = new El('input');
  slash.document.body.appendChild(field);
  slash.key(field, '/');
  assert.equal(slash.dialog.open, false);
  slash.key(slash.other, '/', { code: 'Slash' });
  assert.equal(slash.dialog.open, true);
  const none = environment({ shortcut: 'none' });
  none.key(none.other, 'k', { ctrlKey: true });
  assert.equal(none.dialog.open, false);
  assert.equal(none.trigger.getAttribute('aria-keyshortcuts'), null);
  assert.equal(none.trigger.querySelector('[data-palette-hint]').hidden, true);
});

test('the shortcut follows the typed letter, not the physical key, on other layouts', () => {
  const mac = environment({ platform: 'MacIntel' });
  const dvorak = mac.key(mac.other, 't', { metaKey: true, code: 'KeyK' });
  assert.equal(mac.dialog.open, false, "Dvorak's physical K types t: Command+T is not Command+K");
  assert.equal(dvorak.defaultPrevented, false);
  mac.key(mac.other, 'k', { metaKey: true, code: 'KeyV' });
  assert.equal(mac.dialog.open, true, 'the key that types k on the layout matches');
  const save = environment({ shortcut: 'mod+s' });
  save.key(save.other, 'o', { ctrlKey: true, code: 'KeyS' });
  assert.equal(save.dialog.open, false, 'a configured mod+s does not steal Control+O on Dvorak');
  const cyrillic = environment();
  cyrillic.key(cyrillic.other, 'л', { ctrlKey: true, code: 'KeyK' });
  assert.equal(cyrillic.dialog.open, true, 'a non-Latin letter falls back to the physical key');
  const option = environment({ platform: 'MacIntel', shortcut: 'mod+alt+k' });
  option.key(option.other, '˚', { metaKey: true, altKey: true, code: 'KeyK' });
  assert.equal(option.dialog.open, true, 'an Option-modified symbol falls back to the physical key');
  const digit = environment({ shortcut: 'mod+shift+1' });
  digit.key(digit.other, '!', { ctrlKey: true, shiftKey: true, code: 'Digit1' });
  assert.equal(digit.dialog.open, true, 'a shifted digit falls back to the physical key');
});

test('a primary click on the trigger opens; modified clicks keep the link', () => {
  const env1 = environment();
  const modified = dispatch(env1.trigger, makeEvent('click', { metaKey: true }));
  assert.equal(modified.defaultPrevented, false);
  assert.equal(env1.dialog.open, false);
  const primary = dispatch(env1.trigger, makeEvent('click'));
  assert.ok(primary.defaultPrevented && env1.dialog.open);
});

test('typing is debounced into one same-origin fragment request', () => {
  const e = environment();
  e.api.open('cmd', e.trigger);
  e.type('o');
  e.advance(100);
  e.type('or');
  e.advance(100);
  e.type('orb');
  assert.equal(e.requests.length, 0);
  assert.equal(e.busy.hidden, false, 'pending input is marked busy');
  assert.equal(e.results.getAttribute('aria-busy'), 'true');
  e.advance(150);
  assert.equal(e.requests.length, 1);
  const req = e.requests[0];
  assert.equal(req.href, 'http://app.test/search?q=orb');
  assert.equal(req.init.headers['X-AppKit-Palette'], '1');
  assert.equal(req.init.credentials, 'same-origin');
  assert.equal(e.page.getAttribute('href'), '/search?q=orb', 'the search-page link follows the query');
});

test('the search-page link keeps the action context and the current query', () => {
  const e = environment({ action: '/search?theme=dark', url: '/search/fragment?theme=dark' });
  e.api.open('cmd', e.trigger);
  e.type('a b&c');
  assert.equal(e.page.getAttribute('href'), '/search?theme=dark&q=a+b%26c');
  e.advance(150);
  assert.equal(e.requests[0].href, 'http://app.test/search/fragment?theme=dark&q=a+b%26c');
  e.type('');
  assert.equal(e.page.getAttribute('href'), '/search?theme=dark');
});

test('a newer query aborts the older request and a late answer is ignored', async () => {
  const e = environment();
  e.api.open('cmd', e.trigger);
  e.type('orb');
  e.advance(150);
  const first = e.requests[0];
  e.type('orbi');
  assert.equal(first.aborted, true, 'changing the query cancels the stale request immediately');
  e.advance(150);
  const second = e.requests[1];
  await respond(second, { query: 'orbi', options: 1, announce: '1 result for “orbi”' });
  await e.flush();
  assert.equal(e.results.firstElementChild.getAttribute('data-palette-query'), 'orbi');
  await respond(first, { query: 'orb', options: 3 });
  await e.flush();
  assert.equal(e.results.firstElementChild.getAttribute('data-palette-query'), 'orbi', 'late stale response must not replace newer results');
  assert.equal(e.busy.hidden, true);
  e.advance(30);
  assert.equal(e.status.textContent, '1 result for “orbi”');
});

test('arrow keys move aria-activedescendant, Enter follows, focus returns', async () => {
  const e = environment();
  e.other.focus();
  e.key(e.other, 'k', { ctrlKey: true });
  e.type('orb');
  e.advance(150);
  await respond(e.requests[0], { query: 'orb', options: 3 });
  await e.flush();
  const opts = e.options();
  assert.equal(e.input.getAttribute('aria-expanded'), 'true');
  assert.equal(e.input.getAttribute('aria-controls'), 'cmd-listbox');
  assert.equal(e.results.querySelector('[data-palette-list]').getAttribute('role'), 'listbox');
  assert.equal(opts[0].getAttribute('role'), 'option');
  assert.equal(opts[0].getAttribute('tabindex'), '-1');
  assert.equal(e.input.getAttribute('aria-activedescendant'), 'cmd-option-0');
  e.inputKey('ArrowDown');
  assert.equal(e.input.getAttribute('aria-activedescendant'), 'cmd-option-1');
  assert.equal(opts[1].getAttribute('aria-selected'), 'true');
  assert.equal(opts[0].getAttribute('aria-selected'), 'false');
  e.inputKey('ArrowUp');
  e.inputKey('ArrowUp');
  assert.equal(e.input.getAttribute('aria-activedescendant'), 'cmd-option-2', 'selection wraps');
  e.inputKey('ArrowDown');
  assert.equal(e.input.getAttribute('aria-activedescendant'), 'cmd-option-0', 'selection wraps forward');
  const enter = e.inputKey('Enter');
  assert.ok(enter.defaultPrevented);
  assert.equal(opts[0].clicks, 1, 'Enter follows the selected link');
  assert.equal(e.dialog.open, false);
  assert.equal(e.document.activeElement, e.other, 'focus returns to the invoker');
});

test('Enter while results are stale falls through to the native GET form', async () => {
  const e = environment();
  e.api.open('cmd', e.trigger);
  e.type('orb');
  e.advance(150);
  await respond(e.requests[0], { query: 'orb', options: 2 });
  await e.flush();
  e.type('orbit');
  const enter = e.inputKey('Enter');
  assert.equal(enter.defaultPrevented, false, 'the full search page answers exactly what was typed');
  assert.equal(e.options()[0].clicks, 0);
  const composing = e.inputKey('Enter', { isComposing: true });
  assert.equal(composing.defaultPrevented, false);
});

test('Escape closes, cancels work and returns focus', () => {
  const e = environment();
  e.other.focus();
  e.key(e.other, 'k', { ctrlKey: true });
  e.type('orb');
  e.advance(150);
  const req = e.requests[0];
  const esc = e.inputKey('Escape');
  assert.ok(esc.defaultPrevented);
  assert.equal(e.dialog.open, false);
  assert.equal(req.aborted, true);
  assert.equal(e.busy.hidden, true);
  assert.equal(e.document.activeElement, e.other);
});

test('dismiss button and backdrop clicks close without overlay.js', () => {
  const e = environment();
  e.api.open('cmd', e.trigger);
  e.close.click();
  assert.equal(e.dialog.open, false);
  e.api.open('cmd', e.trigger);
  dispatch(e.dialog, makeEvent('click', { clientX: 10, clientY: 10 }));
  assert.equal(e.dialog.open, false);
  e.api.open('cmd', e.trigger);
  dispatch(e.dialog, makeEvent('click', { clientX: 300, clientY: 300 }));
  assert.equal(e.dialog.open, true, 'a click inside the panel does not dismiss');
});

for (const [name, answer] of [
  ['non-2xx status', (req) => respond(req, { query: 'x', options: 1 }, { status: 503 })],
  ['full page instead of a fragment', (req) => respond(req, '<html>sign in</html>')],
  // A redirect to any page that renders the palette carries its idle fragment.
  ['full page carrying the palette idle fragment', (req) => respond(req, fullPage())],
  ['full page carrying a fragment for this query', (req) => respond(req, fullPage({ query: 'x', options: 1 }))],
  ['text beside the fragment', (req) => respond(req, ['Signed out. ', { comment: 'x' }, { query: 'x', options: 1 }])],
  ['two fragments', (req) => respond(req, [{ query: 'x', options: 1 }, { query: 'x', options: 1 }])],
  ['a fragment for another query', (req) => respond(req, { query: 'y', options: 1 })],
  ['network failure', (req) => req.reject(new TypeError('offline'))],
]) {
  test(`${name} shows the error state for that query`, async () => {
    const e = environment();
    e.api.open('cmd', e.trigger);
    e.type('x');
    e.advance(150);
    await answer(e.requests[0]);
    await e.flush();
    assert.equal(e.state(), 'error');
    assert.equal(e.results.firstElementChild.getAttribute('data-palette-query'), 'x');
    assert.equal(e.input.getAttribute('aria-expanded'), 'false');
    assert.equal(e.input.getAttribute('aria-activedescendant'), null);
    assert.equal(e.busy.hidden, true);
    e.advance(30);
    assert.equal(e.status.textContent, 'Search failed. Offline.');
    assert.equal(e.inputKey('Enter').defaultPrevented, false, 'Enter reaches the search page');
  });
}

test('a fragment surrounded by whitespace and comments is accepted', async () => {
  const e = environment();
  e.api.open('cmd', e.trigger);
  e.type('x');
  e.advance(150);
  await respond(e.requests[0], ['\n  ', { comment: ' results ' }, { query: 'x', options: 2, announce: '2 results for “x”' }, '\n']);
  await e.flush();
  assert.equal(e.state(), 'results');
  assert.equal(e.options().length, 2);
});

test('a request that outlives the timeout is aborted and reported once', async () => {
  const e = environment({ timeout: '1000' });
  e.api.open('cmd', e.trigger);
  e.type('slow');
  e.advance(150);
  e.advance(1000);
  assert.equal(e.requests[0].aborted, true);
  assert.equal(e.state(), 'error');
  await e.flush();
  assert.equal(e.state(), 'error');
});

test('cross-origin fragment URLs are refused without a request', () => {
  const e = environment({ url: 'https://elsewhere.test/search' });
  e.api.open('cmd', e.trigger);
  e.type('x');
  e.advance(150);
  assert.equal(e.requests.length, 0);
  assert.equal(e.state(), 'error');
});

test('clearing the input restores idle without a request', async () => {
  const e = environment();
  e.api.open('cmd', e.trigger);
  e.type('orb');
  e.advance(150);
  const req = e.requests[0];
  e.type('  ');
  assert.equal(req.aborted, true);
  assert.equal(e.state(), 'idle');
  e.advance(1000);
  assert.equal(e.requests.length, 1);
  assert.equal(e.page.getAttribute('href'), '/search');
});

test('a re-mounted palette answers the shortcut; the detached one is dropped', () => {
  const e = environment();
  e.document.body.removeChild(e.dialog); // e.g. an htmx or Datastar swap
  const fresh = e.mount();
  const early = e.key(e.other, 'k', { ctrlKey: true });
  assert.equal(early.defaultPrevented, false, 'a detached palette neither throws nor swallows the browser key');
  assert.equal(e.api.open('cmd', e.trigger), false, 'the detached palette cannot open');
  e.api.init(e.document);
  e.other.focus();
  const ev = e.key(e.other, 'k', { ctrlKey: true });
  assert.ok(fresh.dialog.open && ev.defaultPrevented, 'the re-mounted palette opens');
  assert.equal(e.document.activeElement, fresh.input);
  assert.equal(e.dialog.open, false);
  fresh.input.fire(makeEvent('keydown', { key: 'Escape', target: fresh.input }));
  assert.equal(e.document.activeElement, e.other, 'focus returns to the invoker after the remount');
  assert.equal(e.trigger.getAttribute('aria-haspopup'), 'dialog');
});

test('triggers inserted after the palette was wired are upgraded by init', () => {
  const e = environment({ platform: 'MacIntel' });
  const late = e.makeTrigger();
  const header = new El('header', {}, [late]);
  e.document.body.appendChild(header);
  e.api.init(header);
  const h = late.querySelector('[data-palette-hint]');
  assert.equal(late.getAttribute('aria-haspopup'), 'dialog');
  assert.equal(late.getAttribute('aria-keyshortcuts'), 'Meta+K');
  assert.equal(h.textContent, '⌘K');
  assert.equal(h.hidden, false);
  const orphan = e.makeTrigger('missing');
  e.document.body.appendChild(orphan);
  e.api.init(e.document);
  assert.equal(orphan.getAttribute('aria-keyshortcuts'), null, 'a trigger without a mounted palette promises no shortcut');
  assert.equal(orphan.getAttribute('aria-haspopup'), null);
  assert.equal(orphan.querySelector('[data-palette-hint]').hidden, true);
  e.document.body.removeChild(e.dialog);
  e.api.init(e.document);
  assert.equal(e.trigger.getAttribute('aria-keyshortcuts'), null, 'removing the palette withdraws the shortcut from its triggers');
  assert.equal(e.trigger.querySelector('[data-palette-hint]').hidden, true);
});

test('init accepts the palette root itself', () => {
  const e = environment();
  const second = e.mount({ id: 'second' });
  assert.equal(e.api.open('second', e.trigger), false);
  e.api.init(second.root);
  assert.equal(e.api.open('second', e.trigger), true);
  assert.equal(second.dialog.open, true);
});

test('loading twice keeps one global and one set of listeners', () => {
  const e = environment();
  const first = e.api;
  e.load();
  assert.equal(e.window.AppKitPalette, first);
  e.api.init(e.document);
  e.key(e.other, 'k', { ctrlKey: true });
  assert.equal(e.dialog.open, true);
});
