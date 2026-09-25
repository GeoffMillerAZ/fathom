// Node/fake-DOM behavior checks, not browser layout, fullscreen or visual proof.
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

const source = readFileSync(new URL('./pricing_compare.js', import.meta.url), 'utf8');

class Element {
  constructor() {
    this.dataset = {}; this.style = {}; this.attrs = new Map(); this.listeners = new Map();
    this.hidden = false; this.value = ''; this.textContent = ''; this.clientWidth = 720;
    this.scrollTop = 0; this.writes = [];
  }
  addEventListener(name, fn) {
    if (!this.listeners.has(name)) this.listeners.set(name, []);
    this.listeners.get(name).push(fn);
  }
  async fire(name, detail = {}) {
    const event = { button: 0, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; }, ...detail };
    for (const fn of this.listeners.get(name) || []) await fn(event);
    return event;
  }
  setAttribute(key, value) { this.attrs.set(key, String(value)); this.writes.push([key, String(value)]); }
  getAttribute(key) { return this.attrs.get(key) ?? null; }
  removeAttribute(key) { this.attrs.delete(key); }
  get contentDocument() { throw new Error('Frame document access forbidden'); }
  get contentWindow() { throw new Error('Frame window access forbidden'); }
  set innerHTML(_) { throw new Error('HTML injection forbidden'); }
  replaceChildren() { throw new Error('Frame replacement forbidden'); }
}

function fixture({ layout = 'flip', active = 'original', scale = 'fit', viewport = '1440',
  preview_theme = 'light', theme = 'dark', hiddenTheme = true, observer = true,
  fullscreen = false, readyState = 'complete' } = {}) {
  const root = new Element();
  root.dataset = { layout, active, scale };
  const fields = Object.fromEntries(Object.entries({ layout, active, scale, viewport, preview_theme,
    ...(hiddenTheme ? { theme } : {}) }).map(([key, value]) => {
    const element = new Element(); element.value = value; return [key, element];
  }));
  const form = new Element();
  form.elements = { namedItem: (key) => fields[key] || null };
  const panes = ['original', 'converted'].map((side) => {
    const pane = new Element(); pane.dataset.side = side;
    const container = new Element(), box = new Element(), frame = new Element();
    frame.setAttribute('src', '/comparisons/p02-pricing/document/' + side + '-' + preview_theme + '.html');
    frame.setAttribute('height', '1000'); frame.writes = [];
    pane.querySelector = (selector) => ({ '.comparison-window': container, '.comparison-framebox': box, iframe: frame })[selector];
    return { pane, container, box, frame };
  });
  const links = ['original', 'converted'].map((side) => {
    const element = new Element(); element.dataset.selectSide = side; return element;
  });
  const button = new Element(); button.hidden = true;
  const status = new Element(), label = new Element(), doc = new Element(), win = new Element();
  root.querySelectorAll = () => panes.map((p) => p.pane);
  const elements = { comparison: root, 'comparison-controls': form, 'comparison-fullscreen': button,
    'comparison-status': status, 'comparison-scale-label': label };
  doc.getElementById = (id) => elements[id] || null;
  doc.querySelectorAll = () => links;
  doc.readyState = readyState;
  doc.documentElement = new Element();
  doc.fullscreenEnabled = fullscreen;
  doc.fullscreenElement = null;
  let fullscreenRequests = 0, fullscreenExits = 0;
  doc.documentElement.requestFullscreen = async () => {
    fullscreenRequests++; doc.fullscreenElement = doc.documentElement; await doc.fire('fullscreenchange');
  };
  doc.exitFullscreen = async () => { fullscreenExits++; doc.fullscreenElement = null; await doc.fire('fullscreenchange'); };
  win.location = { href: 'http://127.0.0.1:8765/comparisons/p02-pricing?theme=' + theme + '#comparison' };
  const history = [];
  win.history = { replaceState: (...args) => { history.push(args[2]); } };
  const observers = [];
  if (observer) win.ResizeObserver = class {
    constructor(callback) { this.callback = callback; this.targets = []; observers.push(this); }
    observe(target) { this.targets.push(target); }
  };
  const sandbox = { document: doc, window: win, URL, URLSearchParams,
    fetch() { throw new Error('Network forbidden'); },
    localStorage: new Proxy({}, { get() { throw new Error('Storage forbidden'); } }) };
  vm.runInNewContext(source, sandbox);
  return { root, form, fields, panes, links, button, status, label, doc, win, history, observers,
    fullscreenCounts: () => [fullscreenRequests, fullscreenExits],
    async change(key, value) { fields[key].value = value; await form.fire('change'); } };
}

