'use strict';

// Chromium DevTools Protocol over --remote-debugging-pipe. Chromium uses
// fd 3 for incoming NUL-delimited JSON and fd 4 for outgoing frames.
const { spawn } = require('node:child_process');
const { EventEmitter } = require('node:events');
const fs = require('node:fs');
const path = require('node:path');

const MAX_FRAME_BYTES = 64 * 1024 * 1024;

class CDPConnection extends EventEmitter {
  constructor({ write, read, child = null, timeoutMs = 30000 }) {
    super();
    if (!write || !read || !Number.isFinite(timeoutMs) || timeoutMs <= 0) {
      throw new TypeError('CDP requires writable fd 3, readable fd 4, and a positive timeout');
    }
    this.write = write;
    this.read = read;
    this.child = child;
    this.timeoutMs = timeoutMs;
    this.nextId = 1;
    this.pending = new Map();
    this.buffer = Buffer.alloc(0);
    this.closed = false;
    this.read.on('data', chunk => this.#consume(chunk));
    this.read.on('error', error => this.#fail(error));
    this.read.on('end', () => this.#fail(new Error('CDP pipe closed')));
    this.write.on('error', error => this.#fail(error));
    if (child) {
      child.on('error', error => this.#fail(error));
      child.on('exit', (code, signal) => this.#fail(new Error(`browser exited: code=${code}, signal=${signal}`)));
    }
  }

  #fail(error) {
    if (this.closed) return;
    this.closed = true;
    for (const { reject, timer } of this.pending.values()) {
      clearTimeout(timer);
      reject(error);
    }
    this.pending.clear();
    this.emit('disconnect', error);
  }

  #consume(chunk) {
    if (this.closed) return;
    this.buffer = Buffer.concat([this.buffer, chunk]);
    if (this.buffer.length > MAX_FRAME_BYTES) {
      this.#fail(new Error('CDP frame exceeds 64 MiB'));
      return;
    }
    for (;;) {
      const end = this.buffer.indexOf(0);
      if (end < 0) return;
      const frame = this.buffer.subarray(0, end);
      this.buffer = this.buffer.subarray(end + 1);
      if (!frame.length) continue;
      let message;
      try {
        message = JSON.parse(frame.toString('utf8'));
      } catch (error) {
        this.#fail(new Error(`invalid CDP JSON frame: ${error.message}`));
        return;
      }
      if (message.id === undefined) {
        this.emit('event', message);
        if (message.method) this.emit(message.method, message);
        continue;
      }
      const request = this.pending.get(message.id);
      if (!request) continue;
      this.pending.delete(message.id);
      clearTimeout(request.timer);
      if (message.error) {
        request.reject(new Error(`${request.method}: ${message.error.message || 'CDP error'} (${message.error.code ?? 'unknown'})`));
      } else {
        request.resolve(message.result || {});
      }
    }
  }

  send(method, params = {}, sessionId = undefined) {
    if (this.closed) return Promise.reject(new Error('CDP connection closed'));
    if (typeof method !== 'string' || !method) return Promise.reject(new TypeError('CDP method required'));
    const id = this.nextId++;
    const message = { id, method, params };
    if (sessionId !== undefined) message.sessionId = sessionId;
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error(`${method} timed out after ${this.timeoutMs} ms`));
      }, this.timeoutMs);
      this.pending.set(id, { method, resolve, reject, timer });
      try {
        this.write.write(JSON.stringify(message) + '\0');
      } catch (error) {
        clearTimeout(timer);
        this.pending.delete(id);
        reject(error);
      }
    });
  }

  async createTargetSession(url = 'about:blank') {
    const { targetId } = await this.send('Target.createTarget', { url });
    if (!targetId) throw new Error('Target.createTarget returned no targetId');
    const { sessionId } = await this.send('Target.attachToTarget', { targetId, flatten: true });
    if (!sessionId) throw new Error('Target.attachToTarget returned no sessionId');
    return { targetId, sessionId, send: (method, params = {}) => this.send(method, params, sessionId) };
  }

  async close({ graceMs = 2000, killWaitMs = 2000 } = {}) {
    if (!Number.isFinite(graceMs) || graceMs <= 0 || !Number.isFinite(killWaitMs) || killWaitMs <= 0) {
      throw new TypeError('close requires positive grace and kill wait times');
    }
    if (!this.closed) this.#fail(new Error('CDP connection closed'));
    const child = this.child;
    if (!child || child.exitCode != null || child.signalCode) return;
    await new Promise((resolve, reject) => {
      let done = false;
      let graceTimer;
      let killTimer;
      const finish = error => {
        if (done) return;
        done = true;
        clearTimeout(graceTimer);
        clearTimeout(killTimer);
        child.removeListener('exit', onExit);
        child.removeListener('error', onError);
        if (error) reject(error);
        else resolve();
      };
      const onExit = () => finish();
      const onError = error => finish(error);
      child.once('exit', onExit);
      child.once('error', onError);
      graceTimer = setTimeout(() => {
        if (child.exitCode != null || child.signalCode) return;
        try {
          child.kill('SIGKILL');
        } catch (error) {
          finish(error);
          return;
        }
        killTimer = setTimeout(() => finish(new Error('browser did not exit after SIGKILL')), killWaitMs);
      }, graceMs);
      try {
        child.kill('SIGTERM');
      } catch (error) {
        finish(error);
      }
    });
  }
}

function startBrowser(binary, { profileDir, timeoutMs = 30000, spawnImpl = spawn } = {}) {
  if (!binary || !profileDir) throw new TypeError('binary and caller-owned profileDir are required');
  const profile = path.resolve(profileDir);
  fs.mkdirSync(profile, { recursive: true });
  const args = [
    '--remote-debugging-pipe',
    '--no-first-run',
    '--no-default-browser-check',
    '--disable-background-networking',
    '--disable-component-update',
    '--disable-sync',
    '--disable-default-apps',
    '--disable-extensions',
    `--user-data-dir=${profile}`,
    'about:blank',
  ];
  const child = spawnImpl(binary, args, { stdio: ['ignore', 'ignore', 'pipe', 'pipe', 'pipe'] });
  if (child.stdio && child.stdio[2]) child.stdio[2].resume();
  if (!child.stdio || !child.stdio[3] || !child.stdio[4]) {
    child.kill();
    throw new Error('browser did not expose CDP fd 3 and fd 4');
  }
  return new CDPConnection({ write: child.stdio[3], read: child.stdio[4], child, timeoutMs });
}

module.exports = { CDPConnection, startBrowser };
