// Optional, isolated test tool. No app Node dependency or live RAC server.
// Usage: node browser.mjs <before-export> <after-export> <empty-report-directory>
import http from 'node:http';
import {readFile, writeFile, readdir, realpath} from 'node:fs/promises';
import {resolve, relative, sep} from 'node:path';
import {tmpdir} from 'node:os';
import {createHash} from 'node:crypto';
import {fileURLToPath} from 'node:url';

const [beforeArg, afterArg, outputArg] = process.argv.slice(2);
if (!beforeArg || !afterArg || !outputArg) throw new Error('Expected before, after, and empty output directories.');
const digest = data => createHash('sha256').update(data).digest('hex');
const paths = await Promise.all([beforeArg, afterArg, outputArg].map(p => realpath(resolve(p))));
const tempRoots = await Promise.all([tmpdir(), '/tmp'].map(p => realpath(p)));
for (const path of paths) {
  if (!tempRoots.some(root => {const r = relative(root, path); return r && r !== '..' && !r.startsWith('..' + sep) && !r.startsWith(sep);})) {
    throw new Error('Every directory must be an existing child of a system temporary directory.');
  }
}
if (new Set(paths).size !== paths.length || (await readdir(paths[2])).length) throw new Error('Distinct inputs and an empty output directory required.');
const [before, after, output] = paths;
const playwrightPath = process.env.APPKIT_PLAYWRIGHT_ENTRY || '/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs';
const {chromium} = await import(playwrightPath);
const report = {started: new Date().toISOString(), before, after, output, checks: [], baseline: [], samples: [], errors: [], blocked: [], responses: [], assets: [], screenshots: [], workers: [], downloads: []};
const check = (name, pass, detail) => report.checks.push({name, pass: Boolean(pass), detail});
report.harnessSHA256 = digest(await readFile(fileURLToPath(import.meta.url)));
report.playwright = {entry: playwrightPath, sha256: digest(await readFile(playwrightPath))};
const servers = [];
const origins = new Map();
const csp = "default-src 'none'; style-src 'self' 'unsafe-inline'; img-src data:; form-action 'none'; base-uri 'none'; frame-ancestors 'none'";
let browser;
try {
  for (const [version, root] of [['before', before], ['after', after]]) {
    for (const state of ['empty', 'populated', 'outcomes', 'incomplete', 'terminal']) {
      const assets = new Map();
      for (const [url, filename, type] of [
        ['/pipeline', 'pipeline/index.html', 'text/html; charset=utf-8'],
        ['/people', 'people/index.html', 'text/html; charset=utf-8'],
        ['/today', 'today/index.html', 'text/html; charset=utf-8'],
        ['/assets/app.css', 'assets/app.css', 'text/css; charset=utf-8'],
      ]) {
        const data = await readFile(`${root}/${state}/${filename}`);
        assets.set(url, {data, type});
        report.assets.push({version, state, url, bytes: data.length, sha256: digest(data)});
      }
      let origin;
      const server = http.createServer((req, res) => {
        const asset = assets.get(req.url);
        res.setHeader('Content-Security-Policy', csp);
        res.setHeader('Cache-Control', 'no-store');
        res.setHeader('X-Content-Type-Options', 'nosniff');
        res.setHeader('Referrer-Policy', 'no-referrer');
        if (req.headers.host !== new URL(origin).host) {res.writeHead(403).end(); return;}
        if (!['GET', 'HEAD'].includes(req.method)) {res.writeHead(405, {Allow: 'GET, HEAD'}).end(); return;}
        if (!asset) {res.writeHead(404).end('Inert preview: route unavailable'); return;}
        res.writeHead(200, {'Content-Type': asset.type, 'Content-Length': asset.data.length});
        res.end(req.method === 'HEAD' ? undefined : asset.data);
      });
      await new Promise((ok, fail) => {server.once('error', fail); server.listen(0, '127.0.0.1', ok);});
      servers.push(server);
      origin = `http://127.0.0.1:${server.address().port}`;
      origins.set(`${version}/${state}`, origin);
    }
  }
  browser = await chromium.launch({executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true, args: ['--no-proxy-server', '--disable-background-networking']});
  report.browser = browser.version();
  const context = await browser.newContext({acceptDownloads: false, deviceScaleFactor: 1});
  context.on('serviceworker', worker => report.workers.push(worker.url()));
  await context.route('**/*', async route => {
    const request = route.request();
    const url = new URL(request.url());
    if (![...origins.values()].includes(url.origin) || !['GET', 'HEAD'].includes(request.method()) || !['/pipeline', '/people', '/today', '/assets/app.css'].includes(url.pathname) || url.search) {
      report.blocked.push({url: request.url(), method: request.method()});
      await route.abort();
    } else await route.continue();
  });
  await context.routeWebSocket('**/*', ws => {report.blocked.push({url: ws.url(), method: 'WS'}); ws.close();});
  const page = await context.newPage();
  context.on('page', other => {if (other !== page) {report.errors.push('Unexpected new page'); void other.close();}});
  page.on('pageerror', e => report.errors.push(e.message));
  page.on('console', msg => {if (['error', 'warning'].includes(msg.type())) report.errors.push(`${msg.type()}: ${msg.text()}`);});
  page.on('download', download => {report.downloads.push(download.suggestedFilename()); void download.cancel();});
  page.on('response', response => report.responses.push({url: response.url(), status: response.status()}));
  for (const version of ['before', 'after']) for (const width of [320, 768, 1440]) for (const scheme of ['light', 'dark']) {
    const cases = version === 'before' ? [['populated', '/pipeline'], ['incomplete', '/pipeline']] : [
      ...['empty', 'populated', 'outcomes', 'incomplete', 'terminal'].map(state => [state, '/pipeline']),
      ['outcomes', '/people'], ['incomplete', '/people'],
    ];
    for (const [state, route] of cases) {
      const name = `${version}-${state}-${route.slice(1)}-${width}-${scheme}`;
      await page.setViewportSize({width, height: 1000});
      await page.emulateMedia({colorScheme: scheme, reducedMotion: 'reduce'});
      const response = await page.goto(origins.get(`${version}/${state}`) + route, {waitUntil: 'load'});
      const sample = await page.evaluate(() => {
        const area = document.querySelector('[aria-labelledby="pipeline-analytics-heading"]') || document.querySelector('[aria-label="How the search is going"]') || document.querySelector('[aria-label="Relationship health"]');
        const rect = area.getBoundingClientRect();
        const cells = [...area.querySelectorAll('th, td')].filter(el => el.getClientRects().length);
        return {text: area.textContent.replace(/\s+/g, ' ').trim(), width: document.documentElement.clientWidth, scroll: document.documentElement.scrollWidth,
          area: {x: rect.x, y: rect.y, width: rect.width, height: rect.height}, scripts: document.scripts.length, forms: document.forms.length,
          cellOverflows: cells.filter(el => {const r = el.getBoundingClientRect(), bounds = el.closest('section').getBoundingClientRect(); return r.right > Math.min(document.documentElement.clientWidth, bounds.right) + 1 || r.left < Math.max(0, bounds.left) - 1 || el.scrollWidth > el.clientWidth + 1;}).map(el => el.textContent),
          charts: [...area.querySelectorAll('figure svg')].map(el => {const r = el.getBoundingClientRect(), bounds = el.closest('figure').getBoundingClientRect(); return {width: r.width, height: r.height, outside: r.right > bounds.right + 1 || r.left < bounds.left - 1,
            text: [...el.querySelectorAll('text')].map(text => {const t = text.getBoundingClientRect(); return {value: text.textContent, outside: t.right > r.right + 1 || t.left < r.left - 1 || t.bottom > r.bottom + 1 || t.top < r.top - 1, renderedFont: Number.parseFloat(getComputedStyle(text).fontSize) * r.width / el.viewBox.baseVal.width};})};}),
          summaries: [...area.querySelectorAll('summary')].map(el => ({text: el.textContent, height: el.getBoundingClientRect().height})),
          statLabels: [...area.querySelectorAll('dl[aria-label="pipeline summary"] dt')].map(el => el.textContent),
          background: getComputedStyle(document.body).backgroundColor};
      });
      report.samples.push({name, ...sample});
      if (version === 'before') {
        report.baseline.push({name, mixedScore: sample.text.includes('Avg fit'), misleadingDisjointRate: state === 'incomplete' && sample.text.includes('20%'), outerOverflow: sample.scroll > sample.width + 1, cellOverflows: sample.cellOverflows});
      } else {
        check(name + ': safe delivery', response.status() === 200 && response.headers()['content-security-policy'] === csp && sample.scripts === 0 && sample.forms === 0);
        check(name + ': readable geometry', sample.scroll <= sample.width + 1 && sample.cellOverflows.length === 0 && sample.area.x >= 0 && sample.area.x + sample.area.width <= width + 1, sample);
        check(name + ': disclosure targets', sample.summaries.every(s => s.height >= 44), sample.summaries);
        check(name + ': readable chart labels', sample.charts.every(chart => !chart.outside && chart.text.every(text => !text.outside && text.renderedFont >= 12)), sample.charts);
        if (route === '/pipeline') {
          check(name + ': bounded direct summary', sample.area.width <= 897 && JSON.stringify(sample.statLabels) === JSON.stringify(['Active', 'In triage', 'Stored assessments']) && !sample.text.includes('Avg fit') && !sample.text.includes('Applied+'));
          if (state === 'empty') check(name + ': genuine empty states', sample.text.includes('No applications tracked yet.') && sample.text.includes('No application history yet.') && await page.locator('[aria-labelledby="pipeline-analytics-heading"] svg').count() === 0);
          if (state === 'terminal') check(name + ': closed is not active', sample.text.includes('No applications currently in active stages.') && sample.text.includes('1 closed application is excluded'));
          if (state !== 'empty') {
            const summary = page.getByText('All current-stage counts', {exact: true});
            await summary.focus(); await page.keyboard.press('Enter');
            check(name + ': keyboard stage data', await summary.evaluate(el => el.parentElement.open) && await page.getByRole('columnheader', {name: 'Current stage', exact: true}).isVisible());
            await page.keyboard.press('Enter');
          }
        }
        if (state !== 'empty') {
          const summary = page.getByText('Rates, sample sizes and sources', {exact: true});
          await summary.focus(); await page.keyboard.press('Enter');
          const detailsText = await summary.evaluate(el => el.parentElement.textContent.replace(/\s+/g, ' ').trim());
          check(name + ': keyboard rate evidence', await summary.evaluate(el => el.parentElement.open) && detailsText.includes('Source: stored application registry.'));
          if (state === 'outcomes') check(name + ': exact measured counts', detailsText.includes('Response: 80% (4/5). Interview: 40% (2/5).'));
          if (state === 'incomplete') check(name + ': invalid cohort refused', detailsText.includes('Rates unavailable: incomplete sent evidence.') && !detailsText.includes('20%') && sample.text.includes('1 application has an outcome without recorded sent evidence.'));
          if (state === 'populated') check(name + ': thin sample stays unavailable', detailsText.includes('not enough outcomes yet (n=3, need 5)') && !detailsText.includes('0%'));
          await page.keyboard.press('Enter');
        }
      }
      if (scheme === 'dark' && route === '/pipeline' && ((version === 'before' && state === 'populated') || (version === 'after' && ['outcomes', 'incomplete'].includes(state)))) {
        const area = page.locator(version === 'after' ? '[aria-labelledby="pipeline-analytics-heading"]' : '[aria-label="How the search is going"]');
        const filename = `${name}.png`;
        await area.screenshot({path: `${output}/${filename}`});
        report.screenshots.push(filename);
      }
    }
  }
  check('no unexpected runtime traffic/errors', report.errors.length === 0 && report.blocked.length === 0 && report.workers.length === 0 && report.downloads.length === 0 && report.responses.every(r => r.status === 200), {errors: report.errors, blocked: report.blocked});
} catch (error) {
  report.fatal = error.stack;
} finally {
  if (browser) {await browser.close(); report.browserClosed = true;}
  await Promise.all(servers.map(server => new Promise(ok => server.close(ok))));
  report.finished = new Date().toISOString();
  await writeFile(`${output}/receipt.json`, JSON.stringify(report, null, 2) + '\n', {mode: 0o600});
  console.log(JSON.stringify({output, checks: report.checks.length, failed: report.checks.filter(c => !c.pass), fatal: report.fatal, screenshots: report.screenshots}, null, 2));
  if (report.fatal || report.checks.some(c => !c.pass)) process.exitCode = 1;
}
