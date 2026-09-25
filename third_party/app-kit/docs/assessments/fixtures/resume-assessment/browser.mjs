// Isolated synthetic assessment review; no app Node dependency or live server.
// node browser.mjs <export-directory> <empty-report-directory>
import http from 'node:http';
import {readFile, writeFile, realpath, readdir} from 'node:fs/promises';
import {relative, resolve, sep} from 'node:path';
import {tmpdir} from 'node:os';
import {createHash} from 'node:crypto';
import {fileURLToPath} from 'node:url';

const args = process.argv.slice(2);
if (args.length !== 2) throw new Error('Expected an export directory and an empty report directory.');
const [root, out] = await Promise.all(args.map(p => realpath(resolve(p))));
const temp = await realpath(tmpdir());
for (const path of [root, out]) {
  const rel = relative(temp, path);
  if (!rel || rel === '..' || rel.startsWith('..' + sep) || rel.startsWith(sep)) throw new Error('Directories must be existing children of system temp.');
}
if (root === out || (await readdir(out)).length) throw new Error('Distinct input and empty output required.');
const hash = data => createHash('sha256').update(data).digest('hex');
const entry = process.env.APPKIT_PLAYWRIGHT_ENTRY || '/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs';
const {chromium} = await import(entry);
const states = ['absent', 'empty', 'thin', 'complete', 'refused', 'hostile', 'malformed'];
const route = '/app/example-assessment';
const csp = "default-src 'none'; style-src 'self' 'unsafe-inline'; img-src data:; form-action 'none'; base-uri 'none'; frame-ancestors 'none'";
const report = {root, out, started: new Date().toISOString(), harnessSHA256: hash(await readFile(fileURLToPath(import.meta.url))),
  budgets: {htmlBytes: 163840, cssBytes: 131072, domNodes: 2000, dclMs: 1500, textContrast: 4.5, targetHeight: 44},
  assets: [], checks: [], samples: [], screenshots: [], errors: [], blocked: [], responses: [], workers: [], downloads: []};
