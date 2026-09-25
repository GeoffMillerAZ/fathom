// Opt-in boundary proof, not a production route or a high-volume renderer claim.
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import os from 'node:os';
import {inflateSync} from 'node:zlib';
import {pathToFileURL} from 'node:url';

const sha = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const same = (a, b) => JSON.stringify(a) === JSON.stringify(b);
const origin = new URL(process.argv[2] ?? '');
if (origin.protocol !== 'http:' || origin.hostname !== '127.0.0.1' || !origin.port || origin.pathname !== '/' || origin.search || origin.hash || origin.username || origin.password) throw Error('Explicit bare loopback HTTP origin required');
const out = process.argv[3] ?? '';
const stat = fs.lstatSync(out);
if (!path.isAbsolute(out) || !stat.isDirectory() || stat.isSymbolicLink() || (stat.mode & 0o777) !== 0o700 || fs.realpathSync(out) !== out) throw Error('Existing real private 0700 output directory required');
function read(name, limit = 2 << 20) {
  const file = path.join(out, name), s = fs.lstatSync(file);
  if (path.basename(name) !== name || !s.isFile() || s.isSymbolicLink() || s.size > limit) throw Error('Bounded regular local input required: ' + name);
  return fs.readFileSync(file);
}
const manifestBytes = read('manifest.json'), manifestSHA = sha(manifestBytes);
if (!/^[a-f0-9]{64}$/.test(process.argv[4] ?? '') || manifestSHA !== process.argv[4]) throw Error('Independent manifest hash required');
const manifest = JSON.parse(manifestBytes);
const expected = {cases: 4, rows: 200, plotted: 189, missing: 11, bounds: 16, html_max_bytes: 262144, css_max_bytes: 131072, dom_max_nodes: 3500, dcl_p95_max_ms: 500};
if (manifest.kind !== 'private-analytics-scale-browser-input' || !same(manifest.expected, expected)) throw Error('Frozen fixture or budgets differ');
for (const [file, key] of [['harness.mjs','harness'], ['fixture.go.txt','fixture_test'], ['max_rows_test.go.txt','model_test'], ['renderer.go.txt','renderer'], ['css.txt','css']]) {
  if (sha(read(file)) !== manifest.inputs[key]) throw Error('Snapshot mismatch: ' + key);
}
if (sha(fs.readFileSync(new URL(import.meta.url))) !== manifest.inputs.harness) throw Error('Executed harness differs');
const entry = process.env.APPKIT_PLAYWRIGHT_ENTRY ?? '';
if (!path.isAbsolute(entry) || path.basename(entry) !== 'index.mjs') throw Error('Explicit reviewed Playwright entry required');
const packageBytes = fs.readFileSync(path.join(path.dirname(entry), 'package.json')), pkg = JSON.parse(packageBytes);
if (pkg.name !== 'playwright' || pkg.version !== '1.63.0' || sha(fs.readFileSync(entry)) !== manifest.inputs.playwright) throw Error('Reviewed Playwright 1.63.0 identity required');
const base = '/__test/analytics-scale', csvPath = base + '/export.csv';
const report = {schema: 1, manifest_sha256: manifestSHA, cases: 0, failures: 0, errors: [], checks: [], lanes: [], artifacts: [], requests: [], blocked: [], downloadAborts: [],
  timing: {samples: [], budget_ms: 500, diagnostic_only: true},
  environment: {node: process.version, platform: process.platform, arch: process.arch, cpus: os.cpus().length, cpu: os.cpus()[0]?.model, memory_bytes: os.totalmem(), playwright: pkg.version, playwright_package_sha256: sha(packageBytes)},
  limits: ['Synthetic 200-row contract boundary, not production volume or app adoption.', 'Headless Chrome, 100% zoom, page scripts disabled, fresh contexts; no human, screen-reader, touch or native-host claim.', 'Viewport captures show named task regions, not full-chart fidelity comparisons.', 'Eight local DOM-ready observations include four native form navigations; not eight cold contexts, a speedup, LCP/INP, or remote latency.']};
