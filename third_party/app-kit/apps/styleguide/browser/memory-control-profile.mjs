const BASELINE_FLAGS = Object.freeze([
  '--headless=new',
  '--remote-debugging-address=127.0.0.1',
  '--remote-debugging-port=0',
  '--user-data-dir={PROFILE}',
  '--disk-cache-dir={PROFILE}/cache',
  '--window-size=1440,1000',
  '--disable-background-networking',
  '--no-proxy-server',
  '--no-first-run',
  '--no-default-browser-check',
  'about:blank',
]);

const NO_BACKGROUND_COMPONENTS = '--disable-component-extensions-with-background-pages';
const PROFILES = new Set(['baseline', 'no-background-components-v1', 'no-background-components-controls-v1',
  'no-background-components-shutdown-v1']);

function requireProfile(profile) {
  if (typeof profile !== 'string') throw new TypeError('profile must be a supported string');
  if (!PROFILES.has(profile)) throw new RangeError(`unsupported memory control profile: ${profile}`);
}

export function memoryControlFlags(profile = 'baseline') {
  requireProfile(profile);
  if (profile === 'baseline') return [...BASELINE_FLAGS];
  return [...BASELINE_FLAGS.slice(0, -1), NO_BACKGROUND_COMPONENTS, BASELINE_FLAGS.at(-1)];
}

export function memoryControlModes(profile = 'baseline') {
  requireProfile(profile);
  if (profile === 'no-background-components-v1') return ['startup'];
  if (profile === 'no-background-components-shutdown-v1') return ['shutdown'];
  return ['positive', 'retention'];
}
