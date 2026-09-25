import test from 'node:test';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {parseArgs, validateReceipt, allowedRequest, sandboxCanaries, pngDimensions, safeFailure, submit} from './run.mjs';

test('arguments require explicit pinned receipt and private output', () => {
  const args = ['--receipt', '/tmp/receipt', '--receipt-sha256', 'a'.repeat(64), '--playwright', '/tmp/index.mjs', '--out', '/tmp/output'];
  assert.equal(parseArgs(args)['--out'], '/tmp/output');
  for (const bad of [[], args.slice(0, -1), [...args, '--out', '/tmp/other'], args.map(value => value === '/tmp/output' ? 'relative' : value), args.map(value => value === 'a'.repeat(64) ? 'latest' : value)]) assert.throws(() => parseArgs(bad));
});

test('only a complete exact receipt can select the server', () => {
  const r = {kind: 'app-kit-private-gallery-host-rehearsal-v1', source_unchanged: true, identical_server: true, identical_native: true,
    candidate: {version: 'v0.0.0-rehearsal.h'+'a'.repeat(64), files: [{path:'apps/gallery/ui/app.css', sha256:'b'.repeat(64)}]},
    runs: [1,2].map(i => ({execution:{name:`host-run-${i}`, binary_sha256:'c'.repeat(64)}, native_sha256:'d'.repeat(64)}))};
  const verify = value => { const body=Buffer.from(JSON.stringify(value)); return validateReceipt(body, createHash('sha256').update(body).digest('hex')); };
  assert.equal(verify(r).runs.length, 2);
  for (const bad of [{...r, error:'failure'}, {...r, source_unchanged:false}, {...r, identical_server:false}, {...r, runs:r.runs.slice(0,1)}, {...r, candidate:{...r.candidate, files:[]}}, {...r, runs:[r.runs[0], {...r.runs[1], execution:{...r.runs[1].execution,binary_sha256:'e'.repeat(64)}}]}]) assert.throws(() => verify(bad));
  assert.throws(() => validateReceipt(Buffer.from(JSON.stringify(r)), '0'.repeat(64)));
});

test('browser allowlist never admits external sources or extra launch parameters', () => {
  const origin='http://127.0.0.1:19876'; const launch=origin+'/auth/launch?token='+'a'.repeat(43);
  assert.equal(allowedRequest(origin,'GET',launch,launch),true);
  assert.equal(allowedRequest(origin,'GET',origin+'/shell/profile',launch),true);
  assert.equal(allowedRequest(origin,'POST',origin+'/shell/action',launch),true);
  for(const [method,url] of [['GET','https://example.org/shell/profile'],['GET',launch+'&extra=1'],['GET',origin+'/references'],['GET',origin+'/shell/profile?extra=1'],['POST',origin+'/shell/profile'],['DELETE',origin+'/shell/action']]) assert.equal(allowedRequest(origin,method,url,launch),false);
  assert.equal(allowedRequest(origin,'GET',launch,''),false);
});

test('canaries use private cwd and explicit environment, refusing failed isolation', () => {
  let calls = 0;
  const fake = (executable, args, options) => {
    calls++;
    assert.equal(executable, '/usr/bin/sandbox-exec');
    assert.equal(options.cwd, '/private/test-root');
    assert.deepEqual(options.env, {PATH: '/usr/bin:/bin'});
    assert.equal(options.timeout, 5000);
    assert.match(args[1], /deny network\*/);
    return args[2] === '/usr/bin/head' ? {status: 1, stderr: 'Operation not permitted'} : {status: 0};
  };
  sandboxCanaries('/private/test-root', fake);
  assert.equal(calls, 2);
  assert.throws(() => sandboxCanaries('/private/test-root', () => ({status: 0})), /source-denial-canary/);
  assert.throws(() => sandboxCanaries('/private/test-root', (_executable, args) => args[2] === '/usr/bin/head' ? {status: 1, stderr: 'Operation not permitted'} : {status: 1}), /network-denial-canary/);
});

test('capture dimensions reject missing, malformed, empty or unbounded headers', () => {
  const png = Buffer.alloc(33);
  Buffer.from([137,80,78,71,13,10,26,10]).copy(png);
  png.writeUInt32BE(13, 8); png.write('IHDR', 12); png.writeUInt32BE(390, 16); png.writeUInt32BE(1000, 20);
  assert.deepEqual(pngDimensions(png), {width: 390, height: 1000});
  assert.throws(() => pngDimensions(png.subarray(0, 32)));
  for (const [offset, value] of [[0, 0], [8, 12], [12, 0], [16, 0], [20, 20001]]) {
    const bad = Buffer.from(png); bad.writeUInt32BE(value, offset);
    assert.throws(() => pngDimensions(bad));
  }
});

test('navigation diagnostics preserve fixed failure categories, never URLs', () => {
  assert.equal(safeFailure(new Error('page.goto: net::ERR_CONNECTION_REFUSED at http://127.0.0.1:1234/auth/launch?token=secret')), 'err-connection-refused');
  assert.equal(safeFailure(new Error('Target page, context or browser has been closed: secret')), 'browser-target-closed');
  assert.equal(safeFailure(new Error('secret http://127.0.0.1:1234/auth/launch?token=secret')), 'execution-failed');
  assert.equal(safeFailure({name:'TimeoutError', message:'secret'}), 'timeout');
});

test('form action waits for its armed navigation, not the previous loaded document', async () => {
  const events = [];
  let navigationDone;
  const navigation = new Promise(resolve => { navigationDone = resolve; });
  const page = {
    waitForResponse() { events.push('response-armed'); return Promise.resolve({status: () => 303}); },
    waitForNavigation(options) { events.push('navigation-armed'); assert.deepEqual(options, {waitUntil:'load'}); return navigation; },
    locator() { return {click: async () => { events.push('click'); }}; },
    waitForLoadState() { assert.fail('old document load must not be used'); },
  };
  let finished = false;
  const pending = submit(page, 'button', 303, ok => { assert(ok); finished = true; }, 'saved', false);
  await new Promise(setImmediate);
  assert.deepEqual(events, ['response-armed', 'navigation-armed', 'click']);
  assert.equal(finished, false);
  navigationDone(); await pending; assert.equal(finished, true);
});
