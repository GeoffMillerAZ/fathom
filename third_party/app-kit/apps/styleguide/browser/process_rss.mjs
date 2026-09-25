import {execFile} from 'node:child_process';
import {promisify} from 'node:util';

const exec = promisify(execFile);
const ROW = /^\s*(\d+)\s+(\d+)\s+(\d+)\s+((?:Sun|Mon|Tue|Wed|Thu|Fri|Sat)\s+(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+(?:0?[1-9]|[12]\d|3[01])\s+(?:[01]\d|2[0-3]):[0-5]\d:[0-5]\d\s+\d{4})\s+(.+?)\s*$/;

export function parseProcessRows(raw) {
  if (typeof raw !== 'string') throw new TypeError('ps output must be text');
  const rows = [], seen = new Set(), lines = raw.split(/\r?\n/);
  for (const [index, line] of lines.entries()) {
    if (!line.trim()) { if (index === lines.length - 1) continue; throw new Error(`blank ps row ${index + 1}`); }
    const match = ROW.exec(line);
    if (!match) throw new Error(`malformed ps row ${index + 1}`);
    const pid = Number(match[1]), ppid = Number(match[2]), rss_kib = Number(match[3]);
    if (!Number.isSafeInteger(pid) || pid <= 0) throw new Error(`invalid pid on row ${index + 1}`);
    if (!Number.isSafeInteger(ppid) || ppid < 0) throw new Error(`invalid ppid on row ${index + 1}`);
    if (!Number.isSafeInteger(rss_kib) || rss_kib < 0) throw new Error(`invalid rss on row ${index + 1}`);
    if (seen.has(pid)) throw new Error(`duplicate pid ${pid}`);
    seen.add(pid); rows.push({pid, ppid, rss_kib, start_identity: match[4].split(/\s+/).join(' '), command: match[5]});
  }
  return rows;
}

async function defaultReadPS(pids) {
  const result = await exec('/bin/ps', ['-p', pids.join(','), '-o', 'pid=,ppid=,rss=,lstart=,command='], {env: {LC_ALL: 'C', PATH: '/usr/bin:/bin'}, timeout: 1000, maxBuffer: 1 << 20});
  return result.stdout;
}

export async function sampleOwnedProcesses({send, browserPid, serverPid, now = () => performance.now(), readPS = defaultReadPS}) {
  const started_ms = now(), errors = [], result = {started_ms, completed_ms: null, process_info: [], raw_ps: '', rows: [], errors};
  const fail = message => errors.push({message: String(message), observed_at: now()});
  if (!Number.isSafeInteger(browserPid) || browserPid <= 0) fail('browserPid must be a positive integer');
  if (!Number.isSafeInteger(serverPid) || serverPid <= 0) fail('serverPid must be a positive integer');
  if (browserPid === serverPid) fail('browserPid and serverPid must differ');
  if (!errors.length) try {
    const response = await send('SystemInfo.getProcessInfo');
    result.process_info = response?.processInfo ?? [];
    if (!Array.isArray(result.process_info)) throw new Error('processInfo is not an array');
    const listed = [], seen = new Set();
    for (const [index, entry] of result.process_info.entries()) { if (typeof entry?.id !== 'number') throw new Error(`processInfo entry ${index + 1} has non-numeric id`); const pid = entry.id; if (!Number.isSafeInteger(pid) || pid <= 0) throw new Error(`processInfo entry ${index + 1} has invalid id`); if (seen.has(pid)) throw new Error(`processInfo duplicate pid ${pid}`); seen.add(pid); listed.push(pid); }
    const browser = result.process_info.filter(entry => entry?.type === 'browser');
    if (seen.has(serverPid)) throw new Error('server PID is reported as a browser process');
    if (browser.length !== 1 || Number(browser[0]?.id) !== browserPid) throw new Error('browser process entry does not match owner PID');
    if (!seen.has(serverPid)) listed.push(serverPid);
    const raw = await readPS([...new Set(listed)]); result.raw_ps = raw; result.rows = parseProcessRows(raw);
    const rowPids = new Set(result.rows.map(row => row.pid)); for (const pid of listed) if (!rowPids.has(pid)) fail(`ps output missing owned PID ${pid}`); for (const pid of rowPids) if (!listed.includes(pid)) fail(`ps output contains unrequested PID ${pid}`);
  } catch (error) { fail(error); }
  result.completed_ms = now(); return result;
}
