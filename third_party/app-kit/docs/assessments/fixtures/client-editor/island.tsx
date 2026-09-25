// First-party lifecycle adapter for the private P12 source, not a kit public API.
import { createRoot, type Root } from 'react-dom/client';
import { flushSync } from 'react-dom';
import type { Editor, JSONContent } from '@tiptap/core';
import MyEditor from './TiptapEdit';
import { parseContext, parseInitial, type Context } from './protocol';

export type Snapshot = Readonly<{ instanceId: string; revision: number; text: string; document: JSONContent }>;
export type Handle = Readonly<{
  readonly ready: boolean;
  readonly disposed: boolean;
  update(value: unknown): void;
  snapshot(): Snapshot | null;
  dispose(): void;
}>;

const owners = new WeakMap<HTMLElement, Handle>();
const liveIDs = new Set<string>();

export function mount(element: HTMLElement, value: unknown, onChange: (snapshot: Snapshot) => void = () => {}): Handle {
  const initial = parseInitial(value);
  if (!(element instanceof HTMLElement) || !element.isConnected || element.childNodes.length !== 0) {
    throw new TypeError('Mount requires a connected empty HTML element');
  }
  if (owners.has(element)) throw new Error('Element already has an editor owner');
  if (liveIDs.has(initial.instanceId)) throw new Error('Instance ID already has a live editor');
  if (typeof onChange !== 'function') throw new TypeError('onChange must be a function');
  let root: Root | undefined;
  let editor: Editor | undefined;
  let disposed = false;
  let revision = 0;
  let current: Context = initial;

  function snapshot(): Snapshot | null {
    if (disposed) throw new Error('Editor is disposed');
    if (!editor || editor.isDestroyed) return null;
    // Never return ProseMirror-owned objects or promise that serialized HTML is safe.
    return Object.freeze({ instanceId: initial.instanceId, revision, text: editor.getText(), document: structuredClone(editor.getJSON()) });
  }
  function changed() {
    if (disposed) return;
    revision++;
    const next = snapshot();
    if (next) onChange(next);
  }
  function ready(instance: Editor) {
    if (disposed) { instance.destroy(); return; }
    if (editor === instance) return;
    if (editor) throw new Error('Editor instance unexpectedly changed');
    editor = instance;
    editor.on('update', changed);
  }
  function render() {
    flushSync(() => root!.render(<MyEditor initialText={initial.initialText} context={current} onReady={ready} />));
  }
  const handle: Handle = Object.freeze({
    get ready() { return !disposed && !!editor && !editor.isDestroyed; },
    get disposed() { return disposed; },
    update(value: unknown) {
      if (disposed) throw new Error('Editor is disposed');
      const next = parseContext(value, initial.instanceId);
      current = next;
      render();
    },
    snapshot,
    dispose() {
      if (disposed) return;
      disposed = true;
      editor?.off('update', changed);
      // Unmount effects first; explicitly destroy instead of relying on deferred hook cleanup.
      flushSync(() => root?.unmount());
      if (editor && !editor.isDestroyed) editor.destroy();
      owners.delete(element);
      liveIDs.delete(initial.instanceId);
    },
  });
  owners.set(element, handle);
  liveIDs.add(initial.instanceId);
  try {
    root = createRoot(element, { identifierPrefix: `p12-${initial.instanceId}-` });
    render();
  } catch (error) {
    handle.dispose();
    throw error;
  }
  return handle;
}
