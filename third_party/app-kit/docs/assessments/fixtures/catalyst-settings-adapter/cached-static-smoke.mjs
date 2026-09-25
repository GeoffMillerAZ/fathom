// Separate opt-in delivery evidence, not the default 360-sample acquisition.
// Uses already-running, explicitly identified loopback listeners and paid inputs
// only in their private store. No installation, source mutation, or saved bodies.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import fs from 'node:fs';
import http from 'node:http';
import path from 'node:path';
import {fileURLToPath, pathToFileURL} from 'node:url';
import {exactLoopbackOrigin} from './payload-checks.mjs';
import {approvedPlaywright, attestServer, fileIdentity, hardware, oneObservation, privatePacket, processIdentity, put} from './payload-browser.mjs';
import {CACHED_STATIC_POLICY_ID, readCachedStaticPacket} from '../catalyst-native/cached-static-reference-server.mjs';
import {auditSampleAssets, auditStaticResponse} from './cached-static-checks.mjs';
import {deriveFontAliases} from './font-query-aliases.mjs';
import {auditWarmFontIdentity, WARM_IDENTITY_ID} from './warm-font-identity.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const CHROME_VERSION = '152.0.7977.84';
const SOURCE_ROOT = '/tmp/appkit-catalyst-hardened.DdYTWU';
const profiles = [
  {id: 'source', route: '/settings', javaScriptEnabled: true, accounting: 'source-full-react-page-with-shell'},
  {id: 'native', route: '/dark/settings', javaScriptEnabled: false, accounting: 'target-go-form-only'},
  {id: 'enhanced', route: '/dark/enhanced/settings', javaScriptEnabled: true, accounting: 'target-go-form-only-plus-local-controller'},
];

// Raw HTTP: no automatic redirects/decompression/cookie jar. URLs are caller-owned
// exact paths from the pinned packet or synthetic refusal probes, never user input.
export function requestStatic(origin, requestPath, {method = 'GET', headers = {}} = {}) {
  const selected = new URL(exactLoopbackOrigin(origin));
  assert.ok(['GET', 'HEAD', 'POST'].includes(method), 'GET/HEAD or empty static-URL POST refusal probe only');
  assert.ok(requestPath.startsWith('/') && !requestPath.startsWith('//') && !/[\r\n]/.test(requestPath));
  return new Promise((resolve, reject) => {
    const chunks = []; let count = 0;
    const request = http.request({hostname: selected.hostname, port: selected.port, path: requestPath, method, headers, agent: false}, response => {
      response.on('data', bytes => {
        count += bytes.length;
        if (count > 16 * 1024 * 1024) { response.destroy(Error('bounded HTTP body exceeded')); return; }
        chunks.push(bytes);
      });
      response.on('error', reject);
      response.on('end', () => resolve({status: response.statusCode, headers: response.headers, body: Buffer.concat(chunks)}));
    });
    request.setTimeout(10_000, () => request.destroy(Error('bounded HTTP request timed out')));
    request.on('error', reject); request.end();
  });
}

export function auditDeliveryMetadata(headers, transport, selected) {
  assert.equal(headers['x-appkit-delivery-experiment'], selected ? transport.policy : undefined, 'response transport experiment scope/identity');
  assert.equal(headers['x-appkit-font-aliases-sha256'], selected ? transport.aliasSHA : undefined, 'response exact alias-set scope/identity');
  return {experiment: headers['x-appkit-delivery-experiment'] ?? null, alias_manifest_sha256: headers['x-appkit-font-aliases-sha256'] ?? null};
}

// Supplemental output has its own disposition. It never changes the sample,
// its strict failure, the run's stop-on-failure rule, or any byte/cache totals.
export function supplementalWarmResult(input) {
  try { return auditWarmFontIdentity(input); }
  catch (error) {
    return {schema_version: 1, id: WARM_IDENTITY_ID, status: 'unavailable', direct_warm_body_observed: false,
      strict_sample_ok: input.sample?.ok ?? null, reason: String(error).slice(0, 1024)};
  }
}

