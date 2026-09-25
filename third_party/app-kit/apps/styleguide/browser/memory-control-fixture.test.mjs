import assert from 'node:assert/strict';
import test from 'node:test';
import {createControlServer} from './memory-control-fixture.mjs';

async function fixture(mode) {
  const server = createControlServer({mode});
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const origin = `http://127.0.0.1:${server.address().port}`;
  return {server, origin, close: () => new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve()))};
}

test('requires one exact control mode and returns an unlistened server', () => {
  for (const value of [undefined, null, '', 'memory', 'Positive']) {
    assert.throws(() => createControlServer(value === undefined ? undefined : {mode: value}), /mode must be exactly/);
  }
  const server = createControlServer({mode: 'positive'});
  assert.equal(server.listening, false);
});

for (const mode of ['positive', 'retention']) test(`${mode} serves only the fixed no-store routes`, async t => {
  const f = await fixture(mode); t.after(f.close);
  for (const pathname of ['/theme-lab', '/theme-lab/preview', '/control.js']) {
    const response = await fetch(f.origin + pathname);
    assert.equal(response.status, 200);
    assert.equal(response.headers.get('cache-control'), 'no-store');
    assert.equal(response.headers.get('x-content-type-options'), 'nosniff');
    await response.arrayBuffer();
  }
  const missing = await fetch(f.origin + '/other');
  assert.equal(missing.status, 404);
  assert.equal((await fetch(f.origin + '/theme-lab?unexpected=1')).status, 404);
  const refused = await fetch(f.origin + '/theme-lab', {method: 'POST'});
  assert.equal(refused.status, 405);
  assert.equal(refused.headers.get('allow'), 'GET');
});

test('parent uses a native targeted form and an empty-sandbox preview', async t => {
  const f = await fixture('positive'); t.after(f.close);
  const response = await fetch(f.origin + '/theme-lab'), source = await response.text();
  assert.match(source, /<form action="\/theme-lab\/preview" method="get" target="preview">/);
  assert.match(source, /<button id="next" type="submit">/);
  assert.match(source, /<iframe id="preview" name="preview" sandbox="" src="\/theme-lab\/preview"/);
  assert.doesNotMatch(source, /id="allocate"|\/control\.js/);
  const csp = response.headers.get('content-security-policy');
  for (const directive of ["default-src 'none'", "script-src 'self'", "frame-src 'self'", "form-action 'self'"]) assert.ok(csp.includes(directive));
});

test('preview generations advance visibly and remain script-free', async t => {
  const f = await fixture('positive'); t.after(f.close);
  const first = await fetch(f.origin + '/theme-lab/preview'), firstHTML = await first.text();
  const second = await fetch(f.origin + '/theme-lab/preview'), secondHTML = await second.text();
  assert.match(firstHTML, /<h1 id="generation">Generation 1<\/h1>/);
  assert.match(secondHTML, /<h1 id="generation">Generation 2<\/h1>/);
  assert.ok(first.headers.get('content-security-policy').includes("script-src 'none'"));
  assert.doesNotMatch(firstHTML + secondHTML, /<script\b/i);
});

test('retention mode exposes an idempotent touched 8 MiB parent allocation', async t => {
  const f = await fixture('retention'); t.after(f.close);
  const page = await (await fetch(f.origin + '/theme-lab')).text();
  assert.match(page, /<button id="allocate" type="button">/);
  assert.match(page, /<p id="allocation-state" role="status">No retained allocation<\/p>/);
  assert.match(page, /<script src="\/control\.js"><\/script>/);
  const scriptResponse = await fetch(f.origin + '/control.js'), source = await scriptResponse.text();
  assert.match(source, /globalThis\.__appkitRetainedAllocation instanceof Uint8Array/);
  assert.match(source, /new Uint8Array\(8 \* 1024 \* 1024\)/);
  assert.match(source, /\.fill\(0x5a\)/);
  assert.match(source, /\.byteLength \+ ' bytes'/);
  assert.equal(scriptResponse.headers.get('content-type'), 'text/javascript; charset=utf-8');
});
