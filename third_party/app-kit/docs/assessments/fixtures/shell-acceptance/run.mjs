// Opt-in, freeze-first launcher for the existing Gallery shell browser probes.
// This deliberately records a selected input closure, not a complete Go build closure.
import crypto from 'node:crypto';
import fs from 'node:fs';
import http from 'node:http';
import net from 'node:net';
import path from 'node:path';
import process from 'node:process';
import {spawn} from 'node:child_process';

const rel = 'docs/assessments/fixtures/shell-acceptance/run.mjs';
const maxLog = 8 * 1024 * 1024, maxArtifact = 64 * 1024 * 1024;
const selected = [
  'go.mod', 'go.sum', rel,
  'apps/gallery/cmd/gallery/main.go', 'apps/gallery/cmd/gallery/main_test.go', 'apps/gallery/cmd/gallery/theme_source.go',
  'apps/gallery/gallery.go', 'apps/gallery/shell_demo.go', 'apps/gallery/shell_demo_model.go',
  'apps/gallery/shell_demo_render.go', 'apps/gallery/shell_demo_templates.go', 'apps/gallery/shell_demo.js',
  'apps/gallery/shell_demo_test.go', 'apps/gallery/shell_demo_test.mjs', 'apps/gallery/shell_demo_live_test.go',
  'apps/gallery/shell_demo_render_test.go', 'apps/gallery/shell_demo_store_test.go',
  'apps/gallery/ui/input.css', 'apps/gallery/ui/app.css',
  'apps/gallery/browser/config.mjs', 'apps/gallery/browser/shell_forms.mjs', 'apps/gallery/browser/settings_sections.mjs',
  'appkit/ui/dashboard/account.go', 'appkit/ui/dashboard/account_test.go', 'appkit/ui/dashboard/dashboard.go', 'appkit/ui/dashboard/dashboard_test.go',
  'appkit/ui/dashboard/nav.go', 'appkit/ui/dashboard/nav_test.go', 'appkit/ui/dashboard/shell.go', 'appkit/ui/dashboard/shell_test.go', 'appkit/ui/dashboard/widget.go',
  'appkit/ui/form/form.go', 'appkit/ui/form/form_test.go', 'appkit/ui/form/settings.go', 'appkit/ui/form/settings_test.go', 'appkit/ui/form/settings_browser_test.go',
  'appkit/ui/appearance/appearance.go', 'appkit/ui/appearance/appearance.js', 'appkit/ui/appearance/appearance_test.go', 'appkit/ui/appearance/appearance_test.mjs',
  'appkit/catalog/catalog.cue', 'appkit/catalog/schema.cue', 'appkit/catalog/catalog.go', 'appkit/catalog/catalog_test.go',
  'appkit/catalog/adapter_inventory_test.go', 'appkit/catalog/elements_inventory_test.go', 'appkit/catalog/faq_inventory_test.go',
  'appkit/catalog/hero_inventory_test.go', 'appkit/catalog/inventory_test.go', 'appkit/catalog/selection_eval_test.go',
  'appkit/catalog/shell_workflow_test.go', 'appkit/catalog/styleguide_test.go', 'internal/catalogtest/testdata/selection.json'
];
const sha = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const fail = message => { throw new Error(message); };
const mkdir = dir => fs.mkdirSync(dir, {recursive:true, mode:0o700});
const realDir = value => { const p=fs.realpathSync(value); if (!fs.lstatSync(p).isDirectory()) fail(`not a directory: ${value}`); return p; };
const regular = value => { const p=fs.realpathSync(value), s=fs.lstatSync(p); if (!s.isFile() || s.isSymbolicLink()) fail(`not a regular file: ${value}`); return p; };
const inside = (root, value) => { const p=path.resolve(value); if (p!==root && !p.startsWith(root+path.sep)) fail(`path escapes owned root: ${value}`); return p; };
function put(file, bytes) { mkdir(path.dirname(file)); fs.writeFileSync(file, bytes, {flag:'wx', mode:0o600}); }
function json(file, value) { put(file, Buffer.from(JSON.stringify(value, null, 2)+'\n')); return identity(file); }
function identity(file) { const p=regular(file), bytes=fs.readFileSync(p); return {path:p, bytes:bytes.length, sha256:sha(bytes)}; }
function snapshot(root, out) {
  const files={};
  for (const name of selected) {
    const source=regular(path.join(root,name)), bytes=fs.readFileSync(source), target=path.join(out,name);
    put(target, bytes); files[name]={bytes:bytes.length,sha256:sha(bytes)};
  }
  return files;
}
function same(root, files) {
  for (const [name, want] of Object.entries(files)) {
    const got=identity(path.join(root,name));
    if (got.bytes!==want.bytes || got.sha256!==want.sha256) fail(`selected source changed: ${name}`);
  }
}
function sameConfig(root) {
  const want=fs.readFileSync(path.join(root,'run-config.sha256'),'utf8').trim(), got=identity(path.join(root,'run-config.json')).sha256;
  if (want!==got) fail('frozen run-config changed');
  return got;
}
function sameIdentity(want) { const got=identity(want.path); if(got.bytes!==want.bytes||got.sha256!==want.sha256) fail(`frozen identity changed: ${want.path}`); return got; }
function envFor(config, root) {
  const tmp=path.join(root,'tmp'); mkdir(tmp);
  const env={...process.env}; for (const key of Object.keys(env)) if(key.startsWith('GIT_')) delete env[key];
  return {...env, GOFLAGS:'', GOWORK:'off', GOENV:'off', GOTOOLCHAIN:'local', GOPROXY:'off', GOSUMDB:'off',
    GOCACHE:config.goCache, TMPDIR:tmp, GOTMPDIR:tmp};
}
function command(root, name, bin, args, env, timeout=180000, cwd=root) {
  const logs=path.join(root,'commands',name); mkdir(logs);
  return new Promise(resolve => {
    const child=spawn(bin,args,{cwd,env,detached:true,stdio:['ignore','pipe','pipe']});
    let stdout=Buffer.alloc(0), stderr=Buffer.alloc(0), overflow=false, timedOut=false, spawnError='';
    const stop=signal=>{try { if(child.pid) process.kill(-child.pid,signal); } catch {}};
    const take=(which, chunk) => { const next=Buffer.concat([which,chunk]); if(next.length>maxLog){overflow=true;stop('SIGKILL');return next.subarray(0,maxLog);} return next; };
    child.stdout.on('data', c=>{stdout=take(stdout,c);}); child.stderr.on('data', c=>{stderr=take(stderr,c);});
    let completed=false; const finish=(code,signal)=>{if(completed)return;completed=true;clearTimeout(timer);const out=path.join(logs,'stdout.log'),err=path.join(logs,'stderr.log');
      put(out,stdout);put(err,stderr);resolve({name,bin,args,timeout,code,signal,timedOut,overflow,spawnError,stdout:identity(out),stderr:identity(err)});};
    const timer=setTimeout(()=>{timedOut=true; stop('SIGKILL');},timeout);
    child.once('error',error=>{spawnError=String(error.stack||error);finish(null,null);}); child.on('close',finish);
  });
}
function requireOK(result) { if (result.code!==0 || result.signal || result.timedOut || result.overflow || result.spawnError) fail(`${result.name} failed`); }
function lastJSON(bytes) { const text=fs.readFileSync(bytes,'utf8'), at=text.lastIndexOf('\n{'); const raw=(at<0?text:text.slice(at+1)).trim(); return JSON.parse(raw); }
function walk(root, out=[]) { for (const item of fs.readdirSync(root,{withFileTypes:true})) { const p=path.join(root,item.name); if(item.isDirectory()) walk(p,out); else if(item.isFile()&&!item.isSymbolicLink()) out.push(identity(p)); else fail(`non-regular artifact: ${p}`); } return out; }
function artifacts(root, reportPath) { const dir=path.dirname(regular(reportPath)); inside(root,dir); const all=walk(dir); if(!all.length||all.length>256||all.reduce((n,x)=>n+x.bytes,0)>maxArtifact) fail(`artifact bounds failed: ${dir}`); return all; }
function launchGallery(root, binary, env) {
  const logs=path.join(root,'commands','gallery-server'); mkdir(logs);
  const child=spawn(binary,['-addr','127.0.0.1:0','-design-assets',''],{cwd:root,env,detached:true,stdio:['ignore','pipe','pipe']});
  const state={child,text:'',stdout:Buffer.alloc(0),stderr:Buffer.alloc(0),overflow:false,spawnError:'',logs};
  const stop=()=>{try {if(child.pid) process.kill(-child.pid,'SIGKILL');} catch {}};
  const take=(key, chunk) => { const next=Buffer.concat([state[key],chunk]); if(next.length>maxLog){state.overflow=true;stop();return next.subarray(0,maxLog);} return next; };
  const note=chunk=>{state.text=(state.text+String(chunk)).slice(-32768);};
  child.stdout.on('data',c=>{state.stdout=take('stdout',c);note(c);}); child.stderr.on('data',c=>{state.stderr=take('stderr',c);note(c);});
  child.once('error',error=>{state.spawnError=String(error.stack||error);});
  return state;
}
function retainGalleryLogs(state) { const out=path.join(state.logs,'stdout.log'), err=path.join(state.logs,'stderr.log'); put(out,state.stdout); put(err,state.stderr); return {stdout:identity(out),stderr:identity(err),overflow:state.overflow}; }
async function waitOrigin(state, timeout=15000) {
  const child=state.child;
  const until=Date.now()+timeout;
  while(Date.now()<until) { const found=state.text.match(/http:\/\/127\.0\.0\.1:\d+/); if(found) return found[0]; if(state.spawnError||child.exitCode!==null||child.signalCode!==null) break; await new Promise(r=>setTimeout(r,50)); }
  fail(`Gallery did not report loopback origin: ${state.spawnError||state.text.slice(-2000)}`);
}
async function portRefused(origin) { const url=new URL(origin); return await new Promise(resolve=>{ const s=net.connect({host:url.hostname,port:Number(url.port)}), done=value=>{clearTimeout(timer);s.destroy();resolve(value);}, timer=setTimeout(()=>done(false),2000); s.once('error',error=>done(error.code==='ECONNREFUSED')); s.once('connect',()=>done(false)); }); }
async function stopGallery(state, origin) {
  const child=state.child;
  const terminal=()=>child.exitCode!==null||child.signalCode!==null;
  if(!terminal()&&!state.spawnError) child.kill('SIGTERM');
  let timer;
  const done=terminal() ? {code:child.exitCode,signal:child.signalCode} : state.spawnError ? {code:null,signal:null,spawnError:state.spawnError} : await Promise.race([new Promise(r=>child.once('close',(code,signal)=>r({code,signal}))),new Promise(r=>{timer=setTimeout(()=>r(null),10000);})]);
  clearTimeout(timer);
  if(!done) { try { process.kill(-child.pid,'SIGKILL'); } catch {} retainGalleryLogs(state); fail('Gallery cleanup timed out'); }
  const refused=origin ? await portRefused(origin) : null, logs=retainGalleryLogs(state);
  if(done.code!==0 || done.signal || (origin&&!refused) || logs.overflow) fail(`Gallery cleanup failed: ${JSON.stringify({done,refused,overflow:logs.overflow})}`);
  return {...done, refused, logs};
}
function getCSS(origin) { return new Promise((resolve,reject)=>{const req=http.get(origin+'/static/app.css',res=>{const parts=[];res.on('data',c=>parts.push(c));res.on('end',()=>resolve({status:res.statusCode,bytes:Buffer.concat(parts)}));}); req.setTimeout(10000,()=>req.destroy(new Error('served CSS timed out'))); req.on('error',reject);}); }
function browserCheck(root, commandResult, kind) {
  const output=lastJSON(commandResult.stdout.path), report=regular(inside(root,output.report));
  const parsed=JSON.parse(fs.readFileSync(report,'utf8'));
  const outputOK=Number.isInteger(output.passes)&&Array.isArray(output.failures)&&typeof output.report==='string';
  const ok=commandResult.code===0&&!commandResult.signal&&!commandResult.timedOut&&!commandResult.overflow&&!commandResult.spawnError&&outputOK&&Array.isArray(parsed.failures)&&parsed.failures.length===0&&Array.isArray(parsed.passes)&&parsed.passes.length>0&&(kind!=='settings'||output.result==='pass');
  return {ok,stdout:output,report:identity(report),artifacts:artifacts(root,report)};
}
function parseArgs(argv) {
  if(argv[0]==='--frozen' && argv.length===2) return {frozen:realDir(argv[1])};
  if(argv.length!==3) fail('usage: node run.mjs <absolute-app-kit-root> <absolute-playwright-module> <absolute-private-go-cache>');
  return {appKitRoot:realDir(argv[0]), playwright:regular(argv[1]), goCache:realDir(argv[2])};
}
async function frozen(root) {
  const config=JSON.parse(fs.readFileSync(path.join(root,'run-config.json'),'utf8')), report={schema:1, selectedClosure:'selected inputs only; not a complete Go build closure', startedAt:new Date().toISOString(), commands:[]};
  let gallery, origin;
  try {
    report.config={sha256:sameConfig(root)}; sameIdentity(config.playwright); sameIdentity(config.browser); same(config.appKitRoot,config.sources); same(path.join(root,'source-snapshot'),config.sources);
    const env=envFor(config,root), css=path.join(root,'css','app.css'), fixture=path.join(root,'fixture'); mkdir(path.dirname(css)); mkdir(fixture);
    const toolchain=await command(root,'go-version','go',['version'],env,10000,config.appKitRoot); report.commands.push(toolchain); requireOK(toolchain);
    const cssRun=await command(root,'css','go',['run','-mod=readonly','./cmd/appkit','css','-i','apps/gallery/ui/input.css','-o',css,'--minify'],env,180000,config.appKitRoot); report.commands.push(cssRun); requireOK(cssRun);
    const overlay=path.join(root,'css-overlay.json'); json(overlay,{Replace:{[path.join(config.appKitRoot,'apps/gallery/ui/app.css')]:css}});
    const build=await command(root,'gallery-build','go',['build','-mod=readonly','-overlay',overlay,'-o',path.join(root,'gallery'),'./apps/gallery/cmd/gallery'],env,180000,config.appKitRoot); report.commands.push(build); requireOK(build);
    const exportRun=await command(root,'settings-fixture','go',['test','-mod=readonly','./appkit/ui/form','-run','^TestExportSettingsSectionsFixture$','-count=1','-v'],{...env,APPKIT_PROBE_SETTINGS_SECTIONS_OUT:fixture},180000,config.appKitRoot); report.commands.push(exportRun); requireOK(exportRun);
    const binary=identity(path.join(root,'gallery')), expectedCSS=identity(css), fixtureFile=identity(path.join(fixture,'settings-sections.html'));
    gallery=launchGallery(root,binary.path,{...env,APPKIT_DESIGN_ASSETS:''}); origin=await waitOrigin(gallery);
    const served=await getCSS(origin); if(served.status!==200||sha(served.bytes)!==expectedCSS.sha256) fail('served Gallery CSS differs from private generated CSS');
    const browserEnv={...env,APPKIT_SHELL_BROWSER_ORIGIN:origin,APPKIT_PLAYWRIGHT_MODULE:config.playwright.path,APPKIT_BROWSER_EXECUTABLE:config.browser.path,APPKIT_BROWSER_VARIANT:'shell-acceptance',APPKIT_SETTINGS_FIXTURE:fixtureFile.path};
    const forms=await command(root,'shell-forms',process.execPath,[path.join(root,'source-snapshot','apps/gallery/browser/shell_forms.mjs')],browserEnv,210000); report.commands.push(forms); report.forms=browserCheck(root,forms,'forms'); if(!report.forms.ok) fail('shell-forms report failed');
    const settings=await command(root,'settings-sections',process.execPath,[path.join(root,'source-snapshot','apps/gallery/browser/settings_sections.mjs')],browserEnv,210000); report.commands.push(settings); report.settings=browserCheck(root,settings,'settings'); if(!report.settings.ok) fail('settings-sections report failed');
    report.binary=binary; report.css={generated:expectedCSS,served:{bytes:served.bytes.length,sha256:sha(served.bytes)}}; report.fixture=fixtureFile;
    if(identity(path.join(root,'gallery')).sha256!==binary.sha256 || identity(css).sha256!==expectedCSS.sha256) fail('private build output changed during browser probes');
    sameConfig(root); sameIdentity(config.playwright); sameIdentity(config.browser); same(config.appKitRoot,config.sources); same(path.join(root,'source-snapshot'),config.sources); report.passed=true;
  } catch (error) { report.passed=false; report.error=String(error.stack||error); }
  finally { if(gallery) { try { report.cleanup=await stopGallery(gallery,origin); } catch(error) { report.passed=false; report.cleanupError=String(error.stack||error); } } }
  report.finishedAt=new Date().toISOString(); json(path.join(root,'run-report.json'),report); if(!report.passed) process.exitCode=1;
}
async function main() {
  const args=parseArgs(process.argv.slice(2)); if(args.frozen) return frozen(args.frozen);
  const parent=realDir(process.env.APPKIT_SHELL_ACCEPTANCE_EVIDENCE_ROOT||'/Volumes/m2ext');
  const root=fs.mkdtempSync(path.join(parent,'appkit-shell-acceptance-')); fs.chmodSync(root,0o700);
  const snapshotRoot=path.join(root,'source-snapshot'), sources=snapshot(args.appKitRoot,snapshotRoot);
  const browser=identity(regular(process.env.APPKIT_BROWSER_EXECUTABLE||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'));
  const config={schema:1,appKitRoot:args.appKitRoot,playwright:identity(args.playwright),browser,goCache:args.goCache,sources,runner:rel,createdAt:new Date().toISOString()}; const configIdentity=json(path.join(root,'run-config.json'),config); put(path.join(root,'run-config.sha256'),Buffer.from(configIdentity.sha256+'\n'));
  // Exceed the sum of all bounded inner commands plus startup and cleanup, so
  // the launcher cannot preempt the retained runner's owned-process cleanup.
  const child=await command(root,'retained-runner',process.execPath,[path.join(snapshotRoot,rel),'--frozen',root],process.env,1200000);
  json(path.join(root,'launcher-result.json'),child); if(child.code!==0||child.signal||child.timedOut||child.overflow||child.spawnError) process.exitCode=1;
  console.log(JSON.stringify({root,report:path.join(root,'run-report.json'),launcher:child}));
}
await main();
