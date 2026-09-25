// Explicit private P10 preparation. No installs, writes to sources, or conversion.
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {spawnSync} from 'node:child_process';
import {fileURLToPath, pathToFileURL} from 'node:url';
import {verifyHardenedReference} from '../catalyst-hardened/reference.mjs';

const here=path.dirname(fileURLToPath(import.meta.url));
const kit=path.resolve(here,'../../../..');
const hash=b=>crypto.createHash('sha256').update(b).digest('hex');
const check=(ok,message)=>{if(!ok)throw Error(message);};
const [project,base,parent]=process.argv.slice(2);
check(process.argv.length===5,'Use capture.mjs ABSOLUTE_PRIVATE_PROJECT EXACT_LOOPBACK_ORIGIN ABSOLUTE_PRIVATE_PARENT');
for(const p of [project,parent])check(path.isAbsolute(p)&&fs.lstatSync(p).isDirectory()&&!fs.lstatSync(p).isSymbolicLink(),'Select existing real absolute directories');
const origin=new URL(base);
check(origin.origin===base&&origin.hostname==='127.0.0.1'&&origin.protocol==='http:'&&origin.port,'Exact loopback origin with explicit port required');
const proof=verifyHardenedReference(project);
const modulePath=process.env.APPKIT_PLAYWRIGHT_MODULE;
check(modulePath&&path.isAbsolute(modulePath),'Select the already-approved isolated Playwright module');
const pkg=JSON.parse(fs.readFileSync(path.join(path.dirname(modulePath),'package.json')));
check(pkg.name==='playwright'&&pkg.version==='1.63.0','Pinned Playwright 1.63.0 required');
const originalArchive='94acb41796a8a97fdef44bf5b2d18f42919e43300cbca90077775f6b36dfa529';
const archivePath=path.join(kit,'data/design-assets/archives',originalArchive+'.zip');
const archiveStat=fs.lstatSync(archivePath);
check(archiveStat.isFile()&&!archiveStat.isSymbolicLink()&&archiveStat.size<=64*1024*1024,'Original archive unavailable or exceeds bound');
check(hash(fs.readFileSync(archivePath))===originalArchive,'Original archive drift');
const out=fs.mkdtempSync(path.join(parent,'p10-source-'));fs.chmodSync(out,0o700);
const put=(name,body)=>{check(/^[a-zA-Z0-9_.-]+$/.test(name)&&Buffer.byteLength(body)<=16*1024*1024,'Invalid output');const bytes=Buffer.from(body);fs.writeFileSync(path.join(out,name),bytes,{flag:'wx',mode:0o600});return {name,sha256:hash(bytes),bytes:bytes.length};};
const json=(name,value)=>put(name,JSON.stringify(value));
const plan={schema_version:1,id:'p10-hardened-rendered-reference-v1',source_manifest_sha256:proof.source_manifest_sha256,original_archive_sha256:originalArchive,
  collector_sha256:hash(fs.readFileSync(fileURLToPath(import.meta.url))),playwright:'1.63.0',chrome:'152.0.7977.84',
  css:{url:'/_next/static/chunks/0_7fu6wp96o5g.css',sha256:'67b5932dbb84baf67fe1143d065cb3c49c7b8d37be7f62823e8330875bfd603d'},
  matrix:[390,1440].flatMap(width=>['light','dark'].map(theme=>({width,theme}))),normalization:'none; raw hydrated form captures',verification:'not-verified'};
