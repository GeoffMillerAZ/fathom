import fs from 'node:fs';
import path from 'node:path';
import {randomBytes} from 'node:crypto';
import {pathToFileURL} from 'node:url';

const launchPattern = /^http:\/\/127\.0\.0\.1:([1-9][0-9]{0,4})\/auth\/launch\?token=([A-Za-z0-9_-]{43})$/;

function refusal(kind) {
  return new Error(`browser handoff refused: ${kind}`);
}

export function validateLaunchURL(value) {
  if (typeof value !== 'string') throw refusal('invalid launch URL');
  const match = launchPattern.exec(value);
  if (!match) throw refusal('invalid launch URL');
  const port = Number(match[1]);
  if (!Number.isSafeInteger(port) || port < 1 || port > 65535) {
    throw refusal('invalid launch URL');
  }

  // The regular expression is the canonical spelling contract. URL parsing is
  // retained as an independent structural check before any value is written.
  const parsed = new URL(value);
  if (parsed.protocol !== 'http:' || parsed.hostname !== '127.0.0.1' ||
      parsed.username !== '' || parsed.password !== '' || parsed.hash !== '' ||
      parsed.pathname !== '/auth/launch' || parsed.searchParams.size !== 1 ||
      parsed.searchParams.get('token') !== match[2]) {
    throw refusal('invalid launch URL');
  }
  return {origin: `http://127.0.0.1:${match[1]}`, launchURL: value};
}

export function validateHandoffPath(value, fsAPI = fs) {
  if (typeof value !== 'string' || value === '' || !path.isAbsolute(value) ||
      path.resolve(value) !== value) {
    throw refusal('invalid output authority');
  }
  const parent = path.dirname(value);
  let parentInfo;
  try {
    if (fsAPI.realpathSync(parent) !== parent) throw refusal('invalid output authority');
    parentInfo = fsAPI.lstatSync(parent);
  } catch (error) {
    if (error?.message?.startsWith('browser handoff refused:')) throw error;
    throw refusal('invalid output authority');
  }
  const uid = typeof process.getuid === 'function' ? process.getuid() : undefined;
  if (!parentInfo.isDirectory() || parentInfo.isSymbolicLink() ||
      (parentInfo.mode & 0o777) !== 0o700 || uid === undefined || parentInfo.uid !== uid) {
    throw refusal('invalid output authority');
  }
  try {
    fsAPI.lstatSync(value);
  } catch (error) {
    if (error?.code === 'ENOENT') return value;
    throw refusal('invalid output authority');
  }
  throw refusal('invalid output authority');
}

function writeRecord(launchURL, outputPath, fsAPI = fs) {
  const record = validateLaunchURL(launchURL);
  const destination = validateHandoffPath(outputPath, fsAPI);
  const body = `${JSON.stringify(record)}\n`;
  if (Buffer.byteLength(body) > 512) throw refusal('output write failed');
  let descriptor;
  let staging;
  let staged = false;
  try {
    staging = path.join(path.dirname(destination),
      `.${path.basename(destination)}.staging-${process.pid}-${randomBytes(12).toString('hex')}`);
    descriptor = fsAPI.openSync(staging, 'wx', 0o600);
    staged = true;
    fsAPI.writeFileSync(descriptor, body, {encoding: 'utf8'});
    fsAPI.fsyncSync(descriptor);
    const info = fsAPI.fstatSync(descriptor);
    if (!info.isFile() || (info.mode & 0o777) !== 0o600 || info.size !== Buffer.byteLength(body)) {
      throw refusal('output verification failed');
    }
    fsAPI.closeSync(descriptor);
    descriptor = undefined;
    // link(2) publishes the completed inode atomically and refuses an existing
    // destination. The consumer may unlink it immediately after this call.
    fsAPI.linkSync(staging, destination);
    fsAPI.unlinkSync(staging);
    staged = false;
    return;
  } catch (error) {
    if (descriptor !== undefined) {
      try { fsAPI.closeSync(descriptor); } catch {}
    }
    if (staged && staging) {
      try { fsAPI.unlinkSync(staging); } catch {}
    }
    if (error?.message?.startsWith('browser handoff refused:')) throw error;
    throw refusal('output write failed');
  }
}

export function writeHandoff(args, environment = process.env, fsAPI = fs) {
  if (!Array.isArray(args) || args.length !== 1) throw refusal('wrong argument count');
  const outputPath = environment.APPKIT_BROWSER_HANDOFF;
  writeRecord(args[0], outputPath, fsAPI);
}

export function runCLI(args, environment = process.env, fsAPI = fs, stderr = process.stderr) {
  try {
    writeHandoff(args, environment, fsAPI);
    return 0;
  } catch {
    stderr.write('browser handoff refused\n');
    return 1;
  }
}

const isCLI = process.argv[1] && pathToFileURL(path.resolve(process.argv[1])).href === import.meta.url;
if (isCLI) {
  process.exitCode = runCLI(process.argv.slice(2));
}