const check = (pass, name, detail) => {report.checks.push({name, pass: !!pass, detail}); if (!pass) report.failures++;};
function artifact(name) {
  const bytes = read(name, 12 << 20);
  report.artifacts.push({path: name, sha256: sha(bytes), bytes: bytes.length});
}
function save(name, bytes) {
  fs.writeFileSync(path.join(out, name), bytes, {flag: 'wx', mode: 0o600});
  artifact(name);
}
async function bounded(promise, ms, label) {
  let timer;
  try {return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(label+' timed out')),ms);})]);}
  finally {clearTimeout(timer);}
}
function pngPixels(bytes, width) {
  if(!bytes.subarray(0,8).equals(Buffer.from([137,80,78,71,13,10,26,10])))throw Error('PNG signature absent');
  const chunks=[]; let w,h,channels,ended=false;
  for(let offset=8;offset+12<=bytes.length;) {
    const n=bytes.readUInt32BE(offset),type=bytes.toString('ascii',offset+4,offset+8),end=offset+12+n;
    if(end>bytes.length)throw Error('Truncated PNG');
    if(type==='IHDR') {
      w=bytes.readUInt32BE(offset+8);h=bytes.readUInt32BE(offset+12);
      channels=bytes[offset+17]===2?3:bytes[offset+17]===6?4:0;
      if(w!==width||h!==1000||bytes[offset+16]!==8||!channels||bytes[offset+20]!==0)throw Error('Unexpected screenshot geometry or encoding');
    }
    if(type==='IDAT')chunks.push(bytes.subarray(offset+8,offset+8+n));
    if(type==='IEND'){ended=true;if(end!==bytes.length)throw Error('Trailing PNG data');}
    offset=end;
  }
  if(!ended||!channels)throw Error('Incomplete PNG');
  const stride=w*channels,raw=inflateSync(Buffer.concat(chunks),{maxOutputLength:(stride+1)*h});
  if(raw.length!==(stride+1)*h)throw Error('PNG decoded row count differs');
  const prior=Buffer.alloc(stride),row=Buffer.alloc(stride),colors=new Set();
  const paeth=(a,b,c)=>{const p=a+b-c,pa=Math.abs(p-a),pb=Math.abs(p-b),pc=Math.abs(p-c);return pa<=pb&&pa<=pc?a:pb<=pc?b:c;};
  for(let y=0;y<h;y++) {
    const filter=raw[y*(stride+1)];if(filter>4)throw Error('Unsupported PNG filter');
    for(let x=0;x<stride;x++) {
      const left=x>=channels?row[x-channels]:0,up=prior[x],corner=x>=channels?prior[x-channels]:0;
      row[x]=(raw[y*(stride+1)+1+x]+[0,left,up,Math.floor((left+up)/2),paeth(left,up,corner)][filter])&255;
    }
    for(let x=0;x<stride&&colors.size<256;x+=channels)colors.add(row[x]*65536+row[x+1]*256+row[x+2]);
    row.copy(prior);
  }
  if(colors.size<32)throw Error('Screenshot lacks meaningful pixel variation');
  return {width:w,height:h,decoded_rows:h,distinct_colors_at_least:colors.size};
}
const missingReason = 'Synthetic source unavailable for this selected row.';
function rowsFor(variant) {
  return Array.from({length: 200}, (_, i) => {
    const missing = i > 0 && i % 17 === 0, bounded = !missing && ((i > 0 && i % 13 === 0) || i === 199);
    return {id: 'service-' + String(i + 1).padStart(3, '0'), label: variant === 'long' && i === 0 ? 'Long service label '.repeat(18) : 'Service ' + String(i + 1).padStart(3, '0'),
      value: missing ? '' : String(i), low: bounded ? String(i - .25) : '', high: bounded ? String(i + .25) : '',
      state: missing ? 'missing' : 'present', reason: missing ? missingReason : '', role: i % 2 ? 'info' : 'accent', meaning: i % 2 ? 'Worker services' : 'API services'};
  });
}
function parseCSV(text) {
  const rows = []; let row = [], field = '', quoted = false;
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (quoted) {
      if (c === '"' && text[i + 1] === '"') {field += c; i++;}
      else if (c === '"') quoted = false;
      else field += c;
    } else if (c === '"') quoted = true;
    else if (c === ',') {row.push(field); field = '';}
    else if (c === '\n') {row.push(field); rows.push(row); row = []; field = '';}
    else if (c !== '\r') field += c;
  }
  if (quoted) throw Error('Unterminated CSV quote');
  if (field || row.length) {row.push(field); rows.push(row);}
  const header = ['id','label','value','unit','state','missing_reason','lower','upper','role','role_label'];
  const index = rows.findIndex(r => same(r, header));
  if (index < 0) throw Error('Analytics CSV schema absent');
  return {context: rows.slice(0, index).filter(r => r.length > 1), rows: rows.slice(index + 1)};
}

