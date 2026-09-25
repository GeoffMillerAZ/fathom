// Read-only, opt-in Activity observer. Datastar owns transport and signals;
// this adapter owns explicit lifetime, bounded rendering and recovery language.
// It deliberately never selects an edit form or changes its optimistic version.
const maxVersion = '18446744073709551615';
const fields = ['version', 'display_name', 'time_zone', 'start_page', 'digest', 'access', 'availability', 'activity'];
const record = (v) => v !== null && typeof v === 'object' && !Array.isArray(v);
const text = (v, limit) => typeof v === 'string' && [...v].length <= limit;

export function validShellSnapshot(v) {
  return record(v) && Object.keys(v).length === fields.length && fields.every((key) => Object.hasOwn(v, key)) &&
    typeof v.version === 'string' && /^[1-9][0-9]{0,19}$/.test(v.version) &&
    (v.version.length < 20 || v.version <= maxVersion) &&
    text(v.display_name, 80) && v.display_name.length > 0 &&
    ['America/Phoenix', 'UTC', 'Europe/London'].includes(v.time_zone) &&
    ['home', 'activity'].includes(v.start_page) && typeof v.digest === 'boolean' &&
    ['editor', 'viewer'].includes(v.access) && ['ready', 'loading', 'error'].includes(v.availability) &&
    Array.isArray(v.activity) && v.activity.length <= 10 && v.activity.every((entry) => text(entry, 256));
}

export function mountShellLive(doc, win, getSnapshot, clearSnapshot) {
  const region = doc.getElementById('shell-live');
  if (!region) return null;
  const start = doc.getElementById('shell-live-start');
  const stop = doc.getElementById('shell-live-stop');
  const status = doc.getElementById('shell-live-status');
  const content = doc.getElementById('shell-live-content');
  const summary = doc.getElementById('shell-live-summary');
  const activity = doc.getElementById('shell-live-activity');
  let pending = null;
  let disposed = false;
  let lastVersion = null;
  let cleared = false;
  const listeners = [];
  const on = (target, type, fn) => {
    target.addEventListener(type, fn);
    listeners.push(() => target.removeEventListener(type, fn));
  };
  const state = (name, message) => {
    region.dataset.state = name;
    status.textContent = message;
  };
  const controls = () => {
    start.disabled = disposed || !!pending || doc.hidden;
    stop.disabled = !pending || pending.controller.signal.aborted;
  };
  const clear = () => {
    cleared = true;
    lastVersion = null;
    content.hidden = true;
    summary.textContent = '';
    activity.replaceChildren();
    clearSnapshot();
  };
  const end = (name, message, redact = false) => {
    if (redact) clear();
    if (pending) pending.terminal = true;
    state(name, message);
    pending?.controller.abort();
    controls();
  };
  const render = (snapshot) => {
    // Versions remain decimal strings; uint64 exceeds JavaScript's safe integer.
    if (lastVersion && (snapshot.version.length < lastVersion.length ||
      (snapshot.version.length === lastVersion.length && snapshot.version < lastVersion))) {
      throw new Error('out-of-order snapshot');
    }
    lastVersion = snapshot.version;
    cleared = false;
    const entries = snapshot.activity.map((entry) => {
      const li = doc.createElement('li');
      li.className = 'py-4 text-sm';
      li.textContent = entry;
      return li;
    });
    activity.replaceChildren(...entries);
    summary.textContent = `Last observed version ${snapshot.version} · ${snapshot.display_name} · ${snapshot.access} · account ${snapshot.availability}`;
    content.hidden = false;
    state('watching', 'Watching for changes. This version is the last observed snapshot, not a freshness guarantee.');
  };
  on(doc, 'datastar-fetch', (event) => {
    const {type, el, argsRaw} = event.detail || {};
    if (disposed || el !== start) return;
    if (!pending || pending.controller.signal.aborted) {
      // A parser can finish an already-buffered chunk after abort. Its watcher
      // runs first, so keep a redacted namespace empty even for those late frames.
      if (cleared && type === 'datastar-patch-signals') clearSnapshot();
      return;
    }
    if (type === 'error' && (argsRaw?.status === '401' || argsRaw?.status === '403')) {
      end('revoked', 'Access ended. Observed data cleared. Sign in or reload before connecting again.', true);
    } else if (type === 'error' || type === 'retries-failed') {
      end('interrupted', 'Connection failed. Displayed data may be stale. Connect again or reload to check for changes.');
    } else if (type === 'datastar-patch-signals') {
      try {
        if (!record(argsRaw) || typeof argsRaw.signals !== 'string' || argsRaw.signals.length > 8192) throw new Error('invalid frame');
        const envelope = JSON.parse(argsRaw.signals);
        if (!record(envelope) || Object.keys(envelope).length !== 1 || !Object.hasOwn(envelope, 'appkitShellProbe')) throw new Error('invalid namespace');
        if (envelope.appkitShellProbe === null) {
          end('revoked', 'Access ended. Observed data cleared. Sign in or reload before connecting again.', true);
          return;
        }
        const snapshot = envelope.appkitShellProbe;
        if (!validShellSnapshot(snapshot)) throw new Error('invalid snapshot');
        // The pinned patch-signals watcher runs before this listener. Rendering
        // must reflect the actual signal store, not a parallel mocked transport.
        const applied = getSnapshot();
        if (!validShellSnapshot(applied) || fields.some((key) => JSON.stringify(applied[key]) !== JSON.stringify(snapshot[key]))) throw new Error('signal mismatch');
        render(applied);
      } catch {
        end('invalid', 'Live data could not be verified and was cleared. Reload for the ordinary page.', true);
      }
    }
  });
  const pause = () => {
    if (pending && !pending.terminal) end('paused', 'Watching paused. Displayed data may be stale. Connect again to request a new snapshot.');
    controls();
  };
  on(stop, 'click', pause);
  on(doc, 'visibilitychange', () => {
    if (doc.hidden) pause();
    controls(); // Showing the page never reconnects by itself.
  });
  on(win, 'pagehide', pause);
  on(win, 'pageshow', controls);
  controls();
  return {
    get controller() { return pending?.controller; },
    async connect(request) {
      if (disposed || pending || doc.hidden || typeof request !== 'function') return;
      const run = {controller: new win.AbortController(), terminal: false};
      pending = run;
      lastVersion = null; // Reconnect is a full snapshot; the session may be new.
      controls();
      state('connecting', 'Connecting. Displayed data may be stale until a new snapshot arrives.');
      try {
        await request();
        if (!run.terminal) state('closed', cleared ? 'Connection ended without a snapshot. Reload to recover.' : 'Connection ended. Displayed data may be stale. Connect again to request a new snapshot.');
      } catch {
        if (!run.terminal) state('interrupted', 'Connection failed. Displayed data may be stale. Connect again or reload to check for changes.');
      } finally {
        if (pending === run) {
          pending = null;
          controls();
        }
      }
    },
    dispose() {
      if (disposed) return;
      pause();
      disposed = true;
      listeners.splice(0).forEach((remove) => remove());
      controls();
    },
  };
}
