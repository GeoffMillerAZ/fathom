// Internal-seam transport check against unchanged upstream bytes, using Node's
// streams/events plus a deliberately small DOM/FormData stub. This does not prove
// browser behavior, declarative attribute compilation, nonce CSP, or a public SDK.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {readFile} from 'node:fs/promises';
import test from 'node:test';

class ElementStub extends EventTarget {
  constructor(tag = 'DIV') {
    super();
    this.tagName = tag;
    this.id = '';
    this.dataset = {};
    this.attributes = [];
  }
  getAttribute() { return null; }
  hasAttribute() { return false; }
  closest() { return null; }
  querySelectorAll() { return []; }
  checkValidity() { return true; }
}
class InputStub extends ElementStub {}
class ButtonStub extends ElementStub {}
class ObserverStub {
  observe() {}
  disconnect() {}
}

test('pinned Datastar internal fetch/signal seam (not browser or CSP proof)', {timeout: 5000}, async (t) => {
  const bundle = await readFile(new URL('./assets/datastar/v1.0.3/datastar.js', import.meta.url));
  assert.equal(bundle.length, 33538);
  assert.equal(createHash('sha256').update(bundle).digest('hex'), '5d6b7794a50a83d82da962aec5e382f5ae83ac7afbc751f903f7a9c6bd433c65');
  const form = new ElementStub('FORM');
  form.id = 'transport-empty-form';
  const button = new ButtonStub('BUTTON');
  button.id = 'transport-connect';
  const doc = new EventTarget();
  Object.assign(doc, {
    baseURI: 'http://localhost:19873/shell/activity',
    hidden: false,
    documentElement: new ElementStub('HTML'),
    createElement: (tag) => new ElementStub(tag.toUpperCase()),
    querySelectorAll: () => [],
    querySelector: (selector) => {
      assert.equal(selector, '#transport-empty-form');
      return form;
    },
  });
  const NativeFormData = globalThis.FormData;
  class EmptyFormData extends NativeFormData {
    constructor(selectedForm) {
      assert.equal(selectedForm, form, 'only the empty observer form may be serialized');
      super();
    }
  }
  const replacements = {
    document: doc, window: globalThis, Element: ElementStub,
    HTMLElement: ElementStub, SVGElement: ElementStub,
    HTMLInputElement: InputStub, HTMLButtonElement: ButtonStub,
    MutationObserver: ObserverStub, FormData: EmptyFormData,
    fetch: undefined,
  };
  const prior = new Map(Object.keys(replacements).map((key) => [key, Object.getOwnPropertyDescriptor(globalThis, key)]));
  for (const [key, value] of Object.entries(replacements)) {
    Object.defineProperty(globalThis, key, {value, writable: true, configurable: true});
  }
  t.after(() => {
    for (const [key, descriptor] of prior) {
      if (descriptor) Object.defineProperty(globalThis, key, descriptor);
      else delete globalThis[key];
    }
  });
  // Import the exact file contents. No substitutions or reimplementation of the
  // client parser, action, cancellation machinery, or signal store are involved.
  const client = await import(`data:text/javascript;base64,${bundle.toString('base64')}`);
  await new Promise((resolve) => setTimeout(resolve, 0)); // Let its empty DOM scan finish.
  const invoke = (controller) => client.actions.get({
    el: button,
    error: (name) => new Error(name),
    cleanups: new Map(),
  }, '/shell/events', {
    contentType: 'form', selector: '#transport-empty-form',
    headers: {Accept: 'text/event-stream'}, openWhenHidden: true,
    retry: 'never', retryMaxCount: 0, requestCancellation: controller,
  });
  const events = [];
  const collect = (event) => {
    assert.equal(event.detail.el, button);
    events.push(event.detail);
  };
  doc.addEventListener('datastar-fetch', collect);
  t.after(() => doc.removeEventListener('datastar-fetch', collect));

  await t.test('empty form sends no query/body; actual watcher applies string uint64 and terminal deletion', async () => {
    const snapshot = {
      version: '18446744073709551615', display_name: 'Alex <literal> Morgan',
      time_zone: 'America/Phoenix', start_page: 'activity', digest: true,
      access: 'editor', availability: 'ready', activity: ['Saved a synthetic change.'],
    };
    const frame = (value) => `event: datastar-patch-signals\ndata: signals ${JSON.stringify({appkitShellProbe: value})}\n\n`;
    const payload = frame(snapshot) + ': keepalive\n\n' + frame(null);
    const observed = [];
    const observe = (event) => {
      if (event.detail.type !== 'datastar-patch-signals') return;
      const applied = client.getPath('appkitShellProbe');
      observed.push(applied === undefined ? undefined : JSON.parse(JSON.stringify(applied)));
    };
    doc.addEventListener('datastar-fetch', observe);
    let requests = 0;
    globalThis.fetch = async (url, init) => {
      requests++;
      assert.equal(url, 'http://localhost:19873/shell/events');
      assert.equal(init.method, 'GET');
      assert.equal(init.body, undefined);
      assert.equal(new Headers(init.headers).get('Accept'), 'text/event-stream');
      assert.equal(new Headers(init.headers).get('Datastar-Request'), 'true');
      const bytes = new TextEncoder().encode(payload);
      return new Response(new ReadableStream({start(controller) {
        controller.enqueue(bytes.slice(0, 37));
        controller.enqueue(bytes.slice(37));
        controller.close();
      }}), {headers: {'Content-Type': 'text/event-stream'}});
    };
    try {
      await invoke(new AbortController());
      assert.equal(requests, 1);
      assert.deepEqual(observed, [snapshot, undefined]);
      assert.equal(client.getPath('appkitShellProbe'), undefined);
      client.mergePatch({appkitShellProbe: snapshot});
      assert.equal(client.getPath('appkitShellProbe').version, snapshot.version);
      client.mergePatch({appkitShellProbe: null});
      assert.equal(client.getPath('appkitShellProbe'), undefined, 'adapter cleanup can clear a denied reconnect without a server patch');
      assert.deepEqual(events.map(({type}) => type), ['started', 'datastar-patch-signals', 'datastar-patch-signals', 'finished']);
    } finally {
      doc.removeEventListener('datastar-fetch', observe);
      events.length = 0;
    }
  });

  await t.test('network failure makes one attempt with retryMaxCount zero', async () => {
    let requests = 0;
    globalThis.fetch = async () => { requests++; throw new TypeError('synthetic network failure'); };
    await assert.rejects(invoke(new AbortController()), /FetchFailed/);
    assert.equal(requests, 1);
    assert.deepEqual(events.map(({type}) => type), ['started', 'retries-failed', 'finished']);
    events.length = 0;
  });

  await t.test('custom controller abort reaches fetch and completes without retry', async () => {
    const controller = new AbortController();
    let requests = 0;
    let transportSignal;
    globalThis.fetch = (_url, init) => {
      requests++;
      transportSignal = init.signal;
      return new Promise((_resolve, reject) => init.signal.addEventListener('abort', () => {
        reject(new DOMException('synthetic cancellation', 'AbortError'));
      }, {once: true}));
    };
    const running = invoke(controller);
    assert.ok(transportSignal);
    doc.hidden = true;
    doc.dispatchEvent(new Event('visibilitychange'));
    assert.equal(transportSignal.aborted, false, 'openWhenHidden true leaves visibility lifetime to the adapter');
    controller.abort();
    await running;
    assert.equal(transportSignal.aborted, true);
    assert.equal(requests, 1);
    assert.deepEqual(events.map(({type}) => type), ['started', 'finished']);
    doc.hidden = false;
    events.length = 0;
  });

  await t.test('HTTP denial reports status in argsRaw and does not retry', async () => {
    let requests = 0;
    globalThis.fetch = async () => { requests++; return new Response('signed out', {status: 401}); };
    await invoke(new AbortController());
    assert.equal(requests, 1);
    assert.deepEqual(events.map(({type}) => type), ['started', 'error', 'finished']);
    assert.deepEqual(events[1].argsRaw, {status: '401'});
    events.length = 0;
  });
});
