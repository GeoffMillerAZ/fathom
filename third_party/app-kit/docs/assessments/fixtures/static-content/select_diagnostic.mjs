#!/usr/bin/env node
import {spawn} from 'node:child_process';
import {createHash} from 'node:crypto';
import fs from 'node:fs/promises';
import net from 'node:net';
import path from 'node:path';
import {fileURLToPath,pathToFileURL} from 'node:url';

const approvedPlaywrightSHA='a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f';
const sha256=bytes=>createHash('sha256').update(bytes).digest('hex');
const bounded=(promise,label,ms=8000)=>Promise.race([promise,new Promise((_,reject)=>setTimeout(()=>reject(Error(`${label} timeout`)),ms))]);

function allowedRequest(origin,request){
  let url;try{url=new URL(request.url());}catch{return false;}
  if(url.origin!==origin||request.method()!=='GET')return false;
  if(url.pathname==='/static/app.css')return url.search==='';
  if(url.pathname!=='/content')return false;
  for(const key of url.searchParams.keys())if(!['theme','state'].includes(key))return false;
  if(url.searchParams.getAll('theme').length>1||url.searchParams.getAll('state').length>1)return false;
  const theme=url.searchParams.get('theme'),state=url.searchParams.get('state');
  return(theme===null||['light','dark'].includes(theme))&&(state===null||['ready','empty','unavailable'].includes(state));
}

async function tabTo(page,selector){
  for(let index=0;index<120;index++){
    if(await page.evaluate(value=>document.activeElement?.matches(value),selector))return;
    await page.keyboard.press('Tab');
  }
  throw Error(`Tab did not reach ${selector}`);
}

async function selectFact(page,phase,key=null){
  return page.evaluate(({phase,key})=>{
    const select=document.querySelector('#content-state'),active=document.activeElement;
    return{phase,key,url:location.href,value:select?.value,selectedIndex:select?.selectedIndex,active:{tag:active?.tagName,id:active?.id}};
  },{phase,key});
}

