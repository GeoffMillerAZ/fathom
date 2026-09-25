// Test-only JSON-line browser driver for the synthetic Gallery workflow probe.
// Stdout is reserved for protocol responses; credentials, scripts, URLs, and
// browser error text are intentionally never reported.
import {configuration} from '../../apps/gallery/browser/config.mjs';
import {createHash} from 'node:crypto';
import {
  closeSync,
  fchmodSync,
  lstatSync,
  openSync,
  realpathSync,
  writeFileSync,
} from 'node:fs';
import path from 'node:path';
import {pathToFileURL} from 'node:url';

const MAX_INPUT_FRAME = 256 * 1024;
const MAX_OUTPUT_FRAME = 64 * 1024;
const MAX_TOTAL_INPUT = 4 * 1024 * 1024;
const MAX_TOTAL_OUTPUT = 4 * 1024 * 1024;
const MAX_COMMANDS = 512;
const MAX_CAPTURE_SIZE = 4 * 1024 * 1024;
const PAGE_PATHS = new Set(['/shell/profile', '/shell/activity', '/shell/settings']);
const GET_PATHS = new Set([
  '/shell',
  '/shell/home',
  '/shell/activity',
  '/shell/profile',
  '/shell/settings',
  '/shell/help',
  '/shell/about',
  '/shell/events',
  '/static/app.css',
  '/static/shell-demo.js',
  '/static/shell-live-entry.js',
  '/static/shell-live.js',
  '/static/datastar-v1.0.3.js',
]);
const NAME = /^[a-zA-Z0-9_-]{1,32}$/;
const TOKEN = /^[a-zA-Z0-9_-]{43}$/;
const EXPECTED_CONSOLE = new Map([
  ['POST /shell/action 422', 1],
  ['POST /shell/action 409', 1],
  ['POST /shell/action 503', 1],
  ['GET /shell/profile 401', 1],
]);

let inputBytes = 0;
let outputBytes = 0;
let commandCount = 0;
let stopped = false;
let browser;
let context;
let origin;
let captureDir = '';
let creatingPage = false;
const pages = new Map();
const launchURLs = new Set();
const observations = {
  blockedRequests: 0,
  websockets: 0,
  workers: 0,
  pageErrors: 0,
  dialogs: 0,
  consoleErrors: 0,
  expectedConsole: new Map(),
  unexpectedPages: 0,
  crashes: 0,
  cleanupErrors: 0,
};

function validID(id) {
  return Number.isSafeInteger(id) || (typeof id === 'string' && NAME.test(id));
}

function response(id, ok, valueOrError) {
  const message = ok
    ? {id, ok: true, value: valueOrError}
    : {id: validID(id) ? id : null, ok: false, error: valueOrError};
  const line = `${JSON.stringify(message)}\n`;
  const bytes = Buffer.byteLength(line);
  if (bytes > MAX_OUTPUT_FRAME || outputBytes + bytes > MAX_TOTAL_OUTPUT) {
    stopped = true;
    const fallback = `${JSON.stringify({id: null, ok: false, error: 'output-limit'})}\n`;
    if (outputBytes + Buffer.byteLength(fallback) <= MAX_TOTAL_OUTPUT) process.stdout.write(fallback);
    return false;
  }
  outputBytes += bytes;
  process.stdout.write(line);
  return true;
}

export function requestClass(originValue, method, rawURL, launchSet = new Set()) {
  let url;
  try { url = new URL(rawURL); } catch { return 'blocked'; }
  if (url.origin !== originValue || url.username || url.password || url.hash) return 'blocked';
  if (method === 'GET' && launchSet.has(url.href)) return 'launch';
  if ((method === 'GET' || method === 'HEAD') && !url.search && GET_PATHS.has(url.pathname)) return 'read';
  if (method === 'POST' && !url.search && url.pathname === '/shell/action') return 'action';
  return 'blocked';
}

export function consoleErrorClass(originValue, text, rawURL) {
  let url;
  try { url = new URL(rawURL); } catch { return 'unexpected'; }
  if (url.origin !== originValue) return 'unexpected';
  const match = /^Failed to load resource: the server responded with a status of (401 \(Unauthorized\)|409 \(Conflict\)|422 \(Unprocessable Entity\)|503 \(Service Unavailable\))$/.exec(text);
  if (!match) return 'unexpected';
  const status = match[1].slice(0, 3);
  const method = url.pathname === '/shell/action' ? 'POST' : url.pathname === '/shell/profile' ? 'GET' : '';
  const key = `${method} ${url.pathname} ${status}`;
  return EXPECTED_CONSOLE.has(key) ? key : 'unexpected';
}

export function consoleMultisetDifference(observed) {
  return [...EXPECTED_CONSOLE].reduce((sum, [key, count]) =>
    sum + Math.abs((observed.get(key) || 0) - count), 0);
}

