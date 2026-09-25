// Optional, dependency-free local feedback for the private P10 native form.
// It is deliberately not a submission, persistence, or validation authority.

const BIO_LIMIT = 240;
const CONFIG_BYTES = 64 << 10;
const VALUE_FIELDS = [
  'name', 'bio', 'email', 'address', 'city', 'postal_code',
  'country[code]', 'region', 'currency',
];
const CONFIG_STRING_FIELDS = [
  'name', 'bio', 'email', 'address', 'city', 'postal_code',
  'country', 'region', 'currency',
];
const CONFIG_FIELD_BYTES = new Map([
  ['name', 512], ['bio', 16 << 10], ['email', 320], ['address', 512],
  ['city', 256], ['postal_code', 64], ['country', 16], ['region', 128],
  ['currency', 16],
]);
const ERROR_FIELDS = ['bio', 'email', 'country', 'region', 'currency'];
const mounted = new WeakMap();

export function countCodePoints(value) {
  return Array.from(value).length;
}

// Form encoding expands logical textarea newlines to CRLF. Keep the raw
// configuration value intact, but count it as textarea.value will expose it.
export function textareaCodePoints(value) {
  return countCodePoints(value) - (value.match(/\r\n/g)?.length || 0);
}

function exactKeys(value, keys) {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return false;
  const actual = Object.keys(value).sort();
  const expected = [...keys].sort();
  return actual.length === expected.length && actual.every((key, index) => key === expected[index]);
}

function boundedUTF8Length(value, limit) {
  if (typeof value !== 'string') return -1;
  let bytes = 0;
  for (let index = 0; index < value.length; index++) {
    const unit = value.charCodeAt(index);
    if (unit >= 0xd800 && unit <= 0xdbff) {
      if (++index >= value.length) return -1;
      const low = value.charCodeAt(index);
      if (low < 0xdc00 || low > 0xdfff) return -1;
      bytes += 4;
    } else if (unit >= 0xdc00 && unit <= 0xdfff) {
      return -1;
    } else {
      bytes += unit <= 0x7f ? 1 : unit <= 0x7ff ? 2 : 3;
    }
    if (bytes > limit) return -1;
  }
  return bytes;
}

function validConfigText(field, value) {
  if (boundedUTF8Length(value, CONFIG_FIELD_BYTES.get(field)) < 0) return false;
  for (const character of value) {
    const code = character.codePointAt(0);
    if ((code <= 0x1f || code >= 0x7f && code <= 0x9f) &&
        !(field === 'bio' && (code === 0x09 || code === 0x0a || code === 0x0d))) return false;
  }
  return true;
}

// JSON.parse establishes syntax first. This bounded token walk adds the one
// property JSON.parse discards: whether an object member occurred twice.
function hasDuplicateJSONMembers(source) {
  const stack = [];
  const whitespace = /\s/;
  for (let index = 0; index < source.length; index++) {
    const character = source[index];
    if (whitespace.test(character)) continue;
    if (character === '{') {
      stack.push({ type: 'object', keys: new Set(), expectKey: true });
      continue;
    }
    if (character === '[') {
      stack.push({ type: 'array' });
      continue;
    }
    if (character === '}' || character === ']') {
      stack.pop();
      continue;
    }
    if (character === ',') {
      const frame = stack[stack.length - 1];
      if (frame?.type === 'object') frame.expectKey = true;
      continue;
    }
    if (character !== '"') continue;
    const start = index;
    for (index++; index < source.length; index++) {
      if (source[index] === '\\') {
        index++;
      } else if (source[index] === '"') {
        break;
      }
    }
    const frame = stack[stack.length - 1];
    if (frame?.type !== 'object' || !frame.expectKey) continue;
    const key = JSON.parse(source.slice(start, index + 1));
    if (frame.keys.has(key)) return true;
    frame.keys.add(key);
    frame.expectKey = false;
  }
  return false;
}

// Kept exported for source-only tests. The DOM seam supplies the canonical options;
// this parses only the app-selected initial snapshot and its fixed protocol shape.
export function parseSettingsConfig(source) {
  if (boundedUTF8Length(source, CONFIG_BYTES) < 0) return null;
  let config;
  try {
    config = JSON.parse(source);
  } catch {
    return null;
  }
  if (hasDuplicateJSONMembers(source)) return null;
  if (!exactKeys(config, ['schema_version', 'bio_limit', 'initial']) ||
      config.schema_version !== 1 || config.bio_limit !== BIO_LIMIT ||
      !exactKeys(config.initial, [...CONFIG_STRING_FIELDS, 'email_is_public'])) return null;
  for (const field of CONFIG_STRING_FIELDS) {
    if (!validConfigText(field, config.initial[field])) return null;
  }
  if (typeof config.initial.email_is_public !== 'boolean') return null;
  if (textareaCodePoints(config.initial.bio) > BIO_LIMIT) return null;
  return config;
}