async function execute(root){
  const playwrightPath=process.env.APPKIT_PLAYWRIGHT_MODULE;
  if(!playwrightPath||!path.isAbsolute(playwrightPath))throw Error('APPKIT_PLAYWRIGHT_MODULE must be absolute');
  const playwrightBytes=await fs.readFile(playwrightPath);
  if(sha256(playwrightBytes)!==approvedPlaywrightSHA)throw Error('unapproved Playwright entry');
  const{chromium}=await import(pathToFileURL(playwrightPath));
  const config={schema_version:'static-content-select-typeahead-v1',chrome:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',playwright:{path:playwrightPath,sha256:approvedPlaywrightSHA},context:{width:390,height:1000,javascript:false,cache:'disabled',serviceWorkers:'block'},cases:[{state:'empty',label:'Synthetic empty state',status:200},{state:'unavailable',label:'Synthetic unavailable state',status:503}]};
  const configBytes=Buffer.from(JSON.stringify(config,null,2)+'\n');
  await fs.writeFile(path.join(root,'diagnostic-config.json'),configBytes,{flag:'wx'});
  const report={schema_version:'static-content-select-typeahead-receipt-v1',root,started:new Date().toISOString(),inputs:{config:{bytes:configBytes.length,sha256:sha256(configBytes)}},requests:[],cases:[],failures:[]};
  for(const name of['select_diagnostic.mjs','gallery','app.css','contract.json']){
    const bytes=await fs.readFile(path.join(root,name));report.inputs[name]={bytes:bytes.length,sha256:sha256(bytes)};
  }
  let gallery,server,browser,origin,port,stdout='',stderr='';
  try{
    gallery=spawn(path.join(root,'gallery'),['-addr','127.0.0.1:0','-design-assets',''],{cwd:root,stdio:['ignore','pipe','pipe']});
    gallery.stdout.on('data',bytes=>{stdout+=bytes;});gallery.stderr.on('data',bytes=>{stderr+=bytes;});
    origin=await bounded(new Promise((resolve,reject)=>{gallery.once('error',reject);gallery.once('exit',()=>reject(Error('Gallery exited before readiness')));gallery.stderr.on('data',bytes=>{const match=String(bytes).match(/(http:\/\/127\.0\.0\.1:(\d+))/);if(match){port=Number(match[2]);resolve(match[1]);}}); }),'Gallery readiness',15000);
    server=await chromium.launchServer({headless:true,executablePath:config.chrome,args:['--host-resolver-rules=MAP * ~NOTFOUND, EXCLUDE 127.0.0.1']});
    browser=await chromium.connect(server.wsEndpoint());report.chromeVersion=browser.version();
    for(const specimen of config.cases){
      const result={...specimen,trace:[],navigations:[],events:[]};report.cases.push(result);
      const context=await browser.newContext({viewport:{width:390,height:1000},colorScheme:'light',reducedMotion:'reduce',javaScriptEnabled:false,serviceWorkers:'block'});
      await context.route('**/*',route=>{const request=route.request(),allowed=allowedRequest(origin,request);report.requests.push({case:specimen.state,method:request.method(),url:request.url(),allowed});if(!allowed)report.failures.push({case:specimen.state,error:'forbidden request',url:request.url()});return allowed?route.continue():route.abort();});
      const page=await context.newPage();page.setDefaultTimeout(7000);
      const cdp=await context.newCDPSession(page);await cdp.send('Network.enable');await cdp.send('Network.setCacheDisabled',{cacheDisabled:true});
      let phase='initial';page.on('framenavigated',frame=>{if(frame===page.mainFrame())result.navigations.push({phase,url:frame.url()});});
      await page.goto(`${origin}/content?theme=light`,{waitUntil:'load'});
      await page.evaluate(()=>{globalThis.__selectEvents=[];const select=document.querySelector('#content-state'),form=select.form;for(const type of['keydown','keyup','input','change'])select.addEventListener(type,event=>globalThis.__selectEvents.push({type,key:event.key,value:select.value,selectedIndex:select.selectedIndex,active:document.activeElement?.id}));form.addEventListener('submit',()=>globalThis.__selectEvents.push({type:'submit',value:select.value,selectedIndex:select.selectedIndex,active:document.activeElement?.id}));});
      await tabTo(page,'#content-state');result.trace.push(await selectFact(page,'typeahead:before'));
      for(const character of specimen.label){
        phase=`typeahead:${JSON.stringify(character)}`;const before=result.navigations.length;
        result.trace.push(await selectFact(page,`${phase}:before`,character));await page.keyboard.type(character);await page.waitForTimeout(20);result.trace.push(await selectFact(page,`${phase}:after`,character));
        if(result.navigations.length!==before)throw Error(`${specimen.state}: navigation during typeahead`);
      }
      phase='typeahead:tab';await page.keyboard.press('Tab');await page.waitForTimeout(100);
      result.finalSelection=await selectFact(page,'typeahead:tab:after');result.trace.push(result.finalSelection);result.events=await page.evaluate(()=>globalThis.__selectEvents);
      if(result.finalSelection.value!==specimen.state||result.finalSelection.active.tag!=='BUTTON'){
        const bytes=await page.screenshot({animations:'disabled'}),file=`${specimen.state}-typeahead-failure.png`;await fs.writeFile(path.join(root,file),bytes,{flag:'wx'});result.failureCapture={file,bytes:bytes.length,sha256:sha256(bytes)};report.failures.push({case:specimen.state,error:'typeahead did not select exact state and focus submit button',finalSelection:result.finalSelection});await context.close();continue;
      }
      phase='submit:enter';const navigation=page.waitForNavigation({waitUntil:'load'});await page.keyboard.press('Enter');const response=await navigation;
      result.submission={url:response?.url(),status:response?.status()};const submitted=new URL(result.submission.url);
      if(result.submission.status!==specimen.status||submitted.searchParams.get('state')!==specimen.state||submitted.searchParams.get('theme')!=='light')report.failures.push({case:specimen.state,error:'submitted GET did not match expected state/theme/status',submission:result.submission});
      await context.close();
    }
  }catch(error){report.failures.push({error:String(error),stack:error.stack});}
  finally{
    try{if(browser)await bounded(browser.close(),'browser close');if(server)await bounded(server.close(),'server close');}catch(error){report.failures.push({cleanup:'browser',error:String(error)});try{server?.process()?.kill('SIGKILL');}catch{}}
    try{if(gallery&&gallery.exitCode===null&&gallery.signalCode===null)gallery.kill('SIGTERM');if(gallery&&gallery.exitCode===null&&gallery.signalCode===null)await bounded(new Promise(resolve=>gallery.once('close',resolve)),'Gallery close');}catch(error){report.failures.push({cleanup:'gallery',error:String(error)});}
    let portRefused=false,refusalError=null;if(Number.isInteger(port))await new Promise(resolve=>{const socket=net.connect({host:'127.0.0.1',port}),timer=setTimeout(()=>{refusalError={code:'TIMEOUT'};socket.destroy();resolve();},3000);socket.once('error',error=>{clearTimeout(timer);refusalError={code:error.code,message:error.message};portRefused=error.code==='ECONNREFUSED';resolve();});socket.once('connect',()=>{clearTimeout(timer);socket.destroy();resolve();});});
    report.cleanup={browser:{exitCode:server?.process()?.exitCode,signalCode:server?.process()?.signalCode??null},gallery:{exitCode:gallery?.exitCode,signalCode:gallery?.signalCode??null},portRefused,refusalError};
    if(report.cleanup.browser.exitCode!==0||report.cleanup.browser.signalCode!==null||report.cleanup.gallery.exitCode!==0||report.cleanup.gallery.signalCode!==null||!portRefused)report.failures.push({cleanup:'postconditions',detail:report.cleanup});
    report.logs={stdout,stderr};report.finished=new Date().toISOString();await fs.writeFile(path.join(root,'select-diagnostic-receipt.json'),JSON.stringify(report,null,2),{flag:'wx'});
  }
  if(report.failures.length)process.exitCode=1;
}

async function main(){
  const[parentArg,binaryArg,cssArg,contractArg]=process.argv.slice(2);
  if(![parentArg,binaryArg,cssArg,contractArg].every(value=>value&&path.isAbsolute(value)))throw Error('usage: select_diagnostic.mjs PARENT BINARY CSS CONTRACT');
  const parent=await fs.realpath(parentArg),root=await fs.mkdtemp(path.join(parent,'select-typeahead-diagnostic-'));await fs.chmod(root,0o700);
  const sources=[fileURLToPath(import.meta.url),binaryArg,cssArg,contractArg],names=['select_diagnostic.mjs','gallery','app.css','contract.json'];
  for(let index=0;index<names.length;index++)await fs.writeFile(path.join(root,names[index]),await fs.readFile(sources[index]),{flag:'wx',mode:names[index]==='gallery'?0o700:0o600});
  const child=spawn(process.execPath,[path.join(root,'select_diagnostic.mjs'),'--execute',root],{stdio:'inherit',env:process.env});process.exitCode=await new Promise((resolve,reject)=>{child.once('error',reject);child.once('exit',code=>resolve(code??1));});
}

if(import.meta.url===pathToFileURL(process.argv[1]??'').href)(process.argv[2]==='--execute'?execute(process.argv[3]):main()).catch(error=>{console.error(error.stack||error);process.exitCode=1;});