async function inspect(page, variant, lane) {
  const state = await page.evaluate(() => {
    const root = document.querySelector('[data-scale-specimen]'), table = root.querySelector('table');
    const svg = root.querySelector('svg:not([data-analytics-legend-swatch])');
    const texts = [...(svg?.querySelectorAll('text') ?? [])];
    const boxes = texts.map(t => {const r = t.getBBox(); return {x:r.x, y:r.y, width:r.width, height:r.height};});
    const view = svg?.viewBox.baseVal;
    const lines = [...table.tBodies[0].rows[0].cells].map(cell => {
      const walker = document.createTreeWalker(cell, NodeFilter.SHOW_TEXT), rects = [];
      for (let node = walker.nextNode(); node; node = walker.nextNode()) {
        if (!node.textContent.trim()) continue;
        const range = document.createRange(); range.selectNodeContents(node);
        rects.push(...[...range.getClientRects()].filter(r => r.width && r.height).map(r => ({top:r.top, bottom:r.bottom})));
      }
      return rects;
    });
    return {variant:root.dataset.variant, rows:[...table.tBodies[0].rows].map(r => [...r.cells].map(c => c.textContent.trim())),
      headers:[...table.tHead.rows[0].cells].map(c => c.textContent.trim()), rowHeaders:table.querySelectorAll('tbody th[scope="row"]').length,
      text:root.textContent, dom:document.querySelectorAll('*').length, scripts:document.querySelectorAll('script').length,
      overflow:document.documentElement.scrollWidth > document.documentElement.clientWidth,
      swatches:root.querySelectorAll('[data-analytics-legend-swatch]').length, charts:root.querySelectorAll('svg:not([data-analytics-legend-swatch])').length,
      svg:svg ? {texts:texts.map(t => t.textContent), boxes, width:view.width, height:view.height,
        tracks:svg.querySelectorAll('rect[fill-opacity="0.5"]').length,
        fills:[...svg.querySelectorAll('rect')].filter(r => r.querySelector('title')).length} : null,
      firstLines:lines.map(r => r[0]?.top ?? null), labelLineCount:lines[0].length,
      labelStyle:{whiteSpace:getComputedStyle(table.tBodies[0].rows[0].cells[0]).whiteSpace, overflowWrap:getComputedStyle(table.tBodies[0].rows[0].cells[0]).overflowWrap}};
  });
  const want = rowsFor(variant), cells = want.map(r => [r.label.trim(),r.state === 'missing' ? 'Missing' : r.value,r.low ? r.low + ' to ' + r.high : 'Not provided',r.reason || 'Present',r.meaning]);
  check(state.variant === variant && same(state.rows, cells), lane + ': all 200 ordered rows match independent truth', {variant, rows:state.rows.length});
  check(same(state.headers, ['Label','Value (events)','Bounds (events)','State','Meaning']) && state.rowHeaders === 200, lane + ': table schema and row headings retained');
  check(state.text.includes('11 unavailable of 200 selected rows.') && state.text.includes('Synthetic scale fixture; no operational data or claim.') && state.text.includes('Uncertainty: Synthetic scale fixture; uncertainty is not quantified.'), lane + ': coverage source and uncertainty visible');
  check(state.dom <= expected.dom_max_nodes && !state.overflow && state.scripts === 0, lane + ': DOM bound, local horizontal regions, no page scripts', {dom:state.dom, overflow:state.overflow, scripts:state.scripts});
  if (variant === 'normal') {
    const measured = want.filter(r => r.state === 'present');
    check(state.charts === 1 && state.swatches === 2 && state.svg.tracks === 189 && state.svg.fills === 188, lane + ': 189 measured rows, zero has no positive fill', state.svg && {tracks:state.svg.tracks,fills:state.svg.fills});
    check(same(state.svg?.texts, measured.flatMap(r => [r.label, r.value + ' events'])), lane + ': all plotted labels and values agree; missing rows absent');
    check(state.svg?.boxes.every((b,i,a) => b.x >= -.5 && b.y >= -.5 && b.x+b.width <= state.svg.width+.5 && b.y+b.height <= state.svg.height+.5 && (i%2 || b.x+b.width <= a[i+1].x+.5)), lane + ': all SVG text fits and each label stays separate from value');
  } else {
    check(state.charts === 0 && state.swatches === 0 && state.text.includes('Bar chart omitted:'), lane + ': explicit complete table fallback without orphan legend');
    check(state.labelLineCount > 1 && state.labelStyle.whiteSpace === 'normal' && state.firstLines.every(y => y !== null && Math.abs(y-state.firstLines[0]) <= 1), lane + ': wrapped first label stays aligned with its value and meaning', {lines:state.labelLineCount,tops:state.firstLines});
  }
  return {dom:state.dom, chart_height:state.svg?.height ?? 0, rows:state.rows.length};
}