export function retainedRegionAfterCountryChange(groups, country, region) {
  return groups.get(country)?.regions.has(region) ? region : '';
}

function hasDescribedBy(control, id) {
  return (control.getAttribute('aria-describedby') || '').split(/\s+/).includes(id);
}

function formPrefix(id) {
  return typeof id === 'string' && id.endsWith('-form') && id.length > 5 ? id.slice(0, -5) : '';
}

function directControls(form) {
  const found = new Map();
  for (const control of form.querySelectorAll('input, textarea, select')) {
    if (control.form !== form || !VALUE_FIELDS.includes(control.name) && control.name !== 'email_is_public') continue;
    if (found.has(control.name)) return null;
    found.set(control.name, control);
  }
  return found.size === 10 ? found : null;
}

function uniqueIDs(doc) {
  const unique = new Map();
  const duplicates = new Set();
  for (const node of doc.querySelectorAll('[id]')) {
    if (unique.has(node.id)) duplicates.add(node.id);
    else unique.set(node.id, node);
  }
  for (const id of duplicates) unique.delete(id);
  return unique;
}

function flatSelectInventory(select) {
  const values = new Set();
  let placeholder = null;
  let invalid = null;
  for (const option of select.children) {
    if (option.tagName !== 'OPTION') return null;
    if (option.value === '') {
      if (placeholder || option.hasAttribute('data-p10-invalid')) return null;
      placeholder = option;
    } else if (option.hasAttribute('data-p10-invalid')) {
      if (invalid) return null;
      invalid = option;
    } else {
      if (values.has(option.value)) return null;
      values.add(option.value);
    }
  }
  if (!placeholder || invalid && values.has(invalid.value)) return null;
  if (select.value !== '' && !values.has(select.value) && (!invalid || select.value !== invalid.value)) return null;
  return values;
}

function selectedOption(select) {
  return Array.from(select.options).find((option) => option.selected) || null;
}

function copyOption(select, value, fallback) {
  const selected = selectedOption(select);
  if (selected && selected.value === value) return selected.cloneNode(true);
  const option = select.ownerDocument.createElement('option');
  option.value = value;
  option.textContent = fallback;
  return option;
}