const planFile=json('capture-plan.json',plan);
console.log(JSON.stringify({out,plan_sha256:planFile.sha256,status:'source capture starting; not conversion'}));
const report={checks:[],errors:[],forbidden:[],responses:[],frames:[]};
const assertion=(name,ok,actual)=>{report.checks.push({name,ok,actual});check(ok,name);};
const pins=new Map(proof.files.map(f=>[f.path,f]));
const readPinned=relative=>{const pin=pins.get(relative);check(pin,'Unselected source input');const selected=path.join(project,relative);const info=fs.lstatSync(selected);check(info.isFile()&&!info.isSymbolicLink(),'Pinned input is not regular');const body=fs.readFileSync(selected);check(body.length===pin.size&&hash(body)===pin.sha256,'Pinned source input drift: '+relative);return body;};
// This exact source module was already admitted by the hardened-reference closure.
// Its getCountries export supplies demo options; it performs no service request.
const {getCountries}=await import(pathToFileURL(path.join(project,'src/data.ts')).href);
const countries=getCountries();
assertion('complete reviewed country set',countries.length===3&&countries.map(c=>c.code).join(',')==='CA,MX,US');
const countriesFile=json('countries.json',countries);
const assetFiles=[],assetByURL=new Map(),licenseFiles=[];
const addAsset=(url,body,mime)=>{check(!assetByURL.has(url),'Duplicate asset URL');const extension=mime==='text/css'?'css':mime==='image/svg+xml'?'svg':mime==='font/woff2'?'woff2':'woff';const file=put('asset-'+hash(body)+'.'+extension,body);const asset={url,file,mime};assetFiles.push(asset);assetByURL.set(url,asset);};
const cssFile=path.join(project,'.next',plan.css.url.slice('/_next/'.length));
const cssStat=fs.lstatSync(cssFile);check(cssStat.isFile()&&!cssStat.isSymbolicLink()&&cssStat.size<=4*1024*1024,'Selected CSS unavailable');
const css=fs.readFileSync(cssFile);check(hash(css)===plan.css.sha256,'Selected build CSS drift');addAsset(plan.css.url,css,'text/css');
for(const pin of proof.files){
  if(pin.path.startsWith('public/__appkit_reference_fonts/')){
    if(pin.path.endsWith('/LICENSE.txt')){licenseFiles.push(put('font-LICENSE.txt',readPinned(pin.path)));continue;}
    const ext=path.extname(pin.path);check(['.css','.woff2','.woff'].includes(ext),'Unexpected font closure file');
    addAsset('/'+pin.path.slice(7),readPinned(pin.path),ext==='.css'?'text/css':ext==='.woff2'?'font/woff2':'font/woff');
  }
}
for(const c of countries){check(/^\/flags\/[a-z]{2}\.svg$/.test(c.flagUrl),'Unexpected flag URL');addAsset(c.flagUrl,readPinned('public'+c.flagUrl),'image/svg+xml');}
check(assetFiles.length<=128,'Asset count exceeds bound');
const {chromium}=await import(pathToFileURL(modulePath).href);
let browser,reference;
try{
  browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
  assertion('pinned Chrome',browser.version()===plan.chrome,browser.version());
  let initialHTML,defaultsFile,currenciesFile,defaultFile,errorFile;
  for(const lane of plan.matrix){
    const context=await browser.newContext({viewport:{width:lane.width,height:1200},colorScheme:lane.theme,reducedMotion:'reduce',serviceWorkers:'block'});
    const pending=[];
    try{
      await context.route('**/*',route=>{const u=new URL(route.request().url());if(u.origin===base&&['GET','HEAD'].includes(route.request().method()))return route.continue();report.forbidden.push(u.href);return route.abort();});
      await context.routeWebSocket('**/*',socket=>{report.forbidden.push(socket.url());socket.close();});
      const page=await context.newPage();page.on('pageerror',e=>report.errors.push(e.message));
      page.on('response',response=>pending.push((async()=>{const u=new URL(response.url());check(response.status()===200,'Non-200 source response');const asset=assetByURL.get(u.pathname);if(asset){const body=await response.body();check(hash(body)===asset.file.sha256,'Rendered asset drift: '+u.pathname);}report.responses.push({path:u.pathname,status:response.status(),pinned:!!asset});})().catch(e=>report.errors.push(e.message))));
      await page.goto(base+'/settings');await page.waitForFunction(()=>document.querySelector('fieldset')&&!document.querySelector('fieldset').disabled);
      await page.evaluate(async()=>{await document.fonts.ready;});
      const snapshot=await page.locator('form').evaluate(form=>{const fd=new FormData(form),r=form.getBoundingClientRect();for(const key of fd.keys()){if(fd.getAll(key).length!==1)throw Error('Duplicate source form control: '+key);}return {html:form.outerHTML,headings:[...form.querySelectorAll('h1,h2,h3')].map(e=>e.textContent),values:Object.fromEntries(fd),currency:[...form.querySelector('select[name=currency]').options].map(o=>({code:o.value,label:o.textContent})),styles:[...document.querySelectorAll('link[rel=stylesheet]')].map(e=>new URL(e.href).pathname),geometry:{width:r.width,height:r.height,inner:innerWidth,scroll:document.documentElement.scrollWidth}};});
      assertion(`${lane.width}-${lane.theme}: complete sections`,snapshot.headings.join('|')==='Settings|Organization Name|Organization Bio|Organization Email|Address|Currency');
      assertion(`${lane.width}-${lane.theme}: all selected styles pinned`,snapshot.styles.length===2&&snapshot.styles.every(url=>assetByURL.has(url)),snapshot.styles);
      assertion(`${lane.width}-${lane.theme}: no page overflow`,snapshot.geometry.scroll<=snapshot.geometry.inner,snapshot.geometry);
      if(initialHTML===undefined){initialHTML=snapshot.html;defaultFile=put('reference-default.html',initialHTML);defaultsFile=json('defaults.json',snapshot.values);currenciesFile=json('currencies.json',snapshot.currency);}
      assertion(`${lane.width}-${lane.theme}: exact default markup`,snapshot.html===initialHTML,hash(snapshot.html));
      const shot=put(`default-${lane.width}-${lane.theme}.png`,await page.locator('form').screenshot());
      report.frames.push({...lane,screenshot:shot,geometry:snapshot.geometry});
      if(lane.width===390&&lane.theme==='light'){
        await page.getByLabel('Organization Bio',{exact:true}).fill('x'.repeat(241));
        await page.getByRole('button',{name:'Country',exact:true}).click();await page.getByRole('option',{name:'Mexico',exact:true}).click();
        const state=await page.locator('form').evaluate(f=>({html:f.outerHTML,bio:f.querySelector('[name=bio]').value,region:f.querySelector('[name=region]')?.value??'',country:f.querySelector('[name="country[code]"]').value}));
        assertion('error capture retains bio and clears incompatible region',state.bio.length===241&&state.region===''&&state.country==='MX');
        errorFile=put('reference-errors.html',state.html);
        put('errors-390-light.png',await page.locator('form').screenshot());
      }
    }finally{await Promise.all(pending);await context.close();}
  }
  verifyHardenedReference(project);
  assertion('no browser errors or forbidden requests',report.errors.length===0&&report.forbidden.length===0);
  reference={schema_version:1,id:plan.id,source_manifest_sha256:proof.source_manifest_sha256,original_archive_sha256:originalArchive,
    original_member:'catalyst-ui-kit/demo/typescript/src/app/(app)/settings/page.tsx',original_member_sha256:'1ca7f4e14738e917390815dc38d48047588eb324090d337f5c718b9d2ec6fa21',
    plan:planFile,default_html:defaultFile,error_html:errorFile,countries:countriesFile,defaults:defaultsFile,currencies:currenciesFile,assets:assetFiles,licenses:licenseFiles,frames:report.frames,
    collector_sha256:plan.collector_sha256,chrome:plan.chrome,playwright:plan.playwright,verification:'not-verified'};
}catch(e){report.errors.push(e.message);process.exitCode=1;}
finally{
  if(browser)try{await browser.close();}catch(e){report.errors.push('Browser cleanup: '+e.message);process.exitCode=1;}
  report.status=report.errors.length||report.forbidden.length?'failed':'captured-reference-only';json('capture-report.json',report);
  if(reference&&report.status!=='failed'){
    const file=json('reference.json',reference);
    const result=spawnSync('go',['tool','cue','vet','-d','#Reference',path.join(here,'reference.cue'),path.join(out,'reference.json')],{cwd:kit,env:{...process.env,GOWORK:'off',GOPROXY:'off'},encoding:'utf8',timeout:30000,maxBuffer:1024*1024});
    if(result.error||result.status!==0){process.exitCode=1;console.error(result.error?.message||result.stderr);console.log(JSON.stringify({out,status:'schema-refused; no usable reference claim'}));}
    else console.log(JSON.stringify({out,status:report.status,checks:report.checks.length,errors:report.errors,assets:assetFiles.length,reference_sha256:file.sha256,default_html_sha256:reference.default_html.sha256}));
  }else{process.exitCode=1;console.log(JSON.stringify({out,status:report.status,errors:report.errors}));}
}
