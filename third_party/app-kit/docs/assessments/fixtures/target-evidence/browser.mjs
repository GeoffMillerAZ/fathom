// Local Gallery navigation proof. Existing isolated Playwright/Chrome only.
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {pathToFileURL} from 'node:url';

const base = new URL(process.argv[2] || '');
if (base.hostname !== '127.0.0.1' || base.protocol !== 'http:' || base.pathname !== '/' || base.search || base.hash) throw Error('Exact loopback origin required');
const parent = fs.realpathSync(process.argv[3] || '');
const out = fs.mkdtempSync(path.join(parent, 'browser-')); fs.chmodSync(out, 0o700);
const sha = b => crypto.createHash('sha256').update(b).digest('hex');
const entry = process.env.APPKIT_PLAYWRIGHT_MODULE;
if (!entry || !path.isAbsolute(entry)) throw Error('Use the approved absolute Playwright module');
const {chromium} = await import(pathToFileURL(entry).href);
const report = {scope:'local Gallery historical-evidence task, not target certification', versions:{}, checks:[], errors:[], forbidden:[], screenshots:[]};
const record = (name, ok, actual) => {report.checks.push({name,ok,actual}); if (!ok) throw Error(name);};
const browser = await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
report.versions.chrome = browser.version();
report.versions.playwright = JSON.parse(fs.readFileSync(path.join(path.dirname(entry),'package.json'))).version;
try {
  record('pinned browser tools',report.versions.chrome==='152.0.7977.84'&&report.versions.playwright==='1.63.0',report.versions);
  for (const width of [390,1440]) for (const theme of ['light','dark']) {
    const key=`${width}-${theme}`;
    const context=await browser.newContext({viewport:{width,height:1000},javaScriptEnabled:false,colorScheme:theme,reducedMotion:'reduce',serviceWorkers:'block'});
    try {
      await context.route('**/*',route=>{const u=new URL(route.request().url());if(u.origin===base.origin&&route.request().method()==='GET')return route.continue();report.forbidden.push(u.href);return route.abort();});
      const page=await context.newPage();page.on('pageerror',e=>report.errors.push(e.message));
      const capture=async name=>{const body=await page.screenshot({fullPage:true});const file=`${key}-${name}.png`;fs.writeFileSync(path.join(out,file),body,{flag:'wx',mode:0o600});report.screenshots.push({file,sha256:sha(body)});};
      const tabTo=async locator=>{for(let i=0;i<64;i++){await page.keyboard.press('Tab');if(await locator.evaluate(e=>e===document.activeElement))return;}throw Error('Natural keyboard focus did not reach the requested link');};
      const activate=async locator=>{if(width===390){await tabTo(locator);await Promise.all([page.waitForNavigation(),page.keyboard.press('Enter')]);}else{await Promise.all([page.waitForNavigation(),locator.click()]);}};
      const initial=base.origin+`/target-evidence?subject_id=reference.navigation-adapter&limit=1&theme=${theme}`;
      await page.goto(initial);
      record(`${key}: two retained records`,await page.getByRole('heading',{name:'2 matching records',exact:true}).count()===1);
      record(`${key}: recorded boundary`,(await page.locator('main').innerText()).includes('Recorded metadata only'));
      const first=await page.getByRole('link',{name:/^Inspect evidence/}).getAttribute('href');
      await capture('index');
      await activate(page.getByRole('link',{name:'Next page',exact:true}));
      const second=await page.getByRole('link',{name:/^Inspect evidence/}).getAttribute('href');
      record(`${key}: second distinct page`,first.split('?')[0]!==second.split('?')[0]);
      await page.goBack();
      record(`${key}: Back restores selection`,page.url()===initial);
      await activate(page.getByRole('link',{name:/^Inspect evidence/}));
      record(`${key}: all nine gates`,await page.locator('section[aria-labelledby="evidence-gates"] dt').count()===9);
      record(`${key}: no certification`,(await page.getByRole('heading',{name:'Historical bytes match. Target compatibility not verified.',exact:true}).count())===1);
      const geometry=await page.locator('main').evaluate(e=>({inner:innerWidth,scroll:document.documentElement.scrollWidth,targets:[...e.querySelectorAll('a,button,summary')].filter(e=>e.checkVisibility()).map(e=>{const r=e.getBoundingClientRect();return {text:e.textContent.trim(),width:r.width,height:r.height};})}));
      record(`${key}: no page overflow`,geometry.scroll<=geometry.inner,geometry);
      record(`${key}: new visible controls at least44px`,geometry.targets.every(t=>t.width>=44&&t.height>=44),geometry.targets);
      await capture('detail');
      await activate(page.getByRole('link',{name:'Open original reference',exact:true}));
      record(`${key}: independent original`,new URL(page.url()).pathname.startsWith('/references/')&&await page.locator('#reference-source').count()===1);
      await page.goBack();
      record(`${key}: Back restores evidence`,new URL(page.url()).pathname===new URL(first,base).pathname);
      await page.goto(initial);await page.getByLabel('Target profile',{exact:true}).selectOption('go-web');
      await activate(page.getByRole('button',{name:'Find evidence',exact:true}));
      record(`${key}: empty filter state`,await page.getByRole('heading',{name:'No evidence records match',exact:true}).count()===1);
      await activate(page.getByRole('link',{name:'Reset filters',exact:true}));
      record(`${key}: reset preserves theme`,new URL(page.url()).searchParams.get('theme')===theme);
      await page.goto(base.origin+`/target-evidence?limit=0&theme=${theme}`);
      record(`${key}: invalid query recovery`,await page.getByRole('heading',{name:'Reset evidence filters',exact:true}).count()===1);
      await activate(page.getByRole('link',{name:'Return to target evidence',exact:true}));
      record(`${key}: recovery usable`,await page.getByRole('heading',{name:'Target evidence',exact:true}).count()===1);
    } finally {await context.close();}
  }
} catch(e) {report.errors.push(String(e.message));process.exitCode=1;}
finally {
  await browser.close();
  report.status=report.errors.length||report.forbidden.length?'failed':'passed-scoped';
  const body=Buffer.from(JSON.stringify(report,null,2));fs.writeFileSync(path.join(out,'report.json'),body,{flag:'wx',mode:0o600});
  console.log(JSON.stringify({out,status:report.status,checks:report.checks.length,errors:report.errors,forbidden:report.forbidden,sha256:sha(body)}));
  if(report.status==='failed')process.exitCode=1;
}
