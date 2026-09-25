// First-party adversarial/lifecycle probes against an explicitly selected local
// private form. No paid markup, network service, or DOM library is embedded here.
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {fileURLToPath, pathToFileURL} from 'node:url';

const [base, parent] = process.argv.slice(2);
const assert = (ok, message) => { if (!ok) throw Error(message); };
const origin = new URL(base);
assert(origin.origin === base && origin.protocol === 'http:' && origin.hostname === '127.0.0.1' && origin.port, 'Explicit loopback origin required');
assert(path.isAbsolute(parent) && fs.lstatSync(parent).isDirectory() && !fs.lstatSync(parent).isSymbolicLink(), 'Explicit private output parent required');
const modulePath = process.env.APPKIT_PLAYWRIGHT_MODULE;
assert(modulePath && path.isAbsolute(modulePath), 'Select approved isolated Playwright');
const pkg = JSON.parse(fs.readFileSync(path.join(path.dirname(modulePath), 'package.json')));
assert(pkg.name === 'playwright' && pkg.version === '1.63.0', 'Pinned Playwright required');
const hash = b => crypto.createHash('sha256').update(b).digest('hex');
const out = fs.mkdtempSync(path.join(parent, 'p10-seam-browser-'));
fs.chmodSync(out, 0o700);
const put = (name, body) => { const b = Buffer.from(body); fs.writeFileSync(path.join(out, name), b, {flag:'wx', mode:0o600}); return {name, sha256:hash(b), bytes:b.length}; };
const cases = ['missing config', 'malformed JSON', 'duplicate JSON members', 'duplicate config', 'duplicate field ID', 'duplicate counter ID', 'external counter', 'duplicate reset ID', 'oversized initial', 'unpaired surrogate', 'invalid initial currency', 'missing country group', 'duplicate canonical region', 'wrong config target'];
const plan = {schema_version:1, fixture:'p10-enhancement-seam', runner_sha256:hash(fs.readFileSync(fileURLToPath(import.meta.url))), chrome:'152.0.7977.84', playwright:'1.63.0', cases,
  checks:['bad seam refuses without DOM mutation', 'two independent instances', 'idempotent mount/dispose', 'hostile initial snapshot is text', 'invalid server draft survives mount/dispose', 'correcting unknown country/currency permits remount', 'no transient-state requests'],
  exclusions:['source fidelity', 'physical input', 'actual zoom', 'performance budgets', 'full accessibility', 'consumer adoption'], verification:'not-verified'};