const check = (name, pass, detail) => report.checks.push({name, pass: Boolean(pass), detail});
const servers = [], origins = new Map();
let browser;
try {
  for (const state of states) {
    const assets = new Map();
    for (const [url, filename, type] of [[route, 'app/example-assessment/index.html', 'text/html'], ['/pipeline', 'pipeline/index.html', 'text/html'], ['/assets/app.css', 'assets/app.css', 'text/css']]) {
      const data = await readFile(`${root}/${state}/${filename}`);
      assets.set(url, {data, type}); report.assets.push({state, url, bytes: data.length, sha256: hash(data)});
    }
    let origin;
    const server = http.createServer((req, res) => {
      res.setHeader('Content-Security-Policy', csp); res.setHeader('X-Content-Type-Options', 'nosniff');
      res.setHeader('Referrer-Policy', 'no-referrer'); res.setHeader('Cache-Control', 'no-store');
      if (req.headers.host !== new URL(origin).host) {res.writeHead(403).end(); return;}
      if (!['GET', 'HEAD'].includes(req.method)) {res.writeHead(405, {Allow: 'GET, HEAD'}).end(); return;}
      const asset = assets.get(req.url);
      if (!asset) {res.writeHead(404).end(); return;}
      res.writeHead(200, {'Content-Type': asset.type + '; charset=utf-8', 'Content-Length': asset.data.length});
      res.end(req.method === 'HEAD' ? undefined : asset.data);
    });
    await new Promise((ok, fail) => {server.once('error', fail); server.listen(0, '127.0.0.1', ok);});
    servers.push(server); origin = `http://127.0.0.1:${server.address().port}`; origins.set(state, origin);
  }
  browser = await chromium.launch({executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true, args: ['--no-proxy-server', '--disable-background-networking']});
  report.browser = browser.version();
  const context = await browser.newContext({acceptDownloads: false, deviceScaleFactor: 1});
  context.on('serviceworker', worker => report.workers.push(worker.url()));
  await context.route('**/*', async interception => {
    const req = interception.request(), url = new URL(req.url());
    if (![...origins.values()].includes(url.origin) || !['GET', 'HEAD'].includes(req.method()) || ![route, '/pipeline', '/assets/app.css'].includes(url.pathname) || url.search) {
      report.blocked.push({url: req.url(), method: req.method()}); await interception.abort();
    } else await interception.continue();
  });
  await context.routeWebSocket('**/*', socket => {report.blocked.push({url: socket.url(), method: 'WS'}); socket.close();});
  const page = await context.newPage();
  context.on('page', other => {if (other !== page) {report.errors.push('Unexpected new page'); void other.close();}});
  page.on('pageerror', error => report.errors.push(error.message));
  page.on('console', msg => {if (['error', 'warning'].includes(msg.type())) report.errors.push(msg.text());});
  page.on('download', download => {report.downloads.push(download.suggestedFilename()); void download.cancel();});
  page.on('response', response => report.responses.push({url: response.url(), status: response.status()}));
  for (const state of states) for (const width of [320, 768, 1440]) for (const scheme of ['light', 'dark']) {
    const name = `${state}-${width}-${scheme}`;
    await page.setViewportSize({width, height: 1000});
    await page.emulateMedia({colorScheme: scheme, reducedMotion: 'reduce'});
    const response = await page.goto(origins.get(state) + route, {waitUntil: 'load'});
    const area = page.locator('#assessment-evidence');
    check(name + ': assessment present', await area.count() === 1);
    if (await area.count() !== 1) continue;
    if (state === 'complete' && scheme === 'dark') {
      const pageFile = `${name}-page.png`, collapsedFile = `${name}-collapsed.png`;
      await page.screenshot({path: `${out}/${pageFile}`});
      await area.screenshot({path: `${out}/${collapsedFile}`});
      report.screenshots.push(pageFile, collapsedFile);
    }
    const summaries = area.locator('summary');
    for (let i = 0; i < await summaries.count(); i++) {
      const summary = summaries.nth(i);
      await summary.focus(); await page.keyboard.press('Enter');
      const state = await summary.evaluate(el => ({open: el.parentElement.open, focus: document.activeElement === el, outline: getComputedStyle(el).outlineStyle, height: el.getBoundingClientRect().height}));
      check(`${name}: keyboard disclosure ${i}`, state.open && state.focus && state.outline !== 'none' && state.height >= 44, state);
    }
    const sample = await area.evaluate(area => {
      const visible = [...area.querySelectorAll('*')].filter(el => el.getClientRects().length);
      const rect = area.getBoundingClientRect();
      const canvas = document.createElement('canvas'); canvas.width = canvas.height = 1;
      const ctx = canvas.getContext('2d', {willReadFrequently: true});
      const rgb = css => {ctx.clearRect(0, 0, 1, 1); ctx.fillStyle = css; ctx.fillRect(0, 0, 1, 1); return [...ctx.getImageData(0, 0, 1, 1).data];};
      const lum = c => c.slice(0, 3).map(v => {v /= 255; return v <= .04045 ? v / 12.92 : ((v + .055) / 1.055) ** 2.4;}).reduce((s, v, i) => s + v * [.2126, .7152, .0722][i], 0);
      const contrast = visible.filter(el => [...el.childNodes].some(n => n.nodeType === Node.TEXT_NODE && n.textContent.trim())).map(el => {
        const fg = rgb(getComputedStyle(el).color); let parent = el, bg;
        while (parent) {bg = rgb(getComputedStyle(parent).backgroundColor); if (bg[3] === 255) break; parent = parent.parentElement;}
        if (!parent || fg[3] !== 255) return null;
        const a = lum(fg), b = lum(bg); return (Math.max(a, b) + .05) / (Math.min(a, b) + .05);
      });
      return {text: area.innerText, rate: area.querySelector('[data-assessment-rate]')?.textContent,
        counts: area.querySelector('[data-assessment-count]')?.textContent, score: area.querySelector('[data-assessment-score]')?.textContent,
        verdicts: [...area.querySelectorAll('[data-requirement-verdict]')].map(el => el.dataset.requirementVerdict),
        refusals: area.querySelectorAll('[data-assessment-refusal]').length,
        overflow: visible.filter(el => {const r = el.getBoundingClientRect(); return r.left < rect.left - 1 || r.right > rect.right + 1 || el.scrollWidth > el.clientWidth + 1 && getComputedStyle(el).display !== 'inline';}).map(el => el.tagName + ':' + el.textContent.slice(0, 65)),
        rect: {x: rect.x, width: rect.width}, pageWidth: document.documentElement.clientWidth, scrollWidth: document.documentElement.scrollWidth,
        contrast, scripts: document.scripts.length, forms: document.forms.length, nodes: document.querySelectorAll('*').length,
        injected: area.querySelectorAll('script,img,svg,a,iframe,form').length, background: getComputedStyle(document.body).backgroundColor,
        dclMs: performance.getEntriesByType('navigation')[0].domContentLoadedEventEnd};
    });
    report.samples.push({name, ...sample});
    check(name + ': inert exact delivery', response.status() === 200 && response.headers()['content-security-policy'] === csp && sample.scripts === 0 && sample.forms === 0 && sample.injected === 0);
    check(name + ': readable geometry', sample.scrollWidth <= sample.pageWidth + 1 && sample.overflow.length === 0 && sample.rect.x >= 0 && sample.rect.x + sample.rect.width <= width + 1, {overflow: sample.overflow, scroll: sample.scrollWidth, width});
    check(name + ': sampled text contrast', sample.contrast.length && sample.contrast.every(c => c !== null && c >= 4.5), sample.contrast);
    check(name + ': bounded render', sample.nodes <= report.budgets.domNodes && sample.dclMs <= report.budgets.dclMs, {nodes: sample.nodes, dclMs: sample.dclMs});
    if (state === 'absent') check(name + ': absent not zero', sample.text.includes('No stored assessment.') && !sample.rate && !sample.text.includes('0%'));
    else {
      check(name + ': uncapped score refused', sample.score === 'unscored — no source class' && !sample.text.includes('91/100'));
      const expected = {empty: [0, 0, 'not enough outcomes yet (n=0, need 5)'], thin: [1, 4, 'not enough outcomes yet (n=4, need 5)'], complete: [1, 5, '20%'], refused: [3, 5, '60%'], hostile: [1, 5, '20%'], malformed: [1, 5, 'Unknown rate unavailable: 1 recorded row has no requirement text.']}[state];
      check(name + ': honest rate and raw counts', sample.rate === expected[2] && sample.counts === `${expected[0]} unknown of ${expected[1]} requirements`, {rate: sample.rate, counts: sample.counts});
      if (state === 'empty') check(name + ': legacy empty evidence', sample.text.includes('No requirement matches recorded.') && sample.text.includes('No author provenance recorded.'));
      else check(name + ': native verdict ledger', sample.verdicts.length === expected[1] && sample.verdicts.filter(v => v === 'unknown').length === expected[0]);
      if (state === 'refused') check(name + ': refusal cannot collapse away', sample.refusals === 2 && sample.text.includes('1 unresolvable reference(s) discarded'));
      if (state === 'hostile') check(name + ': hostile values stay visible text', sample.text.includes('<script>alert("citation")</script>') && sample.text.includes('javascript:alert("reference")') && sample.text.includes('<svg onload=alert("model")>'));
      if (state === 'malformed') check(name + ': malformed provenance is named', sample.text.includes('Author provenance is invalid:') && sample.text.includes('Requirement not recorded'));
    }
    if (scheme === 'dark' && ['complete', 'hostile', 'malformed'].includes(state)) {
      const file = `${name}.png`; await area.screenshot({path: `${out}/${file}`}); report.screenshots.push(file);
    }
    for (let i = await summaries.count() - 1; i >= 0; i--) {
      await summaries.nth(i).focus(); await page.keyboard.press('Enter');
      check(`${name}: collapse ${i}`, !(await summaries.nth(i).evaluate(el => el.parentElement.open)));
    }
    const breadcrumb = page.locator('main a[href="/pipeline"]').first();
    await breadcrumb.click(); await page.waitForURL(origins.get(state) + '/pipeline');
    await page.goBack({waitUntil: 'load'});
    check(name + ': pipeline and back retain detail URL', page.url() === origins.get(state) + route && await area.count() === 1);
  }
  check('payload budgets', report.assets.every(a => a.bytes <= (a.url.endsWith('.css') ? report.budgets.cssBytes : report.budgets.htmlBytes)), report.assets);
  check('no unexpected traffic or runtime errors', !report.errors.length && !report.blocked.length && !report.workers.length && !report.downloads.length && report.responses.every(r => r.status === 200), {errors: report.errors, blocked: report.blocked});
} catch (error) {report.fatal = error.stack;}
finally {
  if (browser) await browser.close();
  await Promise.all(servers.map(server => new Promise(ok => server.close(ok))));
  report.finished = new Date().toISOString();
  await writeFile(`${out}/report.json`, JSON.stringify(report, null, 2) + '\n', {mode: 0o600});
  console.log(JSON.stringify({out, checks: report.checks.length, failed: report.checks.filter(c => !c.pass).map(c => c.name), fatal: report.fatal, screenshots: report.screenshots}, null, 2));
  if (report.fatal || report.checks.some(c => !c.pass)) process.exitCode = 1;
}