function preflight(form) {
  if (!form || form.nodeType !== 1 || form.tagName !== 'FORM' || !form.hasAttribute('data-p10-settings')) return null;
  const doc = form.ownerDocument;
  const prefix = formPrefix(form.id);
  if (!doc || !prefix) return null;
  const ids = uniqueIDs(doc);
  if (ids.get(form.id) !== form) return null;

  // Do not construct an ID-bearing selector from form.id. A malformed or hostile ID
  // cannot affect the lookup because equality is evaluated only after static selection.
  const configurations = Array.from(doc.querySelectorAll('template[data-p10-config-for]'))
    .filter((template) => template.getAttribute('data-p10-config-for') === form.id);
  if (configurations.length !== 1) return null;
  const template = configurations[0];
  if (template.id !== `${prefix}-config` || ids.get(template.id) !== template ||
      template.parentElement !== form.parentElement || !template.content) return null;
  const config = parseSettingsConfig(template.content.textContent);
  if (!config) return null;

  const controls = directControls(form);
  if (!controls) return null;
  const bio = controls.get('bio');
  const country = controls.get('country[code]');
  const region = controls.get('region');
  const currency = controls.get('currency');
  const emailPublic = controls.get('email_is_public');
  if (bio.tagName !== 'TEXTAREA' || country.tagName !== 'SELECT' || region.tagName !== 'SELECT' ||
      currency.tagName !== 'SELECT' || emailPublic.tagName !== 'INPUT' || emailPublic.type !== 'checkbox') return null;
  for (const field of VALUE_FIELDS) {
    const control = controls.get(field);
    const idField = field === 'country[code]' ? 'country' : field;
    if (control.id !== `${prefix}-${idField}` || ids.get(control.id) !== control) return null;
  }
  if (emailPublic.id !== `${prefix}-email_is_public` || ids.get(emailPublic.id) !== emailPublic) return null;

  const count = ids.get(`${prefix}-bio-count`);
  const statuses = Array.from(form.querySelectorAll('[role="status"]'));
  if (!count || !form.contains(count) || statuses.length !== 1 || statuses[0].id !== `${prefix}-status` ||
      ids.get(statuses[0].id) !== statuses[0] ||
      !hasDescribedBy(bio, count.id) || !hasDescribedBy(bio, `${prefix}-bio-error`)) return null;

  const errors = new Map();
  for (const error of form.querySelectorAll('p[data-p10-error]')) {
    const field = error.getAttribute('data-p10-error');
    if (!ERROR_FIELDS.includes(field) || errors.has(field) || error.id !== `${prefix}-${field}-error` ||
        ids.get(error.id) !== error) return null;
    errors.set(field, error);
  }
  if (errors.size !== ERROR_FIELDS.length || ERROR_FIELDS.some((field) => !errors.has(field))) return null;
  for (const field of ['email', 'country', 'region', 'currency']) {
    const control = controls.get(field === 'country' ? 'country[code]' : field);
    if (!hasDescribedBy(control, `${prefix}-${field}-error`)) return null;
  }

  const countries = flatSelectInventory(country);
  const currencies = flatSelectInventory(currency);
  if (!countries || !currencies || countries.size === 0 || currencies.size === 0) return null;
  const groups = new Map();
  let emptyRegionOption = null;
  let invalidRegionOption = null;
  for (const child of region.children) {
    if (child.tagName === 'OPTION') {
      if (child.value === '' && !child.hasAttribute('data-p10-invalid') && emptyRegionOption === null) {
        emptyRegionOption = child.cloneNode(true);
        continue;
      }
      if (child.value !== '' && child.hasAttribute('data-p10-invalid') && invalidRegionOption === null && child.selected) {
        invalidRegionOption = child;
        continue;
      }
      return null;
    }
    if (child.tagName !== 'OPTGROUP') return null;
    const code = child.getAttribute('data-p10-country');
    const options = Array.from(child.children);
    const values = new Set();
    for (const option of options) {
      if (option.tagName !== 'OPTION' || option.value === '' || option.hasAttribute('data-p10-invalid') || values.has(option.value)) return null;
      values.add(option.value);
    }
    if (!code || groups.has(code) || values.size === 0 || !countries.has(code)) return null;
    groups.set(code, { node: child.cloneNode(true), regions: values });
  }
  if (!emptyRegionOption || groups.size !== countries.size ||
      Array.from(countries).some((code) => !groups.has(code)) ||
      invalidRegionOption && region.value !== invalidRegionOption.value ||
      !groups.has(config.initial.country) ||
      !groups.get(config.initial.country).regions.has(config.initial.region) ||
      !currencies.has(config.initial.currency)) return null;

  const actionButtons = Array.from(form.querySelectorAll('button[name="_action"]'));
  const resetButtons = actionButtons.filter(isVisibleReset);
  const validateButtons = actionButtons.filter((button) => button.tagName === 'BUTTON' && button.type === 'submit' &&
    button.value === 'validate' && button.form === form);
  if (actionButtons.length !== 2 || resetButtons.length !== 1 || validateButtons.length !== 1 || form.method !== 'post') return null;
  const resetButton = resetButtons[0];
  const resetID = `${prefix}-reset-form`;
  const resetTransport = ids.get(resetID);
  if (resetButton.getAttribute('form') !== resetID || resetButton.form !== resetTransport || !resetTransport ||
      resetTransport.tagName !== 'FORM' || resetTransport.parentElement !== form.parentElement ||
      !resetTransport.hasAttribute('data-p10-reset') || !resetTransport.hidden || resetTransport.method !== 'post' ||
      resetButton.getAttribute('formaction') !== resetTransport.getAttribute('action')) return null;

  return { config, controls, count, status: statuses[0], errors, groups, emptyRegionOption, resetButton };
}

function isVisibleReset(button) {
  return button.tagName === 'BUTTON' && button.type === 'submit' && button.name === '_action' &&
    button.value === 'reset' && !button.hidden && !button.hasAttribute('hidden') &&
    button.getAttribute('aria-hidden') !== 'true';
}

