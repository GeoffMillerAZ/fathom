import {mountShellLive} from '/static/shell-live.js';

// Register before importing: the pinned engine announces its initial attribute
// scan asynchronously. Keep Connect inert until its click binding exists.
const ready = new Promise((resolve) => document.addEventListener('datastar-ready', resolve, {once: true}));
const {getPath, mergePatch} = await import('/static/datastar-v1.0.3.js');
await ready;
window.AppKitShellLive = mountShellLive(document, window,
  () => getPath('appkitShellProbe'), () => mergePatch({appkitShellProbe: null}));
