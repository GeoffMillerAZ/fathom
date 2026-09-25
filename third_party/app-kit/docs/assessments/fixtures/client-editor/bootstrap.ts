import './preload-icons';
import { mount, type Handle } from './island';
import { parseInitial } from './protocol';

document.documentElement.dataset.colorTheme = 'BLUE_THEME';
document.documentElement.classList.toggle('dark', matchMedia('(prefers-color-scheme: dark)').matches);

const slot = document.getElementById('editor-slot');
const config = document.getElementById('editor-config');
const state = document.getElementById('host-state');
if (!(slot instanceof HTMLElement) || !config || !state) throw new Error('Missing host contract');
const initial = parseInitial(JSON.parse(config.textContent ?? ''));
let primary: Handle | undefined;
let changes = 0;
function status(message: string) {
  // Repeated keystrokes must not turn the polite status into a revision announcer.
  if (state!.textContent !== message) state!.textContent = message;
}
function mountPrimary() {
  if (primary && !primary.disposed) { status('Editor is already mounted. Draft unchanged.'); return; }
  primary = mount(slot!, initial, () => {
    changes++;
    status('Local draft changed. Nothing is saved.');
  });
  status('Editor ready. Nothing is saved.');
}
mountPrimary();

document.getElementById('host-update')!.addEventListener('click', async () => {
  // Capture this instance: a slow response must not update a replacement editor.
  const target = primary;
  if (!target || target.disposed) { status('Mount the editor before updating context.'); return; }
  try {
    const response = await fetch('/context', { credentials: 'same-origin', cache: 'no-store' });
    if (!response.ok) throw new Error('Context request failed');
    const value: unknown = await response.json();
    if (target !== primary || target.disposed) { status('Old context response ignored.'); return; }
    target.update(value);
    status('Context updated. Local draft unchanged.');
  } catch {
    status('Context could not update. Local draft unchanged.');
  }
});
document.getElementById('host-dispose')!.addEventListener('click', () => {
  primary?.dispose();
  status('Editor disposed. Unsaved draft discarded.');
});
document.getElementById('host-mount')!.addEventListener('click', mountPrimary);

// Explicit synthetic-test entrypoint; this page has no production client API.
Object.defineProperty(window, 'P12Specimen', { value: Object.freeze({
  mount,
  get primary() { return primary; },
  get changes() { return changes; },
}), configurable: false });