const sourceWrites = (p) => p.frame.writes.filter(([key]) => key === 'src');

test('safe in Node without a document; deferred and immediate initialization', async () => {
  const sandbox = { module: { exports: {} } };
  vm.runInNewContext(source, sandbox);
  assert.equal(typeof sandbox.module.exports.mount, 'function');
  const f = fixture({ readyState: 'loading' });
  assert.equal(f.form.listeners.size, 0);
  await f.doc.fire('DOMContentLoaded');
  assert.equal(f.root.dataset.layout, 'flip');
  assert.equal(f.panes[1].pane.hidden, true);
  assert.equal(f.panes[0].frame.style.transform, 'scale(0.5)');
  assert.equal(f.panes[0].box.style.width, '720px');
  assert.equal(f.panes[0].box.style.height, '500px');
  assert.equal(f.panes[0].frame.getAttribute('height'), '1000');
  assert.equal(f.observers[0].targets.length, 2);
  assert.equal(f.button.hidden, true);
});

test('flip/layout/scale preserve frames and independent scroll; fallback links retain state', async () => {
  const f = fixture();
  const frames = f.panes.map((p) => p.frame);
  f.panes[0].container.scrollTop = 91; f.panes[1].container.scrollTop = 203;
  const event = await f.links[1].fire('click');
  assert.equal(event.defaultPrevented, true);
  assert.equal(f.root.dataset.active, 'converted');
  assert.equal(f.fields.active.value, 'converted');
  assert.deepEqual(f.panes.map((p) => p.pane.hidden), [true, false]);
  assert.equal(f.links[1].getAttribute('aria-current'), 'true');
  assert.equal(f.links[0].getAttribute('aria-current'), null);
  for (const layout of ['side', 'stack', 'flip']) {
    await f.change('layout', layout);
    assert.deepEqual(f.panes.map((p) => p.pane.hidden), layout === 'flip' ? [true, false] : [false, false]);
  }
  await f.change('scale', 'actual');
  assert.equal(f.panes[0].frame.style.transform, 'scale(1)');
  assert.equal(f.panes[0].box.style.width, '1440px');
  assert.equal(f.panes[0].box.style.height, '1000px');
  assert.deepEqual(f.panes.map((p) => p.frame), frames);
  assert.deepEqual(f.panes.map((p) => p.container.scrollTop), [91, 203]);
  assert.deepEqual(f.panes.map(sourceWrites), [[], []]);
  const url = new URL(f.links[0].getAttribute('href'), f.win.location.href);
  assert.equal(url.searchParams.get('active'), 'original');
  assert.equal(url.searchParams.get('theme'), 'dark');
  assert.equal(url.searchParams.get('layout'), 'flip');
  assert.equal(url.searchParams.get('scale'), 'actual');
  assert.equal(url.searchParams.get('viewport'), '1440');
  assert.equal(url.searchParams.get('preview_theme'), 'light');
  assert.equal(url.hash, '#comparison');
  assert.equal(new URL(f.history.at(-1), f.win.location.href).searchParams.get('active'), 'converted');
  assert.equal((await f.links[0].fire('click', { ctrlKey: true })).defaultPrevented, false);
  assert.equal(f.root.dataset.active, 'converted');
});

test('fit uses minimum visible window, caps enlargement and reacts to resize', async () => {
  const f = fixture({ layout: 'side' });
  f.panes[0].container.clientWidth = 900; f.panes[1].container.clientWidth = 360;
  f.observers[0].callback();
  assert.equal(f.panes[0].frame.style.transform, 'scale(0.25)');
  assert.equal(f.panes[1].box.style.height, '250px');
  await f.change('layout', 'flip');
  assert.equal(f.panes[0].frame.style.transform, 'scale(0.625)');
  await f.change('viewport', '390');
  assert.equal(f.panes[0].frame.style.transform, 'scale(1)');
  assert.equal(f.panes[0].frame.getAttribute('width'), '390');
  assert.equal(f.panes[0].frame.style.width, '390px');
  assert.match(f.panes[1].frame.getAttribute('title'), /390 × 1000, light/);
  assert.deepEqual(f.panes.map(sourceWrites), [[], []]);
  const fallback = fixture({ observer: false });
  fallback.panes[0].container.clientWidth = 144;
  await fallback.win.fire('resize');
  assert.equal(fallback.panes[0].frame.style.transform, 'scale(0.1)');
  assert.match(fallback.label.textContent, /^10%/);
});