const planFile = put('plan.json', JSON.stringify(plan));
console.log(JSON.stringify({out, plan_sha256:planFile.sha256, status:'seam tasks starting'}));
const report = {checks:[], errors:[], forbidden:[], requests:[]};
const check = (name, ok, actual) => { report.checks.push({name, ok, actual}); assert(ok, name); };
const {chromium} = await import(pathToFileURL(modulePath).href);
let browser;
const deadline = setTimeout(() => { report.errors.push('120 second collector deadline exceeded'); process.exitCode=1; void browser?.close(); }, 120_000);
try {
  browser = await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless:true});
  check('pinned Chrome', browser.version() === plan.chrome, browser.version());
  const context = await browser.newContext({viewport:{width:1440,height:1200}, serviceWorkers:'block', reducedMotion:'reduce'});
  await context.route('**/*', route => { const req=route.request(), u=new URL(req.url()); if (u.origin===base && ['GET','POST','HEAD'].includes(req.method())) return route.continue(); report.forbidden.push(u.href); return route.abort(); });
  const page = await context.newPage();
  page.setDefaultTimeout(10_000);
  page.on('pageerror', e => report.errors.push(e.message));
  page.on('request', r => report.requests.push({path:new URL(r.url()).pathname, method:r.method()}));
  const fresh = async () => {
    const response = await page.goto(base+'/light/enhanced/settings');
    assert(response.status()===200, 'Initial HTTP 200 required');
    await page.locator('form[data-p10-enhanced=true]').waitFor();
    await page.evaluate(async () => {
      const module = await import('/controller.mjs');
      module.mountSettings(document.querySelector('form[data-p10-settings]')).dispose();
      await document.fonts.ready;
    });
  };
  for (const name of cases) {
    await fresh();
    const result = await page.evaluate(async name => {
      const {mountSettings} = await import('/controller.mjs');
      const form = document.querySelector('form[data-p10-settings]');
      const template = document.querySelector('template[data-p10-config-for]');
      const config = JSON.parse(template.content.textContent);
      const setConfig = () => template.content.replaceChildren(document.createTextNode(JSON.stringify(config)));
      switch (name) {
        case 'missing config': template.remove(); break;
        case 'malformed JSON': template.content.replaceChildren(document.createTextNode('{')); break;
        case 'duplicate JSON members': template.content.replaceChildren(document.createTextNode(JSON.stringify(config).replace('"bio_limit":240', '"bio_limit":1,"bio_limit":240'))); break;
        case 'duplicate config': template.after(template.cloneNode(true)); break;
        case 'duplicate field ID': { const el=document.createElement('span');el.id=form.querySelector('[name=name]').id;document.body.prepend(el);break; }
        case 'duplicate counter ID': { const el=document.getElementById('p10-settings-bio-count').cloneNode(true);document.body.prepend(el);break; }
        case 'external counter': document.body.append(document.getElementById('p10-settings-bio-count')); break;
        case 'duplicate reset ID': { const el=document.getElementById('p10-settings-reset-form');el.after(el.cloneNode(true));break; }
        case 'oversized initial': config.initial.name='x'.repeat(513);setConfig();break;
        case 'unpaired surrogate': config.initial.name='\uD800';setConfig();break;
        case 'invalid initial currency': { const option=document.createElement('option');option.value='ZZZ';option.textContent='ZZZ (invalid)';option.setAttribute('data-p10-invalid','');form.querySelector('[name=currency]').append(option);config.initial.currency='ZZZ';setConfig();break; }
        case 'missing country group': { const option=document.createElement('option');option.value='ZZ';option.textContent='Unconfigured';form.querySelector('[name="country[code]"]').append(option);break; }
        case 'duplicate canonical region': { const group=form.querySelector('optgroup');group.append(group.querySelector('option').cloneNode(true));break; }
        case 'wrong config target': template.setAttribute('data-p10-config-for','another-form');break;
      }
      const before=document.body.innerHTML;
      const handle=mountSettings(form);
      return {refused:handle===null, unchanged:document.body.innerHTML===before};
    }, name);
    check(name+': native fallback untouched', result.refused && result.unchanged, result);
  }
  await fresh();
  let result = await page.evaluate(async () => {
    const {mountSettings} = await import('/controller.mjs');
    const original=document.querySelector('main');
    const copy=original.cloneNode(true);
    for (const el of [copy,...copy.querySelectorAll('*')]) for (const attr of [...el.attributes]) {
      if (['id','for','form','aria-describedby','data-p10-config-for'].includes(attr.name)) el.setAttribute(attr.name,attr.value.split('p10-settings').join('p10-second'));
    }
    original.after(copy);
    const first=original.querySelector('form'),second=copy.querySelector('form');
    const a=mountSettings(first), b=mountSettings(second);
    if (!a||!b) return {mounted:false};
    const sameHandle=mountSettings(first)===a;
    const secondBefore=second.outerHTML;
    const bio=first.querySelector('[name=bio]');bio.value='😀'.repeat(241);bio.dispatchEvent(new Event('input',{bubbles:true}));
    const independent=second.outerHTML===secondBefore;
    const region=first.querySelector('[name=region]');region.focus();const value=region.value;
    a.dispose();a.dispose();
    const disposed=first.querySelector('[name=region]')===region && document.activeElement===region && region.value===value && region.querySelectorAll('optgroup').length===3;
    const c=mountSettings(first);
    return {mounted:true,sameHandle,independent,disposed,remounted:!!c&&c!==a&&mountSettings(first)===c,bioRetained:bio.value==='😀'.repeat(241)};
  });
  check('independent instances and lifecycle', Object.values(result).every(Boolean), result);
  await fresh();
  const requestCount=report.requests.length;
  result = await page.evaluate(async () => {
    const {mountSettings} = await import('/controller.mjs');
    const form=document.querySelector('form[data-p10-settings]'), template=document.querySelector('template[data-p10-config-for]');
    const config=JSON.parse(template.content.textContent);
    const hostile='</template><script>alert(1)</script> & " \u2028\u2029';
    config.initial.name=hostile;
    template.content.replaceChildren(document.createTextNode(JSON.stringify(config)));
    const handle=mountSettings(form);
    if (!handle) return {mounted:false};
    const reset=form.querySelector('button[value=reset]');reset.focus();reset.click();
    return {mounted:true,exact:form.querySelector('[name=name]').value===hostile,sameFocus:document.activeElement===reset,noInjection:document.querySelectorAll('script').length===1,unchangedURL:location.pathname==='/light/enhanced/settings'};
  });
  check('hostile reset snapshot stays exact text',Object.values(result).every(Boolean),result);
  check('local hostile reset has no request',report.requests.length===requestCount,report.requests.slice(requestCount));
  await fresh();
  // Use the real native request to create an inconsistent retained draft, not a
  // fabricated validity badge. Enhancement must not silently fix submitted data.
  await page.locator('[name="country[code]"]').selectOption('MX');
  await page.locator('[name=region]').selectOption('Ontario');
  const [response]=await Promise.all([page.waitForNavigation(),page.getByRole('button',{name:'Validate changes',exact:true}).click()]);
  check('server refuses mismatched region',response.status()===422,response.status());
  await page.locator('form[data-p10-enhanced=true]').waitFor();
  result=await page.evaluate(async()=>{
    const {mountSettings}=await import('/controller.mjs');const form=document.querySelector('form[data-p10-settings]'),region=form.querySelector('[name=region]');
    const mountedDraft=region.value;const country=form.querySelector('[name="country[code]"]').value;
    region.focus();mountSettings(form).dispose();
    return {country, mountedDraft,disposedDraft:region.value,focus:document.activeElement===region,groups:region.querySelectorAll('optgroup').length,error:form.querySelector('[data-p10-error=region]').hidden===false};
  });
  check('invalid draft survives mount and disposal',result.country==='MX'&&result.mountedDraft==='Ontario'&&result.disposedDraft==='Ontario'&&result.focus&&result.groups===3&&result.error,result);
  await fresh();
  await page.evaluate(()=>{
    for (const name of ['country[code]','currency']) {
      const select=document.getElementsByName(name)[0];
      select.append(new Option('Unknown draft','ZZZ'));select.value='ZZZ';
    }
  });
  const [unknownResponse]=await Promise.all([page.waitForNavigation(),page.getByRole('button',{name:'Validate changes',exact:true}).click()]);
  check('server refuses unknown country/currency',unknownResponse.status()===422,unknownResponse.status());
  await page.locator('form[data-p10-enhanced=true]').waitFor();
  const beforeUnknownReset=report.requests.length;
  await page.getByRole('button',{name:'Reset',exact:true}).click();
  result=await page.evaluate(async()=>{
    const {mountSettings}=await import('/controller.mjs');const form=document.querySelector('form[data-p10-settings]');
    const before=Object.fromEntries(new FormData(form));const first=mountSettings(form);first.dispose();const next=mountSettings(form);
    return {remounted:!!next, retained:JSON.stringify(before)===JSON.stringify(Object.fromEntries(new FormData(form))), country:before['country[code]'],currency:before.currency};
  });
  check('correcting unknown options permits remount without requests',result.remounted&&result.retained&&result.country==='CA'&&result.currency==='cad'&&report.requests.length===beforeUnknownReset,result);
  check('no exceptions or forbidden requests',report.errors.length===0&&report.forbidden.length===0);
  await context.close();
} catch (e) { report.errors.push(e.stack||e.message); process.exitCode=1; }
finally {
  clearTimeout(deadline);
  if (browser) try { await browser.close(); } catch(e) { report.errors.push(e.message);process.exitCode=1; }
  report.status=report.errors.length||report.forbidden.length?'failed':'scoped-seam-tasks-passed';
  const result=put('report.json',JSON.stringify(report));
  console.log(JSON.stringify({out,status:report.status,checks:report.checks.length,errors:report.errors,report_sha256:result.sha256}));
}
