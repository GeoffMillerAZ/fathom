import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

const source = readFileSync(new URL('./shell_demo.js', import.meta.url), 'utf8');

function fixture({ baseline = { display_name: 'Alex Morgan' }, value = 'Alex Morgan', checkbox = false, malformed = false, retry = false, disabled = false, extraField } = {}) {
  let document;
  const makeField = options => ({...options, focus() { document.activeElement = this; }});
  const field = makeField({ name: checkbox ? 'digest' : 'display_name', type: checkbox ? 'checkbox' : 'text', value, checked: value, disabled });
  const fields = [field];
  if (extraField) fields.push(makeField(extraField));
  let statusText = '';
  const status = { writes: 0, get textContent() { return statusText; },
    set textContent(value) { statusText = value; this.writes++; } };
  const button = { textContent: 'Save changes', disabled: false };
  const resume = { hidden: true, addEventListener: (_, fn) => { resume.click = fn; }, focus() { document.activeElement = resume; } };
  const events = new Map(), windowEvents = new Map(), attrs = new Map();
  const form = {
    elements: fields, isConnected: true, valid: true,
    getAttribute: (name) => name === 'data-shell-retry' ? String(retry) : malformed ? 'broken JSON' : JSON.stringify(baseline),
    setAttribute: (key, value) => attrs.set(key, value), removeAttribute: (key) => attrs.delete(key),
    querySelector: (selector) => selector === '[data-shell-save]' ? button : selector === '[data-shell-resume]' ? resume : status,
    addEventListener: (name, fn) => events.set(name, fn), checkValidity: () => form.valid,
  };
  document = { readyState: 'complete', activeElement: null, querySelectorAll: () => [form] };
  const window = { addEventListener: (name, fn) => windowEvents.set(name, fn), stops: 0, stop() { this.stops++; } };
  const context = vm.createContext({ window, document });
  vm.runInContext(source, context);
  return { form, field, fields, status, button, resume, attrs, events, windowEvents, window, document, context };
}

test('draft changes stay local and beforeunload warns only for dirty connected forms', () => {
  const f = fixture();
  assert.equal(f.button.disabled, true);
  assert.equal(f.status.textContent, 'No unsaved changes');
  f.field.value = '<unsafe> revised name';
  f.events.get('input')();
  assert.equal(f.button.disabled, false);
  assert.equal(f.status.textContent, 'Unsaved changes');
  const event = { prevented: false, preventDefault() { this.prevented = true; } };
  f.windowEvents.get('beforeunload')(event);
  assert.equal(event.prevented, true);
  assert.equal(event.returnValue, '');
  f.form.isConnected = false;
  event.prevented = false;
  f.windowEvents.get('beforeunload')(event);
  assert.equal(event.prevented, false);
});

test('valid native submission shows progress; returning to the pending document exposes uncertainty', () => {
  const f = fixture({ value: 'Unsaved draft' });
  const event = { defaultPrevented: false, preventDefault() { throw Error('must not intercept native submission'); } };
  f.events.get('submit')(event);
  assert.equal(f.button.disabled, true);
  assert.equal(f.attrs.get('aria-busy'), 'true');
  assert.match(f.status.textContent, /^Saving changes/);
  f.windowEvents.get('beforeunload')({ preventDefault() { throw Error('own submission must not warn'); } });
  f.windowEvents.get('pageshow')();
  assert.equal(f.attrs.has('aria-busy'), false);
  assert.equal(f.button.disabled, false);
  assert.equal(f.button.textContent, 'Save changes');
  assert.match(f.status.textContent, /server may already have saved/);
  assert.equal(f.window.stops, 0, 'pageshow recovery does not stop a new navigation');
});

test('native-invalid or already cancelled submission does not enter pending state', () => {
  const f = fixture({ value: '' });
  f.form.valid = false;
  f.events.get('submit')({ defaultPrevented: false });
  assert.equal(f.attrs.has('aria-busy'), false);
  assert.equal(f.button.disabled, false);
  f.form.valid = true;
  f.events.get('submit')({ defaultPrevented: true });
  assert.equal(f.attrs.has('aria-busy'), false);
});

test('server-returned draft is compared with saved values, not treated as saved', () => {
  const f = fixture({ baseline: { display_name: 'Saved elsewhere' }, value: 'My retained draft' });
  assert.equal(f.status.textContent, 'Unsaved changes');
  f.field.value = 'Saved elsewhere';
  f.events.get('change')();
  assert.equal(f.status.textContent, 'No unsaved changes');
  assert.equal(f.button.disabled, true);
});

test('checkbox comparison uses boolean state; mounting twice preserves one controller', () => {
  const f = fixture({ baseline: { digest: true }, value: true, checkbox: true });
  assert.equal(f.button.disabled, true);
  f.field.checked = false;
  f.events.get('change')();
  assert.equal(f.button.disabled, false);
  const submit = f.events.get('submit');
  f.window.AppKitShellDemo.mount({ querySelectorAll: () => [f.form] });
  vm.runInContext(source, f.context);
  assert.equal(f.events.get('submit'), submit);
});

test('malformed enhancement metadata leaves the ordinary HTML form usable', () => {
  const f = fixture({ malformed: true });
  assert.equal(f.events.size, 0);
  assert.equal(f.button.disabled, false);
});

test('interrupted navigation has explicit recovery without automatic repost or a false server-cancel claim', () => {
  const f = fixture({ value: 'Retained draft' });
  f.events.get('submit')({ defaultPrevented: false });
  assert.equal(f.resume.hidden, false);
  f.resume.focus();
  f.resume.click();
  assert.equal(f.window.stops, 1);
  assert.equal(f.resume.hidden, true);
  assert.equal(f.document.activeElement, f.field);
  assert.equal(f.button.disabled, false);
  assert.equal(f.attrs.has('aria-busy'), false);
  assert.match(f.status.textContent, /server may already have saved/);
  let warned = false;
  f.windowEvents.get('beforeunload')({ preventDefault() { warned = true; } });
  assert.equal(warned, true);
});