test('only a preview-theme change reloads exactly the two known documents', async () => {
  const f = fixture({ hiddenTheme: false });
  await f.change('preview_theme', 'dark');
  for (let i = 0; i < f.panes.length; i++) {
    const side = i ? 'converted' : 'original';
    assert.deepEqual(sourceWrites(f.panes[i]), [['src', '/comparisons/p02-pricing/document/' + side + '-dark.html']]);
  }
  await f.change('preview_theme', 'dark');
  assert.deepEqual(f.panes.map((p) => sourceWrites(p).length), [1, 1]);
  assert.equal(new URL(f.history.at(-1), f.win.location.href).searchParams.get('theme'), 'dark');
  assert.equal((await f.form.fire('submit')).defaultPrevented, true);
  assert.deepEqual(f.panes.map((p) => sourceWrites(p).length), [1, 1]);
});

test('invalid enum changes, submissions and side selections fail without partial updates', async () => {
  const f = fixture();
  for (const [key, value] of [['layout', 'grid'], ['scale', '2'], ['viewport', '0390'],
    ['preview_theme', 'https://example.com'], ['active', '<script>']]) {
    const previous = f.fields[key].value;
    await f.change(key, value);
    assert.equal(f.fields[key].value, previous);
    assert.match(f.status.textContent, /previous view is unchanged/);
  }
  f.fields.layout.value = 'SIDE';
  await f.form.fire('submit');
  assert.equal(f.root.dataset.layout, 'flip');
  f.links[1].dataset.selectSide = '../elsewhere';
  await f.links[1].fire('click');
  assert.equal(f.root.dataset.active, 'original');
  assert.equal(f.history.length, 0);
  assert.deepEqual(f.panes.map(sourceWrites), [[], []]);
  const invalid = fixture({ viewport: '999' });
  assert.equal(invalid.form.listeners.size, 0);
  const invalidTheme = fixture({ theme: 'system' });
  assert.equal(invalidTheme.form.listeners.size, 0);
});

test('fullscreen is progressive, updates labels and catches enter/exit rejection', async () => {
  const f = fixture({ fullscreen: true });
  assert.equal(f.button.hidden, false);
  assert.equal(f.button.textContent, 'Enter fullscreen');
  await f.button.fire('click');
  assert.equal(f.button.textContent, 'Exit fullscreen');
  assert.equal(f.button.getAttribute('aria-pressed'), 'true');
  f.panes[0].container.clientWidth = 1440;
  await f.doc.fire('fullscreenchange');
  assert.equal(f.panes[0].frame.style.transform, 'scale(1)');
  await f.button.fire('click');
  assert.deepEqual(f.fullscreenCounts(), [1, 1]);
  assert.equal(f.button.textContent, 'Enter fullscreen');
  f.doc.documentElement.requestFullscreen = () => { throw new Error('denied'); };
  await f.button.fire('click');
  assert.match(f.status.textContent, /Fullscreen is unavailable/);
  f.doc.fullscreenElement = f.doc.documentElement;
  f.doc.exitFullscreen = async () => { throw new Error('denied'); };
  await f.button.fire('click');
  assert.match(f.status.textContent, /Fullscreen is unavailable/);
  assert.deepEqual(f.panes.map(sourceWrites), [[], []]);
  const unsupported = fixture({ fullscreen: true, readyState: 'loading' });
  delete unsupported.doc.documentElement.requestFullscreen;
  await unsupported.doc.fire('DOMContentLoaded');
  assert.equal(unsupported.button.hidden, true);
});

test('history refusal retains native links and working local controls', async () => {
  const f = fixture();
  f.win.history.replaceState = () => { throw new Error('history unavailable'); };
  await f.change('layout', 'stack');
  assert.equal(f.root.dataset.layout, 'stack');
  assert.equal(new URL(f.links[1].getAttribute('href'), f.win.location.href).searchParams.get('layout'), 'stack');
  assert.match(f.status.textContent, /Both previews shown/);
});