export function mountSettings(form) {
  const existing = mounted.get(form);
  if (existing) return existing;
  const state = preflight(form);
  if (!state) return null;

  const { controls, count, status, errors, groups, emptyRegionOption, config, resetButton } = state;
  const country = controls.get('country[code]');
  const region = controls.get('region');
  const bio = controls.get('bio');
  const listeners = [];
  const on = (node, event, listener) => {
    node.addEventListener(event, listener);
    listeners.push([node, event, listener]);
  };
  const clearStatus = () => { status.textContent = ''; };
  const setError = (field, message) => {
    const error = errors.get(field);
    if (!error) return;
    error.textContent = message;
    error.hidden = !message;
    const control = controls.get(field === 'country' ? 'country[code]' : field);
    if (control) control.setAttribute('aria-invalid', message ? 'true' : 'false');
  };
  const clearError = (field) => setError(field, '');
  const updateBio = () => {
    const length = textareaCodePoints(bio.value);
    count.textContent = `${length} / ${BIO_LIMIT} Unicode code points`;
    setError('bio', length > BIO_LIMIT ? `must be at most ${BIO_LIMIT} Unicode code points` : '');
  };
  const rebuildRegions = (wantedCountry, wantedRegion, full) => {
    const previous = wantedRegion === region.value ? copyOption(region, wantedRegion, `${wantedRegion} (invalid)`) : null;
    while (region.firstChild) region.removeChild(region.firstChild);
    region.appendChild(emptyRegionOption.cloneNode(true));
    const canonical = retainedRegionAfterCountryChange(groups, wantedCountry, wantedRegion);
    if (wantedRegion && !canonical) region.appendChild(previous || copyOption(region, wantedRegion, `${wantedRegion} (invalid)`));
    const selectedGroups = full ? groups.values() : (groups.has(wantedCountry) ? [groups.get(wantedCountry)] : []);
    for (const group of selectedGroups) region.appendChild(group.node.cloneNode(true));
    region.value = wantedRegion;
  };
  const validateRegion = () => {
    if (!groups.has(country.value)) {
      setError('region', region.value ? 'cannot be used without a configured country' : '');
    } else if (!groups.get(country.value).regions.has(region.value)) {
      setError('region', 'must select a configured region');
    } else {
      clearError('region');
    }
  };
  const reset = () => {
    const initial = config.initial;
    for (const field of VALUE_FIELDS) controls.get(field).value = initial[field === 'country[code]' ? 'country' : field];
    controls.get('email_is_public').checked = initial.email_is_public;
    rebuildRegions(initial.country, initial.region, false);
    updateBio();
    for (const field of ERROR_FIELDS) clearError(field);
    clearStatus();
  };

  // Preserve the server's current draft, including a deliberate invalid region option.
  rebuildRegions(country.value, region.value, false);
  updateBio();
  for (const [field, control] of controls) {
    if (field === 'country[code]' || field === 'region' || field === 'email_is_public') continue;
    on(control, control.tagName === 'SELECT' ? 'change' : 'input', () => {
      clearStatus();
      clearError(field);
      if (field === 'bio') updateBio();
    });
  }
  on(controls.get('email_is_public'), 'change', clearStatus);
  on(country, 'change', () => {
    clearStatus();
    const next = retainedRegionAfterCountryChange(groups, country.value, region.value);
    rebuildRegions(country.value, next, false);
    setError('country', groups.has(country.value) ? '' : 'must select a configured country');
    validateRegion();
  });
  on(region, 'change', () => { clearStatus(); validateRegion(); });
  on(resetButton, 'click', (event) => { event.preventDefault(); reset(); });

  form.setAttribute('data-p10-enhanced', 'true');
  const handle = {
    dispose() {
      if (mounted.get(form) !== handle) return;
      for (const [node, event, listener] of listeners) node.removeEventListener(event, listener);
      // Keep the select node (and therefore focus) in place while restoring native fallback choices.
      rebuildRegions(country.value, region.value, true);
      form.removeAttribute('data-p10-enhanced');
      mounted.delete(form);
    },
  };
  mounted.set(form, handle);
  return handle;
}

export function mountAll(root) {
  const target = root || (typeof document === 'undefined' ? null : document);
  if (!target || typeof target.querySelectorAll !== 'function') return [];
  const forms = [];
  if (target.nodeType === 1 && target.tagName === 'FORM' && target.hasAttribute('data-p10-settings')) forms.push(target);
  forms.push(...target.querySelectorAll('form[data-p10-settings]'));
  return forms.map((form) => mountSettings(form)).filter(Boolean);
}

function autoMount() {
  if (typeof document === 'undefined' || !document.body || !document.body.hasAttribute('data-p10-enhance')) return;
  mountAll(document);
}

// Importing this module in Node is safe: global document is read only after a typeof guard.
if (typeof document !== 'undefined') {
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', autoMount, { once: true });
  else autoMount();
}