test('editing during pending navigation resumes the draft and restores dirty warnings', () => {
  const f = fixture({ value: 'Submitted value' });
  f.events.get('submit')({ defaultPrevented: false });
  f.field.value = 'New unsent value';
  f.field.focus();
  f.events.get('input')();
  assert.equal(f.window.stops, 1);
  assert.equal(f.field.value, 'New unsent value');
  assert.equal(f.button.disabled, false);
  assert.equal(f.attrs.has('aria-busy'), false);
  assert.equal(f.document.activeElement, f.field);
});

test('a rejected submission can be retried even when canonical form values match saved values', () => {
  const f = fixture({ retry: true });
  assert.equal(f.button.disabled, false);
  assert.match(f.status.textContent, /retry the failed submission/);
  f.events.get('submit')({ defaultPrevented: false });
  assert.equal(f.attrs.get('aria-busy'), 'true');
  f.resume.focus();
  f.resume.click();
  assert.equal(f.document.activeElement, f.field);
});

test('explicit recovery skips disabled participating fields', () => {
  const f = fixture({
    baseline: { display_name: 'Saved name', timezone: 'UTC' },
    value: 'Changed but disabled',
    disabled: true,
    extraField: {name: 'timezone', type: 'text', value: 'UTC', checked: false, disabled: false},
  });
  f.events.get('submit')({ defaultPrevented: false });
  f.resume.focus();
  f.resume.click();
  assert.equal(f.document.activeElement, f.fields[1]);
});

test('explicit recovery selects the changed field rather than an earlier unchanged field', () => {
  const f = fixture({
    baseline: { display_name: 'Saved name', timezone: 'UTC' },
    value: 'Saved name',
    extraField: {name: 'timezone', type: 'text', value: 'Europe/London', checked: false, disabled: false},
  });
  f.events.get('submit')({ defaultPrevented: false });
  f.resume.focus();
  f.resume.click();
  assert.equal(f.document.activeElement, f.fields[1]);
});

test('pending field edits preserve focus even when another field changed first', () => {
  const f = fixture({
    baseline: { display_name: 'Saved name', timezone: 'UTC' },
    value: 'Changed name',
    extraField: {name: 'timezone', type: 'text', value: 'Europe/London', checked: false, disabled: false},
  });
  f.events.get('submit')({ defaultPrevented: false });
  f.fields[1].focus();
  f.events.get('input')();
  assert.equal(f.document.activeElement, f.fields[1]);
});

test('unknown save outcome survives continued typing and returning to the old baseline', () => {
  const f = fixture({ value: 'Submitted name' });
  f.events.get('submit')({ defaultPrevented: false });
  for (const value of ['New', 'Newer', 'Alex Morgan']) {
    f.field.value = value;
    f.field.focus();
    f.events.get('input')();
    assert.match(f.status.textContent, /server may already have saved/);
    assert.equal(f.button.disabled, false);
    assert.equal(f.document.activeElement, f.field);
    assert.equal(f.field.value, value);
    let warned = false;
    const departure = { preventDefault() { warned = true; } };
    f.windowEvents.get('beforeunload')(departure);
    assert.equal(warned, true);
    assert.equal(departure.returnValue, '');
  }
  assert.equal(f.window.stops, 1, 'only the pending navigation is stopped');
  f.windowEvents.get('pageshow')();
  assert.match(f.status.textContent, /server may already have saved/);
  assert.equal(f.button.disabled, false, 'history restoration cannot resolve the save outcome');
});

test('checkbox input/change pair retains uncertainty even when restoring the baseline', () => {
  const f = fixture({ baseline: { digest: true }, value: false, checkbox: true });
  f.events.get('submit')({ defaultPrevented: false });
  f.field.checked = true;
  f.events.get('input')();
  f.events.get('change')();
  assert.match(f.status.textContent, /server may already have saved/);
  assert.equal(f.button.disabled, false);
  assert.equal(f.window.stops, 1);
  f.events.get('submit')({ defaultPrevented: false });
  assert.match(f.status.textContent, /^Saving changes/);
  assert.equal(f.button.disabled, true);
  f.windowEvents.get('beforeunload')({ preventDefault() { throw Error('explicit retry must not warn'); } });
  f.resume.click();
  assert.match(f.status.textContent, /server may already have saved/);
  assert.equal(f.button.disabled, false);
  const fresh = fixture({ baseline: { digest: true }, value: true, checkbox: true });
  fresh.windowEvents.get('pageshow')();
  assert.equal(fresh.status.textContent, 'No unsaved changes');
  assert.equal(fresh.button.disabled, true, 'a fresh authoritative page is clean');
  f.form.isConnected = false;
  f.windowEvents.get('beforeunload')({ preventDefault() { throw Error('detached uncertainty must not warn'); } });
});

test('continued typing does not rewrite the same polite live-region warning', () => {
  const f = fixture({ value: 'Submitted name' });
  f.events.get('submit')({ defaultPrevented: false });
  f.field.value = 'New draft';
  f.events.get('input')();
  const writes = f.status.writes;
  for (const value of ['More typing', 'Alex Morgan', 'Another draft']) {
    f.field.value = value;
    f.events.get('input')();
    f.events.get('change')();
  }
  assert.equal(f.status.writes, writes);
  f.events.get('submit')({ defaultPrevented: false });
  assert.equal(f.status.writes, writes + 1, 'a real status transition is announced');
});
