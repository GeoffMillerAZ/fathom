const keys = ['bytes', 'path', 'scope', 'sha256'];
const digest = /^[0-9a-f]{64}$/;

function canonical(records, name) {
  if (!Array.isArray(records)) throw new TypeError(`${name} must be an array`);
  const seen = new Set();
  const copy = records.map((record, index) => {
    if (record === null || typeof record !== 'object' || Array.isArray(record)) {
      throw new TypeError(`${name}[${index}] must be a record`);
    }
    const actualKeys = Object.keys(record).sort();
    if (JSON.stringify(actualKeys) !== JSON.stringify(keys)) {
      throw new TypeError(`${name}[${index}] must contain exactly scope, path, bytes and sha256`);
    }
    if (typeof record.scope !== 'string' || record.scope.length === 0 || record.scope.includes('\0') ||
        typeof record.path !== 'string' || record.path.length === 0 || record.path.includes('\0') ||
        !Number.isSafeInteger(record.bytes) || record.bytes < 0 ||
        typeof record.sha256 !== 'string' || !digest.test(record.sha256)) {
      throw new TypeError(`${name}[${index}] has invalid field values`);
    }
    const key = `${record.scope}\0${record.path}`;
    if (seen.has(key)) throw new Error(`${name} contains duplicate inventory key ${record.scope}:${record.path}`);
    seen.add(key);
    return {scope: record.scope, path: record.path, bytes: record.bytes, sha256: record.sha256};
  });
  return copy.sort((a, b) => a.scope < b.scope ? -1 : a.scope > b.scope ? 1 : a.path < b.path ? -1 : a.path > b.path ? 1 : 0);
}

export function assertInventoryEqual(actual, expected) {
  const left = canonical(actual, 'actual');
  const right = canonical(expected, 'expected');
  if (JSON.stringify(left) !== JSON.stringify(right)) throw new Error('inventory records differ');
  return true;
}
