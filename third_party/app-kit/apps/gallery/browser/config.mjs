// Opt-in only: existing approved Playwright installation, installed browser,
// explicit local Gallery origin, and a fresh temporary profile/output directory.
import path from 'node:path';
import os from 'node:os';
import { mkdtempSync } from 'node:fs';
import { pathToFileURL } from 'node:url';

export function configuration(env = process.env) {
  const raw = env.APPKIT_SHELL_BROWSER_ORIGIN;
  if (!raw) throw new Error('Set APPKIT_SHELL_BROWSER_ORIGIN to the explicitly approved synthetic Gallery HTTP loopback origin.');
  const url = new URL(raw);
  if (url.protocol !== 'http:' || !['127.0.0.1', '[::1]'].includes(url.hostname) || !url.port || raw !== url.origin) {
    throw new Error('Gallery origin must be an exact HTTP loopback IP origin with a non-default explicit port, no credentials, path, query or fragment.');
  }
  const modulePath = env.APPKIT_PLAYWRIGHT_MODULE;
  if (!modulePath || !path.isAbsolute(modulePath)) throw new Error('Set APPKIT_PLAYWRIGHT_MODULE to the absolute index.mjs of an existing approved isolated Playwright installation; this probe installs nothing.');
  const executablePath = env.APPKIT_BROWSER_EXECUTABLE || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
  if (!path.isAbsolute(executablePath)) throw new Error('APPKIT_BROWSER_EXECUTABLE must be an absolute path to an installed browser.');
  const variant = env.APPKIT_BROWSER_VARIANT || 'candidate';
  if (!/^[a-z0-9-]{1,32}$/.test(variant)) throw new Error('APPKIT_BROWSER_VARIANT must contain 1–32 lowercase letters, digits or hyphens.');
  return {base: url.origin, moduleURL: pathToFileURL(modulePath).href, executablePath, variant};
}

export async function bootstrap() {
  const config = configuration();
  const {chromium} = await import(config.moduleURL);
  const root = mkdtempSync(path.join(os.tmpdir(), 'appkit-shell-browser-'));
  console.log(`Synthetic shell browser evidence: ${root}`);
  return {...config, chromium, root};
}
