import {createServer} from 'node:http';

const htmlHeaders = csp => ({
  'cache-control': 'no-store',
  'content-security-policy': csp,
  'content-type': 'text/html; charset=utf-8',
  'referrer-policy': 'no-referrer',
  'x-content-type-options': 'nosniff',
});
const parentCSP = "default-src 'none'; script-src 'self'; frame-src 'self'; form-action 'self'; base-uri 'none'; object-src 'none'; style-src 'none'; img-src 'none'; font-src 'none'; connect-src 'none'; frame-ancestors 'none'";
const previewCSP = "default-src 'none'; script-src 'none'; form-action 'none'; base-uri 'none'; object-src 'none'; style-src 'none'; img-src 'none'; font-src 'none'; connect-src 'none'; frame-ancestors 'self'";

function parent(mode) {
  const retention = mode === 'retention' ? `
    <button id="allocate" type="button">Retain 8 MiB allocation</button>
    <p id="allocation-state" role="status">No retained allocation</p>
    <script src="/control.js"></script>` : '';
  return `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Memory lifecycle control</title></head>
<body><main><h1>Memory lifecycle control</h1>
  <form action="/theme-lab/preview" method="get" target="preview">
    <button id="next" type="submit">Next preview generation</button>
  </form>${retention}
  <iframe id="preview" name="preview" sandbox="" src="/theme-lab/preview" title="Opaque preview generation"></iframe>
</main></body></html>`;
}

const retentionScript = `'use strict';
const button = document.getElementById('allocate');
const status = document.getElementById('allocation-state');
button.addEventListener('click', () => {
  if (!(globalThis.__appkitRetainedAllocation instanceof Uint8Array)) {
    globalThis.__appkitRetainedAllocation = new Uint8Array(8 * 1024 * 1024);
    globalThis.__appkitRetainedAllocation.fill(0x5a);
  }
  status.textContent = 'Retained ' + globalThis.__appkitRetainedAllocation.byteLength + ' bytes';
});
`;

export function createControlServer({mode} = {}) {
  if (mode !== 'positive' && mode !== 'retention') throw new Error("mode must be exactly 'positive' or 'retention'");
  let generation = 0;
  return createServer((request, response) => {
    let parsed, pathname;
    try { parsed = new URL(request.url, 'http://fixture.invalid'); pathname = parsed.pathname; }
    catch { response.writeHead(400, {'cache-control': 'no-store', 'x-content-type-options': 'nosniff'}); response.end('bad request'); return; }
    const known = new Set(['/theme-lab', '/theme-lab/preview', '/control.js']);
    if (!known.has(pathname) || parsed.search || parsed.hash) {
      response.writeHead(404, {'cache-control': 'no-store', 'content-type': 'text/plain; charset=utf-8', 'x-content-type-options': 'nosniff'});
      response.end('not found'); return;
    }
    if (request.method !== 'GET') {
      response.writeHead(405, {'allow': 'GET', 'cache-control': 'no-store', 'content-type': 'text/plain; charset=utf-8', 'x-content-type-options': 'nosniff'});
      response.end('method not allowed'); return;
    }
    if (pathname === '/theme-lab') {
      response.writeHead(200, htmlHeaders(parentCSP)); response.end(parent(mode)); return;
    }
    if (pathname === '/theme-lab/preview') {
      generation++;
      response.writeHead(200, htmlHeaders(previewCSP));
      response.end(`<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Preview</title></head><body><h1 id="generation">Generation ${generation}</h1></body></html>`);
      return;
    }
    response.writeHead(200, {
      'cache-control': 'no-store',
      'content-security-policy': "default-src 'none'",
      'content-type': 'text/javascript; charset=utf-8',
      'referrer-policy': 'no-referrer',
      'x-content-type-options': 'nosniff',
    });
    response.end(mode === 'retention' ? retentionScript : `'use strict';\n`);
  });
}