export function observePlaywrightWrapper(modulePath) {
  try {
    return {availability: 'observed', ...fileIdentity(path.resolve(path.dirname(modulePath), '../playwright-core/lib/coreBundle.js'), 'Playwright CDP error wrapper')};
  } catch (error) {
    // The supplemental proof may be unavailable; this is not a new prerequisite
    // for the strict delivery smoke or its cold observations.
    return {availability: 'unavailable', sha256: null, reason: String(error).slice(0, 1024)};
  }
}

async function assetHTTPChecks(origin, host, assets, rows, transport = null) {
  const attest = response => transport ? auditDeliveryMetadata(response.headers, transport, true) : null;
  for (const asset of assets) for (const encoding of ['identity', 'gzip']) {
    const headers = {'accept-encoding': encoding === 'gzip' ? 'gzip, identity;q=0' : 'identity, gzip;q=0'};
    const full = await requestStatic(origin, asset.url, {headers});
    const delivery = attest(full);
    const observed = auditStaticResponse({asset, method: 'GET', encoding, ...full});
    rows.push({host, url: asset.url, method: 'GET', encoding, conditional: false, observed, delivery});
    for (const [method, conditional] of [['HEAD', false], ['GET', true], ['HEAD', true]]) {
      const response = await requestStatic(origin, asset.url, {method, headers: {...headers, ...(conditional ? {'if-none-match': `W/${full.headers.etag}`} : {})}});
      const delivery = attest(response);
      const observed = auditStaticResponse({asset, method, encoding, conditional, ...response});
      for (const name of ['etag', 'cache-control', 'vary', 'content-encoding']) assert.equal(response.headers[name], full.headers[name], `${host} ${method} representation ${name}`);
      if (!conditional || response.headers['content-length'] !== undefined) assert.equal(response.headers['content-length'], full.headers['content-length'], 'selected representation length');
      rows.push({host, url: asset.url, method, encoding, conditional, observed, delivery});
    }
  }
}

async function exclusionHTTPChecks(origin, host, assets, rows, aliases = [], transport = null) {
  const css = assets.find(asset => asset.mime === 'text/css').url;
  const routeHeaders = ['rsc', 'next-router-prefetch', 'next-router-segment-prefetch', 'next-router-state-tree', 'next-url', 'next-action'];
  const cases = [
    {name: 'document', url: host === 'source' ? '/settings' : '/dark/settings'},
    {name: 'unknown', url: '/appkit-cached-static-missing.css', expectedStatus: 404},
    {name: 'query', url: `${css}?appkit_probe=1`},
    {name: 'empty-query', url: `${css}?`},
    {name: 'encoded-path', url: css.replace('/', '/%2e/')},
    {name: 'wrong-host', url: css, headers: {host: 'example.invalid'}, expectedStatus: host === 'source' ? 403 : 421},
    {name: 'unacceptable-encoding', url: css, headers: {'accept-encoding': 'identity;q=0, gzip;q=0'}, expectedStatus: 406},
    {name: 'static-post-refusal', url: css, method: 'POST', expectedStatus: 405},
    ...routeHeaders.map(name => ({name, url: css, headers: {[name]: '1'}})),
    ...(host === 'target' ? ['/native.css', '/fixture.css', '/controller.mjs', '/favicon.ico'].map(url => ({name: url, url})) : []),
    ...(aliases.length ? [
      ...['?v=4.2', '?V=4.1', '?v=4.1&extra=1', '?v=4.1&v=4.1', '?extra=1&v=4.1', '?v=%34.1', '?'].map(query => ({name: `font-near-miss-${query}`, url: aliases[0].canonical_url + query})),
      ...routeHeaders.map(name => ({name: `font-alias-${name}`, url: aliases[0].url, headers: {[name]: '1'}})),
      {name: 'font-alias-host-refusal', url: aliases[0].url, headers: {host: 'example.invalid'}, expectedStatus: host === 'source' ? 403 : 421},
      {name: 'font-alias-post-refusal', url: aliases[0].url, method: 'POST', expectedStatus: 405},
    ] : []),
  ];
  for (const fixture of cases) {
    const response = await requestStatic(origin, fixture.url, {method: fixture.method, headers: fixture.headers});
    assert.equal(response.headers['cache-control'], 'no-store', `${host} exclusion ${fixture.name}`);
    if (fixture.expectedStatus) assert.equal(response.status, fixture.expectedStatus, `${host} status ${fixture.name}`);
    // A selected static request with no acceptable representation remains a
    // tagged 406/no-store. Dynamic and unselected fallback traffic is untagged.
    const delivery = transport ? auditDeliveryMetadata(response.headers, transport, fixture.name === 'unacceptable-encoding') : null;
    rows.push({host, case: fixture.name, status: response.status, cache_control: response.headers['cache-control'], delivery});
  }
}