export function browserInitOptions(command) {
  const headed = command?.headed === undefined ? false : command.headed;
  if (typeof headed !== 'boolean') throw new Error('state');
  const requestedDir = command?.captureDir === undefined ? '' : command.captureDir;
  if (typeof requestedDir !== 'string') throw new Error('state');
  if (!requestedDir) return {headed, captureDir: ''};
  if (!path.isAbsolute(requestedDir)) throw new Error('state');
  let info;
  let real;
  try {
    info = lstatSync(requestedDir);
    real = realpathSync(requestedDir);
  } catch {
    throw new Error('state');
  }
  if (!info.isDirectory() || info.isSymbolicLink() || path.resolve(requestedDir) !== real) {
    throw new Error('state');
  }
  return {headed, captureDir: real};
}

export function writeViewportCapture(directory, name, png, viewport) {
  if (!directory || !NAME.test(name || '') || !Buffer.isBuffer(png) ||
      png.length === 0 || png.length > MAX_CAPTURE_SIZE ||
      !Number.isSafeInteger(viewport?.width) || viewport.width <= 0 ||
      !Number.isSafeInteger(viewport?.height) || viewport.height <= 0) {
    throw new Error('state');
  }
  const file = `${name}.png`;
  const target = path.join(directory, file);
  let descriptor;
  try {
    descriptor = openSync(target, 'wx', 0o600);
    fchmodSync(descriptor, 0o600);
    writeFileSync(descriptor, png);
  } finally {
    if (descriptor !== undefined) closeSync(descriptor);
  }
  return JSON.stringify({
    file,
    sha256: createHash('sha256').update(png).digest('hex'),
    width: viewport.width,
    height: viewport.height,
  });
}

function launchURL(raw) {
  if (typeof raw !== 'string' || raw.length > 256) throw new Error('launch');
  const parsed = new URL(raw);
  if (parsed.origin !== origin || parsed.pathname !== '/auth/launch' || parsed.hash ||
      parsed.username || parsed.password || [...parsed.searchParams.keys()].length !== 1 ||
      !TOKEN.test(parsed.searchParams.get('token') || '') ||
      raw !== `${origin}/auth/launch?token=${parsed.searchParams.get('token')}`) {
    throw new Error('launch');
  }
  return raw;
}

function observePage(page) {
  page.on('crash', () => { observations.crashes += 1; });
  page.on('pageerror', () => { observations.pageErrors += 1; });
  page.on('dialog', async dialog => {
    observations.dialogs += 1;
    await dialog.dismiss().catch(() => {});
  });
  page.on('console', message => {
    if (message.type() !== 'error') return;
    const location = message.location();
    const text = message.text();
    const key = consoleErrorClass(origin, text, location.url);
    if (key !== 'unexpected') {
      observations.expectedConsole.set(key, (observations.expectedConsole.get(key) || 0) + 1);
      return;
    }
    observations.consoleErrors += 1;
  });
}

async function initialise(command) {
  if (context || typeof command.origin !== 'string') throw new Error('state');
  const options = browserInitOptions(command);
  const config = configuration({...process.env, APPKIT_SHELL_BROWSER_ORIGIN: command.origin});
  if (!/^http:\/\/127\.0\.0\.1:[1-9][0-9]{0,4}$/.test(config.base)) throw new Error('origin');
  origin = config.base;
  const {chromium} = await import(config.moduleURL);
  browser = await chromium.launch({headless: !options.headed, executablePath: config.executablePath});
  context = await browser.newContext({
    viewport: {width: 1440, height: 1000},
    serviceWorkers: 'block',
  });
  context.setDefaultTimeout(5000);
  captureDir = options.captureDir;
  context.on('serviceworker', () => { observations.workers += 1; });
  context.on('page', page => {
    if (!creatingPage) observations.unexpectedPages += 1;
    observePage(page);
  });
  await context.route('**/*', async route => {
    const request = route.request();
    const rawURL = request.url();
    const method = request.method();
    const kind = requestClass(origin, method, rawURL, launchURLs);
    if (kind === 'launch') launchURLs.delete(rawURL);
    if (kind !== 'blocked') await route.continue();
    else {
      observations.blockedRequests += 1;
      await route.abort('blockedbyclient');
    }
  });
  await context.routeWebSocket('**/*', websocket => {
    observations.websockets += 1;
    websocket.close();
  });
}

async function closeOwnedBrowser() {
  const ownedContext = context;
  const ownedBrowser = browser;
  context = undefined;
  browser = undefined;
  pages.clear();
  let failures = 0;
  if (ownedContext) {
    try { await ownedContext.close(); } catch { failures += 1; }
  }
  if (ownedBrowser) {
    try { await ownedBrowser.close(); } catch { failures += 1; }
  }
  return failures;
}

function category(error, fallback) {
  if (error?.message === 'launch') return 'invalid-launch';
  if (error?.message === 'state') return 'invalid-state';
  return fallback;
}