async function tabTo(page, selector, name) {
  for (let i = 0; i < 100; i++) {
    if (await page.evaluate(s => document.activeElement?.matches(s), selector)) {check(true,name,{tabs:i}); return;}
    await page.keyboard.press('Tab');
  }
  check(false,name); throw Error('Keyboard could not reach ' + selector);
}
async function settleScroll(page) {
  let previous='',stable=0;
  for(let i=0;i<30;i++) {
    await page.waitForTimeout(40);
    const current=await page.evaluate(()=>JSON.stringify([scrollX,scrollY,...[...document.querySelectorAll('[role="region"]')].flatMap(e=>[e.scrollLeft,e.scrollTop])]));
    stable=current===previous?stable+1:0; previous=current;
    if(stable>=5)return;
  }
  throw Error('Native scrolling did not settle within its observation bound');
}
async function bringRow(page, selector, name) {
  for (let i = 0; i < 70; i++) {
    await settleScroll(page);
    const box = await page.locator(selector).boundingBox();
    if (!box) throw Error('Missing task target: ' + selector);
    if (box.y >= 0 && box.y+box.height <= 1000) {check(true,name,{keys:i,box}); return box;}
    await page.keyboard.press(box.y < 0 ? 'PageUp' : 'PageDown');
  }
  check(false,name,await page.locator(selector).boundingBox());
  throw Error('Native page keys did not expose complete target');
}
async function capture(page, name, selector) {
  await settleScroll(page);
  const before=await page.locator(selector).boundingBox(),viewport=page.viewportSize();
  const visible=b=>b&&b.width>0&&b.height>0&&b.x>=-1&&b.y>=-1&&b.x+b.width<=viewport.width+1&&b.y+b.height<=viewport.height+1;
  check(visible(before),name+': entire named task target is on screen before capture',{selector,box:before});
  if(!visible(before))throw Error('Capture target is not fully visible: '+name);
  const file = name + '.png';
  await page.screenshot({path:path.join(out,file), fullPage:false});
  fs.chmodSync(path.join(out,file),0o600); artifact(file);
  check(true,name+': complete nonblank viewport PNG',pngPixels(read(file,12<<20),page.viewportSize().width));
  const after=await page.locator(selector).boundingBox();
  check(visible(after)&&['x','y','width','height'].every(k=>Math.abs(after[k]-before[k])<.5),name+': named target stays visible and stable through capture',{before,after});
}
async function tableTask(page, name) {
  await tabTo(page, '[data-scale-specimen] [role="region"]:has(table)', name + ': Tab reaches named table region');
  const region = page.locator('[data-scale-specimen] [role="region"]:has(table)');
  check((await region.getAttribute('aria-label'))?.includes('data table'),name + ': focus region has meaningful name');
  await bringRow(page,'[data-scale-specimen] tbody tr:first-child',name + ': first row fully reachable with native page keys');
  await capture(page,name + '-first-label','[data-scale-specimen] tbody tr:first-child th');
  for (let i = 0; i < 60; i++) {
    const done = await region.evaluate(e => e.scrollLeft+e.clientWidth >= e.scrollWidth-1);
    if (done) break;
    await page.keyboard.press('ArrowRight'); await page.waitForTimeout(30);
  }
  await settleScroll(page);
  await bringRow(page,'[data-scale-specimen] tbody tr:first-child',name + ': first row remains reachable after horizontal scroll');
  const horizontal = await region.evaluate(e => {const r=e.getBoundingClientRect(), cell=e.querySelector('tbody tr td:last-child').getBoundingClientRect(); return {left:e.scrollLeft,width:e.clientWidth,scroll:e.scrollWidth,meaningVisible:cell.left>=r.left-1&&cell.right<=r.right+1};});
  check(horizontal.meaningVisible,name + ': native horizontal keys expose Meaning',horizontal);
  await capture(page,name + '-first-meaning','[data-scale-specimen] tbody tr:first-child td:last-child');
  await bringRow(page,'[data-scale-specimen] tbody tr:last-child',name + ': last row fully reachable with native page keys');
  await capture(page,name + '-last-meaning','[data-scale-specimen] tbody tr:last-child td:last-child');
}
async function downloadTask(page, context, variant, theme, lane) {
  await tabTo(page,'a[data-scale-export]',lane + ': Tab reaches export');
  const href = await page.locator('[data-scale-export]').getAttribute('href');
  const [download] = await Promise.all([page.waitForEvent('download'),page.keyboard.press('Enter')]);
  const name = lane + '.csv';
  await download.saveAs(path.join(out,name)); fs.chmodSync(path.join(out,name),0o600); artifact(name);
  const bytes = read(name), parsed = parseCSV(bytes.toString('utf8'));
  const want = rowsFor(variant).map(r => [r.id,r.label,r.value,'events',r.state,r.reason,r.low,r.high,r.role,r.meaning]);
  check(sha(bytes) === manifest.responses[variant+'-csv'] && same(parsed.rows,want),lane + ': native export matches pinned bytes and 200 independent rows',{bytes:bytes.length,rows:parsed.rows.length,url:new URL(href,origin).href});
  const contextFields = Object.fromEntries(parsed.context);
  check(parsed.context.length === 17 && contextFields.coverage_note === '11 unavailable of 200 selected rows.' && contextFields.completeness === 'partial' && contextFields.evaluated_at === '2026-10-05T12:00:00Z' && contextFields.unit === 'events',lane + ': export retains selection context');
  return new URL(href,origin).href;
}

