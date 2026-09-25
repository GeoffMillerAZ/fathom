// First-party P12 specimen contract. No renderer or paid source dependency.
export type Context = Readonly<{
  version: 1;
  instanceId: string;
  label: string;
  hint: string;
}>;
export type Initial = Context & Readonly<{ initialText: string }>;

const encoder = new TextEncoder();

function record(value: unknown, fields: readonly string[]): Record<string, unknown> {
  if (!value || typeof value !== 'object' || ![Object.prototype, null].includes(Object.getPrototypeOf(value))) {
    throw new TypeError('Expected a plain protocol record');
  }
  const keys = Reflect.ownKeys(value);
  if (keys.length !== fields.length || keys.some(key => typeof key !== 'string' || !fields.includes(key))) {
    throw new TypeError('Missing or unknown protocol field');
  }
  const result: Record<string, unknown> = Object.create(null);
  for (const field of fields) {
    const descriptor = Object.getOwnPropertyDescriptor(value, field);
    if (!descriptor || !('value' in descriptor)) throw new TypeError('Protocol accessors are not supported');
    result[field] = descriptor.value;
  }
  return result;
}

function text(value: unknown, name: string, limit: number, nonempty = false): string {
  if (typeof value !== 'string' || (nonempty && value.length === 0)) throw new TypeError(`Invalid ${name}`);
  // UTF-8 bytes cannot be fewer than UTF-16 code units: bound work before encoding.
  if (value.length > limit) throw new RangeError(`${name} exceeds ${limit} UTF-8 bytes`);
  // TextEncoder replaces lone UTF-16 surrogates; reject before counting rather than changing input.
  for (let i = 0; i < value.length; i++) {
    const code = value.charCodeAt(i);
    if (code >= 0xd800 && code <= 0xdbff) {
      const low = value.charCodeAt(++i);
      if (!(low >= 0xdc00 && low <= 0xdfff)) throw new TypeError(`Invalid Unicode in ${name}`);
    } else if (code >= 0xdc00 && code <= 0xdfff) throw new TypeError(`Invalid Unicode in ${name}`);
  }
  if (encoder.encode(value).byteLength > limit) throw new RangeError(`${name} exceeds ${limit} UTF-8 bytes`);
  return value;
}

function context(value: Record<string, unknown>): Context {
  if (value.version !== 1) throw new TypeError('Unsupported protocol version');
  const instanceId = text(value.instanceId, 'instanceId', 48, true);
  if (!/^[A-Za-z]/.test(instanceId) || /[^A-Za-z0-9-]/.test(instanceId)) throw new TypeError('Invalid instanceId');
  return {
    version: 1, instanceId,
    label: text(value.label, 'label', 512, true),
    hint: text(value.hint, 'hint', 512),
  };
}

export function parseInitial(value: unknown): Initial {
  const source = record(value, ['version', 'instanceId', 'label', 'hint', 'initialText']);
  return Object.freeze({ ...context(source), initialText: text(source.initialText, 'initialText', 65536) });
}

export function parseContext(value: unknown, expectedInstanceID: string): Context {
  const parsed = context(record(value, ['version', 'instanceId', 'label', 'hint']));
  if (parsed.instanceId !== expectedInstanceID) throw new TypeError('Context instanceId mismatch');
  return Object.freeze(parsed);
}