async function command(raw) {
  let item;
  try { item = JSON.parse(raw); } catch {
    response(null, false, 'invalid-json');
    return;
  }
  const id = item?.id;
  if (!validID(id) || typeof item?.op !== 'string') {
    response(id, false, 'invalid-command');
    return;
  }
  try {
    switch (item.op) {
      case 'init':
        await initialise(item);
        response(id, true, JSON.stringify({version: await browser.version(), headed: item.headed ?? false}));
        break;
      case 'new': { // All pages share the one fresh context and its process-session cookie.
        if (!context || !NAME.test(item.name || '') || pages.has(item.name)) throw new Error('state');
        const target = launchURL(item.launch);
        launchURLs.add(target);
        creatingPage = true;
        let page;
        try { page = await context.newPage(); } finally { creatingPage = false; }
        try {
          await page.goto(target, {waitUntil: 'load'});
          pages.set(item.name, page);
        } catch (error) {
          await page.close().catch(() => {});
          throw error;
        } finally {
          launchURLs.delete(target);
        }
        response(id, true, 'ready');
        break;
      }
      case 'eval': {
        const page = pages.get(item.name);
        if (!page || typeof item.script !== 'string' || Buffer.byteLength(item.script) > MAX_INPUT_FRAME) throw new Error('state');
        const value = await page.evaluate(item.script);
        if (typeof value !== 'string') throw new Error('result');
        if (Buffer.byteLength(value) > MAX_OUTPUT_FRAME / 2) throw new Error('result');
        response(id, true, value);
        break;
      }
      case 'navigate': {
        const page = pages.get(item.name);
        if (!page || !PAGE_PATHS.has(item.path)) throw new Error('state');
        await page.goto(`${origin}${item.path}`, {waitUntil: 'load'});
        response(id, true, 'ready');
        break;
      }
      case 'capture': {
        const page = pages.get(item.name);
        if (!page || !captureDir) throw new Error('state');
        const viewport = page.viewportSize();
        const png = await page.screenshot({type: 'png', fullPage: false});
        response(id, true, writeViewportCapture(captureDir, item.name, png, viewport));
        break;
      }
      case 'close': {
        const page = pages.get(item.name);
        if (!page) throw new Error('state');
        pages.delete(item.name);
        await page.close();
        response(id, true, 'closed');
        break;
      }
      case 'finish': {
        observations.cleanupErrors += await closeOwnedBrowser();
        const missingConsole = consoleMultisetDifference(observations.expectedConsole);
        const failures = observations.blockedRequests + observations.websockets +
          observations.workers + observations.pageErrors + observations.dialogs +
          observations.consoleErrors + observations.unexpectedPages + observations.crashes +
          observations.cleanupErrors + missingConsole;
        const safeCounts = `blocked=${observations.blockedRequests},ws=${observations.websockets},workers=${observations.workers},pageerrors=${observations.pageErrors},dialogs=${observations.dialogs},console=${observations.consoleErrors},expected=${missingConsole},pages=${observations.unexpectedPages},crashes=${observations.crashes},cleanup=${observations.cleanupErrors}`;
        response(id, failures === 0,
          failures === 0 ? 'finished' : `browser-observation:${safeCounts}`);
        stopped = true;
        break;
      }
      default:
        response(id, false, 'invalid-command');
    }
  } catch (error) {
    response(id, false, category(error,
      item.op === 'init' ? 'initialization' :
      item.op === 'new' || item.op === 'navigate' ? 'navigation' :
      item.op === 'eval' ? 'evaluation' : 'operation'));
  }
}

async function main() {
  // Prevent inherited Playwright debugging switches from writing sensitive browser
  // diagnostics outside this deliberately small protocol.
  delete process.env.DEBUG;
  delete process.env.PWDEBUG;
  let pending = '';
  process.stdin.setEncoding('utf8');
  for await (const chunk of process.stdin) {
    inputBytes += Buffer.byteLength(chunk);
    if (inputBytes > MAX_TOTAL_INPUT) {
      response(null, false, 'input-limit');
      stopped = true;
      break;
    }
    pending += chunk;
    if (!pending.includes('\n') && Buffer.byteLength(pending) > MAX_INPUT_FRAME) {
      response(null, false, 'input-limit');
      stopped = true;
      break;
    }
    for (;;) {
      const newline = pending.indexOf('\n');
      if (newline < 0) break;
      const line = pending.slice(0, newline).replace(/\r$/, '');
      pending = pending.slice(newline + 1);
      if (Buffer.byteLength(line) > MAX_INPUT_FRAME || ++commandCount > MAX_COMMANDS) {
        response(null, false, Buffer.byteLength(line) > MAX_INPUT_FRAME ? 'input-limit' : 'command-limit');
        stopped = true;
        break;
      }
      if (line) await command(line);
      if (stopped) break;
    }
    if (stopped) break;
  }
  if (!stopped && pending.trim()) {
    if (Buffer.byteLength(pending) > MAX_INPUT_FRAME || ++commandCount > MAX_COMMANDS) {
      response(null, false, Buffer.byteLength(pending) > MAX_INPUT_FRAME ? 'input-limit' : 'command-limit');
    } else await command(pending.replace(/\r$/, ''));
  }
  await closeOwnedBrowser();
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  await main().catch(async () => {
    response(null, false, 'fatal');
    await closeOwnedBrowser();
    process.exitCode = 1;
  });
}