export function verifiedTargetManifest(binary) {
  const directory = path.dirname(binary);
  const manifest = fileIdentity(path.join(directory, 'SHA256SUMS'), 'target generated manifest');
  assert.ok(manifest.bytes < 16 * 1024, 'bounded target manifest');
  const lines = fs.readFileSync(manifest.path, 'utf8').trim().split('\n');
  assert.equal(lines.length, 16, 'fixed generated artifact count');
  const names = new Set(), files = [];
  for (const line of lines) {
    const match = /^([a-f0-9]{64})  ([A-Za-z0-9_./-]+)$/.exec(line);
    assert.ok(match, 'target manifest record');
    const [, expected, name] = match;
    assert.ok(!names.has(name) && !name.split('/').some(part => !part || part === '.' || part === '..'), 'unique clean relative artifact path');
    names.add(name);
    const observed = fileIdentity(path.join(directory, name), 'target generated artifact');
    assert.equal(observed.sha256, expected, `target artifact drift: ${name}`);
    files.push({name, sha256: observed.sha256, bytes: observed.bytes});
  }
  assert.ok(names.has(path.basename(binary)) && names.has('cmd/server/server.go'), 'binary and transport source manifest membership');
  return {...manifest, verified_files: files};
}

async function main() {
  const [sourceInput, sourceRoot, targetInput, targetBinary, sourcePacket, parent, sourcePID, targetPID] = process.argv.slice(2);
  const fontAliases = process.argv[10] === '--font-query-aliases';
  assert.equal(process.argv.length, fontAliases ? 11 : 10, 'Use cached-static-smoke.mjs SOURCE_ORIGIN SOURCE_ROOT TARGET_ORIGIN TARGET_BINARY SOURCE_PACKET PRIVATE_PARENT SOURCE_PID TARGET_PID [--font-query-aliases]');
  const policyID = fontAliases ? 'p10-cached-static-v2' : CACHED_STATIC_POLICY_ID;
  assert.equal(sourceRoot, SOURCE_ROOT, 'explicit selected hardened source');
  const sourceOrigin = exactLoopbackOrigin(sourceInput), targetOrigin = exactLoopbackOrigin(targetInput);
  assert.notEqual(sourceOrigin, targetOrigin);
  const sourceTransport = path.resolve(here, fontAliases ? '../catalyst-native/cached-static-font-alias-reference-server.mjs' : '../catalyst-native/cached-static-reference-server.mjs');
  const sourceProcess = attestServer(processIdentity(sourcePID, 'SOURCE'), sourceOrigin, {label: 'cached source', sourceCommandSuffix: ` ${sourceTransport} ${sourceRoot} ${sourcePacket}`});
  const targetProcess = attestServer(processIdentity(targetPID, 'TARGET'), targetOrigin, {label: 'cached target', binary: targetBinary});
  assert.equal(targetProcess.command, `${targetBinary} ${sourcePacket} --cached-static-delivery=${policyID}`, 'target transport selection');
  const packet = privatePacket(sourcePacket, parent, sourceRoot);
  const assets = readCachedStaticPacket(sourcePacket).assets;
  assert.equal(assets.length, 43);
  let aliasManifest = null, aliasSHA = null;
  if (fontAliases) {
    const reference = JSON.parse(fs.readFileSync(path.join(sourcePacket, 'reference.json')));
    const css = reference.assets.find(asset => asset.url.endsWith('/inter.css'));
    aliasManifest = deriveFontAliases({css: fs.readFileSync(path.join(sourcePacket, css.file.name)), cssURL: css.url, assets});
    aliasSHA = createHash('sha256').update(JSON.stringify(aliasManifest)).digest('hex');
  }
  const deliveryAssets = [...assets, ...(aliasManifest?.aliases ?? [])];
  if (fontAliases) {
    assert.equal(aliasManifest.aliases.length, 38, 'fixed CSS-declared alias count');
    assert.equal(deliveryAssets.length, 81, '43 canonical assets plus 38 delivery aliases');
  }
  const playwright = approvedPlaywright();
  const playwrightWrapper = fontAliases ? observePlaywrightWrapper(playwright.module.path) : null;
  const pins = {
    source_process: sourceProcess, target_process: targetProcess, packet,
    target_binary: fileIdentity(targetBinary, 'cached target binary'),
    target_generated_manifest: verifiedTargetManifest(targetBinary),
    source_transport: fileIdentity(sourceTransport, 'cached source transport'),
    core_transport: fileIdentity(path.resolve(here, '../catalyst-native/cached-static-reference-server.mjs'), 'v1 core transport'),
    runner: fileIdentity(fileURLToPath(import.meta.url), 'cached smoke runner'),
    collector: fileIdentity(path.join(here, 'payload-browser.mjs'), 'payload collector'),
    payload_checks: fileIdentity(path.join(here, 'payload-checks.mjs'), 'payload checks'),
    cache_checks: fileIdentity(path.join(here, 'cached-static-checks.mjs'), 'cache checks'),
    ...(fontAliases ? {font_alias_helper: fileIdentity(path.join(here, 'font-query-aliases.mjs'), 'font alias helper')} : {}),
    ...(fontAliases ? {warm_identity_checker: fileIdentity(path.join(here, 'warm-font-identity.mjs'), 'supplemental warm identity checker'), playwright_wrapper: playwrightWrapper} : {}),
    playwright, hardware: hardware(),
  };
  const output = fs.mkdtempSync(path.join(parent, 'p10-cached-static-')); fs.chmodSync(output, 0o700);
  const plan = {
    schema_version: 1, id: fontAliases ? 'p10-cached-static-smoke-v2' : 'p10-cached-static-smoke-v1', policy_id: policyID,
    created_at: new Date().toISOString(), pins, assets, delivery_urls: deliveryAssets, alias_manifest: aliasManifest, alias_manifest_sha256: aliasSHA,
    canonical_asset_count: assets.length, alias_url_count: aliasManifest?.aliases.length ?? 0, delivery_url_count: deliveryAssets.length,
    http: `Both listeners: all${deliveryAssets.length} selected URLs, identity/gzip GET/HEAD and conditional GET/HEAD, then excluded/dynamic/refusal cases including empty POST to a selected static URL only. No form mutation. Stop on failure; raw bodies discarded after digest/decoding checks.`,
    browser: {planned_samples: 6, lane: {width: 390, theme: 'dark'}, profiles, kinds: ['cold', 'warm'], sample_count: 1,
      method: 'Existing CDP collector; fresh context per sample, warm uses same-context prime and reload; actual cache hit required. No stream wrappers. Stop at first failure.'},
    supplemental_identity: fontAliases ? {id: WARM_IDENTITY_ID, boundary: 'Separate validator-linked WOFF2 identity only; strict sample, body availability, totals, stop condition and performance gate unchanged.'} : null,
    exclusions: ['not the default360 acquisition', 'no performance, fidelity, whole-app speed or fully warmed React-runtime claim', 'no source runtime JS caching', 'no source/binary/listener mutations or dependency installation'],
    verification: 'not-verified',
  };
  const planFile = put(output, 'plan.json', plan);
  console.log(JSON.stringify({output, plan_sha256: planFile.sha256, status: 'starting separate delivery smoke'}));
  const report = {schema_version: 1, plan: planFile, status: 'incomplete', started_at: new Date().toISOString(), http: [], exclusions: [], samples: [], supplemental_identity: [], planned_samples: 6};
  let browser;
  try {
    for (const [host, origin] of [['source', sourceOrigin], ['target', targetOrigin]]) {
      if (fontAliases) {
        const response = await requestStatic(origin, aliasManifest.aliases[0].url, {method: 'HEAD'});
        assert.equal(response.headers['x-appkit-delivery-experiment'], policyID, 'actual transport experiment identity');
        assert.equal(response.headers['x-appkit-font-aliases-sha256'], aliasSHA, 'actual transport exact alias-set identity');
      }
      await assetHTTPChecks(origin, host, deliveryAssets, report.http, fontAliases ? {policy: policyID, aliasSHA} : null);
      await exclusionHTTPChecks(origin, host, assets, report.exclusions, aliasManifest?.aliases, fontAliases ? {policy: policyID, aliasSHA} : null);
    }
    assert.equal(report.http.length, fontAliases ? 1296 : 688, 'complete selected-URL HTTP matrix');
    assert.equal(report.exclusions.length, fontAliases ? 62 : 32, 'complete exclusion matrix');
    console.log(JSON.stringify({output, http_checks: report.http.length, exclusion_checks: report.exclusions.length, status: 'HTTP delivery checks passed; browser pending'}));
    const {chromium} = await import(pathToFileURL(playwright.module.path).href);
    browser = await chromium.launch({executablePath: CHROME, headless: true, args: ['--disable-background-networking', '--no-proxy-server', '--host-resolver-rules=MAP * 0.0.0.0, EXCLUDE 127.0.0.1']});
    assert.equal(browser.version(), CHROME_VERSION); report.chrome = browser.version();
    for (const kind of ['cold', 'warm']) for (const profile of profiles) {
      const sample = await oneObservation(browser, {profile, lane: {width: 390, theme: 'dark'}, sourceOrigin, targetOrigin}, {kind, ordinal: 1});
      try { sample.asset_delivery = auditSampleAssets(sample, deliveryAssets); }
      catch (error) { sample.ok = false; sample.failures.push(`cache-contract: ${error.message}`); }
      report.samples.push(sample); put(output, `sample-${String(report.samples.length).padStart(3, '0')}.json`, sample);
      if (fontAliases && kind === 'warm') {
        const supplemental = supplementalWarmResult({sample, assets: deliveryAssets, http: report.http,
          origin: profile.id === 'source' ? sourceOrigin : targetOrigin, host: profile.id === 'source' ? 'source' : 'target',
          transport: {policy: policyID, aliasSHA}, toolchain: {playwright_version: playwright.package.version, wrapper_sha256: playwrightWrapper.sha256}});
        try {
          const retained = put(output, `warm-identity-${String(report.samples.length).padStart(3, '0')}.json`, supplemental);
          report.supplemental_identity.push({sample: report.samples.length, status: supplemental.status, file: retained});
        } catch (error) {
          report.supplemental_identity.push({sample: report.samples.length, status: 'unavailable', file: null, reason: `Supplemental receipt write: ${String(error).slice(0, 900)}`});
        }
        console.log(JSON.stringify({output, supplemental_identity: supplemental.status, linked_fonts: supplemental.linked_fonts?.length ?? 0}));
      }
      console.log(JSON.stringify({output, profile: profile.id, kind, ok: sample.ok, cache: sample.cache.status, failures: sample.failures}));
      assert.ok(sample.ok, `${profile.id}/${kind} smoke failed; later samples not run`);
    }
    report.status = 'delivery-smoke-complete-not-a-performance-gate';
  } catch (error) {
    report.status = 'failed-partial-delivery-smoke'; report.failure = String(error);
  } finally {
    if (browser) try { await browser.close(); } catch (error) { report.status = 'failed-browser-close'; report.browser_close_failure = String(error); }
    report.not_run_samples = report.planned_samples - report.samples.length;
    report.failed_samples = report.samples.filter(sample => !sample.ok).length;
    report.finished_at = new Date().toISOString();
    const receipt = put(output, 'receipt.json', report);
    console.log(JSON.stringify({output, status: report.status, http_checks: report.http.length, samples: report.samples.length, receipt_sha256: receipt.sha256}));
    if (report.status !== 'delivery-smoke-complete-not-a-performance-gate') process.exitCode = 1;
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) await main();
