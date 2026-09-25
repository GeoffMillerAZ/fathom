// Private, optional proof module. No declarative expressions or editing ownership.
import { actions } from './datastar-v1.0.3.js';

const button = document.getElementById('host-morph');
const status = document.getElementById('host-state');
if (!button || !status) throw new Error('Missing morph host contract');
let pending;
async function request() {
  if (pending) return { outcome: 'busy', events: [] };
  const controller = new AbortController();
  pending = controller;
  const events = [];
  const collect = event => {
    if (event.detail.el === button) events.push({ type: event.detail.type, args: event.detail.argsRaw });
  };
  document.addEventListener('datastar-fetch', collect);
  const timeout = setTimeout(() => controller.abort(), 5000);
  let error = '';
  try {
    await actions.get({ el: button, error: name => new Error(name), cleanups: new Map() }, '/morph-fragment', {
      contentType: 'form', selector: '#morph-transport-form', headers: { Accept: 'text/html' },
      openWhenHidden: true, retry: 'never', retryMaxCount: 0, requestCancellation: controller,
    });
  } catch (reason) { error = String(reason); }
  finally {
    clearTimeout(timeout);
    document.removeEventListener('datastar-fetch', collect);
    pending = undefined;
  }
  const failed = !!error || events.some(event => ['error', 'retries-failed'].includes(event.type));
  const patched = events.some(event => event.type === 'datastar-patch-elements');
  const outcome = controller.signal.aborted ? 'cancelled' : failed || !patched ? 'failed' : 'updated';
  status.textContent = outcome === 'updated' ? 'Surrounding context updated. Local draft unchanged.'
    : outcome === 'cancelled' ? 'Surrounding update cancelled. Local draft unchanged.'
    : 'Surrounding context could not update. Local draft unchanged.';
  return { outcome, events, error };
}
button.addEventListener('click', () => { void request(); });
Object.defineProperty(window, 'P12Morph', { value: Object.freeze({
  request, cancel() { pending?.abort(); }, get pending() { return !!pending; },
}), configurable: false });
