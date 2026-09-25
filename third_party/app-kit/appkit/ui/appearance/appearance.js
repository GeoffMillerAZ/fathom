// app-kit appearance: one page preference, one optional app-owned storage key.
// Classic script; no framework, network, build step or storage dependency.
(function (w, d) {
  'use strict';
  if (w.AppKitAppearance) return;

  var initialized = false;
  var storageKey = '';
  var preference = 'system';
  var persistence = 'session';
  var error = null;
  var media = null;
  try {
    if (typeof w.matchMedia === 'function') media = w.matchMedia('(prefers-color-scheme: dark)');
  } catch (_) { /* CSS still owns system appearance if media queries are unavailable. */ }

  function valid(value) {
    return value === 'light' || value === 'dark' || value === 'system';
  }

  function snapshot() {
    return {
      preference: preference,
      effective: preference === 'system' ? (media && media.matches ? 'dark' : 'light') : preference,
      storageKey: storageKey,
      persistence: persistence,
      error: error
    };
  }

  function controls(root) {
    var found = [];
    if (root.matches && root.matches('[data-appkit-appearance]')) found.push(root);
    return found.concat(Array.from(root.querySelectorAll('[data-appkit-appearance]')));
  }

  function message() {
    if (persistence === 'unsaved') return 'Applied to this page. Your appearance could not be saved.';
    if (persistence === 'unavailable') return 'Your saved appearance could not be loaded. System appearance is active.';
    if (persistence === 'invalid') return 'Your saved appearance is invalid. System appearance is active; retry to save it.';
    if (persistence === 'saving') return 'Saving appearance…';
    if (persistence === 'session') return 'Applies to this page only.';
    return 'Saved on this device.';
  }

  function sync(root) {
    controls(root || d).forEach(function (control) {
      var select = control.querySelector('[data-appkit-appearance-select]');
      var status = control.querySelector('[data-appkit-appearance-status]');
      var retry = control.querySelector('[data-appkit-appearance-retry]');
      var conflict = (control.getAttribute('data-appearance-storage-key') || '') !== storageKey;
      var canRetry = !conflict && (persistence === 'unsaved' || persistence === 'unavailable' || persistence === 'invalid');
      control.setAttribute('data-appearance-state', conflict ? 'configuration-error' : persistence);
      if (select) {
        select.disabled = conflict;
        if (!conflict) select.value = preference;
      }
      if (status) status.textContent = conflict
        ? 'This appearance control is configured differently from the rest of the page.'
        : message();
      if (retry) {
        // Hiding a successful retry must not strand keyboard focus on the body.
        if (!canRetry && d.activeElement === retry && select && !select.disabled) select.focus();
        retry.hidden = !canRetry;
        retry.textContent = persistence === 'unavailable' ? 'Retry loading' : 'Retry saving';
      }
    });
  }

  function applyRoot() {
    if (preference === 'system') d.documentElement.removeAttribute('data-theme');
    else d.documentElement.setAttribute('data-theme', preference);
  }

  function publish(source) {
    applyRoot();
    sync();
    var state = snapshot();
    w.dispatchEvent(new w.CustomEvent('appkit:appearance-change', { detail: Object.assign({ source: source }, state) }));
    return state;
  }

  function readValue(value) {
    error = null;
    persistence = 'saved';
    if (value === null || value === undefined) preference = 'system';
    else if (valid(value)) preference = value;
    else {
      preference = 'system';
      persistence = 'invalid';
      error = 'invalid-stored-value';
    }
  }

  function restore(source) {
    preference = 'system';
    error = null;
    persistence = 'session';
    if (storageKey) {
      try {
        readValue(w.localStorage.getItem(storageKey));
      } catch (_) {
        persistence = 'unavailable';
        error = 'storage-read';
      }
    }
    return publish(source);
  }

  function init(options) {
    options = options || {};
    var key = options.storageKey === undefined ? '' : options.storageKey;
    if (typeof key !== 'string') throw new TypeError('AppKitAppearance storageKey must be a string');
    if (initialized) {
      if (key !== storageKey) throw new Error('AppKitAppearance conflicting storage keys: a page may use only one key');
      sync();
      return snapshot();
    }
    initialized = true;
    storageKey = key;
    return restore('init');
  }

  function get() {
    if (!initialized) init({});
    return snapshot();
  }

  function persist(source) {
    // Apply first: denied/quota-limited storage cannot block the page preference.
    applyRoot();
    error = null;
    persistence = storageKey ? 'saving' : 'session';
    sync();
    if (storageKey) {
      try {
        if (preference === 'system') w.localStorage.removeItem(storageKey);
        else w.localStorage.setItem(storageKey, preference);
        persistence = 'saved';
      } catch (_) {
        persistence = 'unsaved';
        error = 'storage-write';
      }
    }
    return publish(source);
  }

  function set(value) {
    if (!valid(value)) throw new TypeError('AppKitAppearance preference must be light, dark or system');
    get();
    preference = value;
    return persist('set');
  }

  function toggle() {
    return set(get().effective === 'dark' ? 'light' : 'dark');
  }

  function retry() {
    get();
    return persistence === 'unavailable' ? restore('retry') : persist('retry');
  }

  function mount(root) {
    root = root || d;
    var found = controls(root);
    if (!initialized && found.length) init({ storageKey: found[0].getAttribute('data-appearance-storage-key') || '' });
    if (initialized) sync(root);
    return initialized ? snapshot() : null;
  }

  function matchingControl(target) {
    if (!target || !target.closest) return null;
    var control = target.closest('[data-appkit-appearance]');
    if (!control) return null;
    mount(control);
    return (control.getAttribute('data-appearance-storage-key') || '') === storageKey ? control : null;
  }

  d.addEventListener('change', function (event) {
    var target = event.target;
    if (target && target.matches && target.matches('[data-appkit-appearance-select]')) {
      var value = target.value;
      if (matchingControl(target)) set(value);
    }
  });
  d.addEventListener('click', function (event) {
    var button = event.target && event.target.closest && event.target.closest('[data-appkit-appearance-retry]');
    if (button && matchingControl(button)) {
      event.preventDefault();
      retry();
    }
  });
  w.addEventListener('storage', function (event) {
    if (!initialized || !storageKey || (event.key !== null && event.key !== storageKey)) return;
    try {
      if (event.storageArea && event.storageArea !== w.localStorage) return;
    } catch (_) { return; }
    readValue(event.key === null ? null : event.newValue);
    publish('storage');
  });
  if (media) {
    var systemChanged = function () {
      if (initialized && preference === 'system') publish('system');
    };
    if (media.addEventListener) media.addEventListener('change', systemChanged);
    else if (media.addListener) media.addListener(systemChanged);
  }

  w.AppKitAppearance = { init: init, get: get, set: set, toggle: toggle, retry: retry, mount: mount };
  if (d.readyState === 'loading') d.addEventListener('DOMContentLoaded', function () { mount(); }, { once: true });
  else mount();
})(window, document);