let browser, browserServer, ownedProcess, watchdog;
async function closeOwnedBrowser() {
  if(!browserServer)return;
  let graceful=true;
  try {await bounded(browserServer.close(),8000,'Owned browser close');}
  catch(e) {
    graceful=false;report.errors.push({kind:'cleanup',message:e.message});
    await bounded(browserServer.kill(),4000,'Owned browser kill').catch(e=>report.errors.push({kind:'cleanup',message:e.message}));
  }
  let absent=false;
  try {process.kill(ownedProcess.pid,0);}catch(e){if(e.code==='ESRCH')absent=true;else report.errors.push({kind:'cleanup',message:e.message});}
  report.cleanup={pid:ownedProcess.pid,graceful,owned_parent_absent:absent,descendant_inventory:false};
  check(graceful&&absent,'Owned Chrome parent exits after bounded close',report.cleanup);
}
const completedDownloads = [];
try {
  const {chromium} = await import(pathToFileURL(entry).href);
  browserServer = await chromium.launchServer({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true,timeout:20000,args:['--no-first-run','--no-default-browser-check']});
  ownedProcess=browserServer.process();report.owned_browser_pid=ownedProcess.pid;
  save('owned-browser.json',JSON.stringify({pid:ownedProcess.pid,scope:'owned Chrome parent; no descendant-inventory claim'},null,2));
  watchdog=setTimeout(()=>{
    report.errors.push({kind:'watchdog',message:'150-second aggregate browser deadline reached'});
    void browserServer.kill().catch(e=>report.errors.push({kind:'watchdog-kill',message:e.message}));
  },150000);
  browser = await chromium.connect(browserServer.wsEndpoint(),{timeout:10000});
  report.environment.browser = browser.version();
  for (const width of [390,1440]) for (const theme of ['light','dark']) {
    const lane = width+'-'+theme, laneResult = {lane,width,height:1000,theme,variants:[]};
    const context = await browser.newContext({viewport:{width,height:1000},colorScheme:theme,reducedMotion:'reduce',serviceWorkers:'block',acceptDownloads:true});
    const pending = [];
    try {
      context.setDefaultTimeout(7000);
      await context.route('**/*', route => {
        const req=route.request(), u=new URL(req.url()), keys=[...u.searchParams.keys()];
        const fixture=(u.pathname===base||u.pathname===csvPath)&&keys.length===2&&new Set(keys).size===2&&keys.includes('theme')&&keys.includes('variant')&&u.searchParams.get('theme')===theme&&['normal','long'].includes(u.searchParams.get('variant'));
        if (u.origin===origin.origin&&['GET','HEAD'].includes(req.method())&&(fixture||(u.pathname==='/static/app.css'&&!u.search))) return route.continue();
        report.blocked.push({lane,url:req.url(),method:req.method()}); return route.abort();
      });
      await context.routeWebSocket('**/*', ws => {report.blocked.push({lane,websocket:ws.url()});ws.close();});
      const page=await context.newPage();
      page.on('pageerror',e=>report.errors.push({lane,kind:'page',message:e.message}));
      page.on('console',m=>{if(m.type()==='error')report.errors.push({lane,kind:'console',message:m.text()});});
      page.on('requestfailed',r=>{
        const u=new URL(r.url());
        if(u.origin===origin.origin&&u.pathname===csvPath&&r.failure()?.errorText==='net::ERR_ABORTED')report.downloadAborts.push({lane,url:r.url()});
        else report.errors.push({lane,kind:'request',url:r.url(),message:r.failure()?.errorText});
      });
      page.on('response',response=>{
        if(response.headers()['content-disposition']?.includes('attachment'))return;
        pending.push((async()=>{
          const bytes=await response.body(),u=new URL(response.url()),css=u.pathname==='/static/app.css',variant=u.searchParams.get('variant');
          const pin=css?manifest.inputs.css:manifest.responses[theme+'-'+variant+'-html'];
          report.requests.push({lane,url:u.href,status:response.status(),bytes:bytes.length,sha256:sha(bytes)});
          check(response.status()===200&&sha(bytes)===pin&&bytes.length<=(css?expected.css_max_bytes:expected.html_max_bytes),lane+': exact admitted '+(css?'CSS':'HTML')+' within byte budget',{url:u.href,bytes:bytes.length});
        })().catch(e=>report.errors.push({lane,kind:'response',message:e.message})));
      });
      await (await context.newCDPSession(page)).send('Emulation.setScriptExecutionDisabled',{value:true});
      await page.goto(origin.origin+base+'?variant=normal&theme='+theme,{waitUntil:'load'});
      for (const variant of ['normal','long']) {
        if(variant==='long') {
          await tabTo(page,'select[name="variant"]',lane+': Tab reaches native variant select');
          await page.keyboard.press('l'); await page.keyboard.press('Tab');
          check(await page.locator('select[name="variant"]').inputValue()==='long',lane+': native typeahead chooses long-label fallback');
          await tabTo(page,'form button',lane+': Tab reaches Apply');
          await Promise.all([page.waitForURL(u=>u.pathname===base&&u.searchParams.get('variant')==='long',{waitUntil:'load'}),page.keyboard.press('Enter')]);
        }
        const name=lane+'-'+variant;
        const dcl=await page.evaluate(()=>performance.getEntriesByType('navigation')[0]?.domContentLoadedEventEnd);
        report.timing.samples.push({lane,variant,ms:dcl});
        check(Number.isFinite(dcl)&&dcl>0,name+': DOM-ready timing observed');
        const metrics=await inspect(page,variant,name); laneResult.variants.push({variant,...metrics});
        const applied=await page.locator('[data-scale-specimen] table').evaluate(e=>({font:getComputedStyle(e).fontSize,align:getComputedStyle(e).textAlign,minWidth:getComputedStyle(e).minWidth,tableColor:getComputedStyle(e).color,bodyColor:getComputedStyle(document.body).backgroundColor}));
        check(applied.font==='14px'&&applied.align==='left'&&applied.minWidth==='512px',name+': selected CSS actually applies table utilities',applied);
        if(variant==='normal') {
          await tabTo(page,'[data-scale-specimen] [role="region"]:has(svg)',name+': Tab reaches chart region');
          await bringRow(page,'[data-scale-specimen] svg:not([data-analytics-legend-swatch]) text:first-of-type',name+': first chart label reachable');
          await capture(page,name+'-chart-first','[data-scale-specimen] svg:not([data-analytics-legend-swatch]) text:first-of-type');
        }
        await tableTask(page,name);
        completedDownloads.push(await downloadTask(page,context,variant,theme,name));
      }
    } catch(e) {
      check(false,lane+': task completes',e.stack);
    } finally {
      await bounded(Promise.allSettled(pending),10000,'Response body collection');
      const requests=report.requests.filter(r=>r.lane===lane);
      check(requests.some(r=>new URL(r.url).pathname==='/static/app.css')&&['normal','long'].every(v=>requests.some(r=>new URL(r.url).pathname===base&&new URL(r.url).searchParams.get('variant')===v)),lane+': exact CSS and both HTML responses were observed');
      await bounded(context.close(),8000,'Context close');
      report.lanes.push(laneResult); report.cases++;
    }
  }
  check(report.lanes.every(l=>l.variants.length===2),'All four lanes reached both variants');
  check(completedDownloads.length===8&&report.downloadAborts.every(a=>completedDownloads.includes(a.url)),'Every attachment navigation has verified saved bytes');
  check(report.blocked.length===0&&report.errors.length===0,'No external, write, socket or browser/resource failures',{blocked:report.blocked,errors:report.errors});
  const samples=report.timing.samples.map(s=>s.ms).filter(Number.isFinite).sort((a,b)=>a-b);
  report.timing.p95_ms=samples[Math.ceil(samples.length*.95)-1]??null;
  report.timing.within_diagnostic_budget=samples.length===8&&report.timing.p95_ms<=expected.dcl_p95_max_ms;
} catch(e) {
  report.errors.push({kind:'fatal',message:e.stack}); report.failures++;
  save('failure.json',JSON.stringify({manifest_sha256:manifestSHA,error:e.stack},null,2));
} finally {
  clearTimeout(watchdog);
  await closeOwnedBrowser();
  report.finished=new Date().toISOString();
  fs.writeFileSync(path.join(out,'report.json'),JSON.stringify(report,null,2),{flag:'wx',mode:0o600});
  console.log(JSON.stringify({out,cases:report.cases,checks:report.checks.length,failures:report.failures,errors:report.errors.length,report_sha256:sha(read('report.json'))}));
}
if(report.failures||report.errors.length)process.exitCode=1;
