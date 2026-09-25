// Opt-in layout-only proof for the compiled settings-section fixture.
import fs from 'node:fs';
import path from 'node:path';
import {createHash} from 'node:crypto';
import {fileURLToPath} from 'node:url';
import {bootstrap} from './config.mjs';

const sha256 = bytes => createHash('sha256').update(bytes).digest('hex');
const identity = file => {
  const bytes = fs.readFileSync(file);
  return {path:file, bytes:bytes.length, sha256:sha256(bytes)};
};
const fixturePath = process.env.APPKIT_SETTINGS_FIXTURE;
if (!fixturePath || !path.isAbsolute(fixturePath)) {
  throw new Error('APPKIT_SETTINGS_FIXTURE must be the absolute path to the retained compiled HTML fixture.');
}
const fixtureInfo = fs.lstatSync(fixturePath);
if (!fixtureInfo.isFile() || fixtureInfo.isSymbolicLink()) throw new Error('APPKIT_SETTINGS_FIXTURE must be a real regular file.');
if (fixtureInfo.size > 64 * 1024) throw new Error('APPKIT_SETTINGS_FIXTURE exceeds the 64 KiB proof bound.');
const fixtureBytes = fs.readFileSync(fixturePath);

const {base, variant, root, chromium, executablePath, moduleURL} = await bootstrap();
const virtualPath = '/__settings-sections-fixture';
const fixtureURL = base + virtualPath;
const cssURL = base + '/static/app.css';
const shots = path.join(root, `settings-sections-${variant}-screenshots`);
fs.mkdirSync(shots, {recursive:true});
const reportPath = path.join(root, `settings-sections-${variant}-report.json`);
const report = {
  schema:1,
  kind:'settings-sections-component-layout-only',
  base,
  variant,
  virtualPath,
  startedAt:new Date().toISOString(),
  node:process.version,
  fixture:{path:fixturePath, bytes:fixtureBytes.length, sha256:sha256(fixtureBytes)},
  sources:{
    harness:identity(fileURLToPath(import.meta.url)),
    config:identity(fileURLToPath(new URL('./config.mjs', import.meta.url))),
    playwrightModule:identity(fileURLToPath(moduleURL)),
  },
  passes:[], failures:[], matrices:[], requests:[], blocked:[], websockets:[],
  pageErrors:[], consoleErrors:[], screenshots:[], servedCSS:[],
  limits:[
    'A synthetic virtual route renders exact compiled helper output with the Gallery\'s real served CSS; this is component layout evidence, not a Gallery application route.',
    'The fixture has no save implementation and this probe never submits its form; it is not save, backend, authentication, consumer-adoption or screen-reader certification.',
    'Range geometry covers rendered headings, descriptions, labels and identity text; native select popup option geometry is browser-owned and is not claimed.',
  ],
};
const check = (ok, name, detail='') => (ok ? report.passes : report.failures).push({name, detail});
let browser;
try {
  browser = await chromium.launch({headless:true, executablePath});
  report.browser={version:browser.version(), executablePath};
  for (const theme of ['light','dark']) for (const width of [390,1440]) {
    const lane = {theme, width, viewport:{width,height:1000}};
    report.matrices.push(lane);
    const context = await browser.newContext({
      javaScriptEnabled:false,
      colorScheme:theme,
      viewport:lane.viewport,
      serviceWorkers:'block',
    });
    context.setDefaultTimeout(6000);
    await context.route('**/*', route => {
      const request = route.request();
      const url = new URL(request.url());
      const exactFixture = request.method()==='GET' && url.origin===base && url.pathname===virtualPath && !url.search;
      const exactCSS = request.method()==='GET' && url.origin===base && url.pathname==='/static/app.css' && !url.search;
      if (exactFixture) return route.fulfill({status:200, contentType:'text/html; charset=utf-8', body:fixtureBytes});
      if (exactCSS) return route.continue();
      report.blocked.push({theme,width,method:request.method(),url:request.url()});
      return route.abort();
    });
    await context.routeWebSocket('**/*', socket => {
      report.websockets.push({theme,width,url:socket.url()});
      socket.close();
    });
    context.on('request', request => report.requests.push({theme,width,method:request.method(),url:request.url()}));
    const page = await context.newPage();
    page.on('pageerror', error => report.pageErrors.push({theme,width,error:String(error)}));
    page.on('console', message => {
      if (message.type()==='error') report.consoleErrors.push({theme,width,text:message.text(),url:message.location().url});
    });
    try {
      const [navigation,cssResponse] = await Promise.all([
        page.goto(fixtureURL, {waitUntil:'load'}),
        page.waitForResponse(response => response.url()===cssURL && response.request().method()==='GET'),
      ]);
      const cssBytes = await cssResponse.body();
      const cssIdentity = {theme,width,status:cssResponse.status(),contentType:cssResponse.headers()['content-type']||'',bytes:cssBytes.length,sha256:sha256(cssBytes)};
      report.servedCSS.push(cssIdentity);
      check(navigation?.status()===200, `fixture response ${width}/${theme}`, navigation?.status());
      check(cssIdentity.status===200 && cssIdentity.bytes>0 && /text\/css/i.test(cssIdentity.contentType), `real Gallery CSS response ${width}/${theme}`, cssIdentity);

      const observed = await page.evaluate(({width,theme}) => {
        const ids = ['fixture-name-title','fixture-region-title','fixture-identity-title'];
        const sections = [...document.querySelectorAll('main form > section')];
        const rect = element => {
          const r=element.getBoundingClientRect();
          return {left:r.left,right:r.right,top:r.top,bottom:r.bottom,width:r.width,height:r.height};
        };
        const sectionData = sections.map(section => {
          const heading=section.querySelector('h3');
          const description=section.querySelector('p');
          const columns=[...section.children].filter(node=>node.tagName==='DIV').map(rect);
          const sectionRect=rect(section);
          const text=[];
          for (const element of section.querySelectorAll('h3,p,label,dt,dd')) {
            const walker=document.createTreeWalker(element,NodeFilter.SHOW_TEXT);
            for(let node=walker.nextNode();node;node=walker.nextNode()) {
              if(!node.data.trim()) continue;
              const range=document.createRange(); range.selectNodeContents(node);
              text.push({value:node.data.trim().slice(0,80),rects:[...range.getClientRects()].map(r=>({left:r.left,right:r.right,top:r.top,bottom:r.bottom}))});
            }
          }
          return {aria:section.getAttribute('aria-labelledby'),headingID:heading?.id,headingText:heading?.textContent,descriptionText:description?.textContent,sectionRect,columns,text};
        });
        const controls=['fixture-name','fixture-region'].map(id=>{
          const control=document.getElementById(id),label=document.querySelector(`label[for="${id}"]`),section=control?.closest('section');
          const style=control&&getComputedStyle(control);
          return {id,label:label?.textContent.trim(),associated:control?.labels?.length===1 && control.labels[0]===label,controlRect:control&&rect(control),sectionRect:section&&rect(section),baseline:style&&{outlineStyle:style.outlineStyle,outlineWidth:style.outlineWidth,outlineColor:style.outlineColor,boxShadow:style.boxShadow}};
        });
        const bodyStyle=getComputedStyle(document.body);
        return {
          width,theme,dark:matchMedia('(prefers-color-scheme: dark)').matches,
          ids, sectionData, controls,
          bodyColors:{backgroundColor:bodyStyle.backgroundColor,color:bodyStyle.color},
          viewportWidth:document.documentElement.clientWidth,
          scrollWidth:document.documentElement.scrollWidth,
          scrollHeight:document.documentElement.scrollHeight,
          scriptCount:document.scripts.length,
        };
      }, {width,theme});
      lane.observed=observed;
      const order = observed.sectionData.map(section=>section.headingID);
      check(JSON.stringify(order)===JSON.stringify(observed.ids), `heading order ${width}/${theme}`, order);
      check(observed.sectionData.every(section=>section.aria===section.headingID), `section heading associations ${width}/${theme}`, observed.sectionData.map(section=>({aria:section.aria,id:section.headingID})));
      check(observed.sectionData[0]?.headingText==='Workspace'.repeat(18) && observed.sectionData[1]?.headingText==='Region and language' && observed.sectionData[2]?.headingText==='Identity'.repeat(18), `compiled heading text ${width}/${theme}`);
      const descriptions=[
        'preference'.repeat(40)+' remains app-owned and should wrap without hiding the control.',
        'Choose the display region for this layout-only fixture.',
        'Read-only identity information remains owned by the application.',
      ];
      check(observed.sectionData.every((section,index)=>section.descriptionText===descriptions[index]), `complete compiled description text ${width}/${theme}`, observed.sectionData.map(section=>section.descriptionText));
      check(observed.dark===(theme==='dark'), `native color-scheme emulation ${width}/${theme}`, observed.dark);
      check(observed.scriptCount===0, `no fixture scripts ${width}/${theme}`, observed.scriptCount);
      check(observed.scrollWidth<=observed.viewportWidth, `no horizontal page overflow ${width}/${theme}`, {scrollWidth:observed.scrollWidth,viewportWidth:observed.viewportWidth});
      const geometryOK = observed.sectionData.every(section => section.columns.length===2 && (width===390
        ? section.columns[1].top>=section.columns[0].bottom-1
        : Math.abs(section.columns[1].top-section.columns[0].top)<=2 && section.columns[1].left>section.columns[0].left));
      check(geometryOK, `${width===390?'stacked':'two-column'} section geometry ${width}/${theme}`, observed.sectionData.map(section=>section.columns));
      const badText = observed.sectionData.flatMap(section => section.text.flatMap(item => {
        if (!item.rects.length) return [{text:item.value,reason:'no Range rect',heading:section.headingID}];
        return item.rects.filter(r=>r.left<section.sectionRect.left-1 || r.right>section.sectionRect.right+1 || r.left<-1 || r.right>observed.viewportWidth+1 || r.top<section.sectionRect.top-1 || r.bottom>section.sectionRect.bottom+1 || r.top<-1 || r.bottom>observed.scrollHeight+1).map(r=>({text:item.value,heading:section.headingID,rect:r,section:section.sectionRect}));
      }));
      check(badText.length===0, `Range text geometry contained ${width}/${theme}`, badText);
      check(observed.controls.every(control=>control.associated), `compiled control labels ${width}/${theme}`, observed.controls.map(control=>({id:control.id,label:control.label,associated:control.associated})));
      const controlsContained = observed.controls.every(control=>control.controlRect && control.sectionRect && control.controlRect.left>=control.sectionRect.left-1 && control.controlRect.right<=control.sectionRect.right+1 && control.controlRect.left>=-1 && control.controlRect.right<=observed.viewportWidth+1);
      check(controlsContained, `controls contained ${width}/${theme}`, observed.controls);

      const focus=[];
      for (const expected of [{id:'fixture-name',label:'Display name'},{id:'fixture-region',label:'Region'}]) {
        await page.keyboard.press('Tab');
        const baseline=observed.controls.find(control=>control.id===expected.id)?.baseline;
        focus.push(await page.evaluate(({expected,baseline}) => {
          const active=document.activeElement,style=getComputedStyle(active);
          const label=document.querySelector(`label[for="${expected.id}"]`);
          const bounds=active?.getBoundingClientRect();
          const focused={outlineStyle:style.outlineStyle,outlineWidth:style.outlineWidth,outlineColor:style.outlineColor,boxShadow:style.boxShadow};
          const changedOutline=focused.outlineStyle!=='none' && parseFloat(focused.outlineWidth)>0 && focused.outlineColor!=='transparent' && !/^rgba?\(0, 0, 0, 0\)$/.test(focused.outlineColor) && (focused.outlineStyle!==baseline?.outlineStyle || focused.outlineWidth!==baseline?.outlineWidth || focused.outlineColor!==baseline?.outlineColor);
          const changedShadow=focused.boxShadow!=='none' && focused.boxShadow!==baseline?.boxShadow;
          return {expected,activeID:active?.id,label:label?.textContent.trim(),associated:active?.labels?.length===1 && active.labels[0]===label,baseline,focused,changedOutline,changedShadow,visibleIndicator:changedOutline||changedShadow,bounds:bounds&&{left:bounds.left,right:bounds.right,top:bounds.top,bottom:bounds.bottom},viewport:{width:innerWidth,height:innerHeight}};
        }, {expected,baseline}));
      }
      lane.focus=focus;
      check(focus.every(item=>item.activeID===item.expected.id && item.label===item.expected.label && item.associated), `native Tab order and labels ${width}/${theme}`, focus);
      check(focus.every(item=>item.visibleIndicator), `visible native focus indicators ${width}/${theme}`, focus);
      check(focus.every(item=>item.bounds && item.bounds.left>=-1 && item.bounds.right<=item.viewport.width+1 && item.bounds.top>=-1 && item.bounds.bottom<=item.viewport.height+1), `focused controls within viewport ${width}/${theme}`, focus);
      const screenshotPath=path.join(shots,`settings-sections-${width}-${theme}.png`);
      await page.screenshot({path:screenshotPath,fullPage:true});
      report.screenshots.push(identity(screenshotPath));
    } catch (error) {
      check(false, `matrix execution ${width}/${theme}`, String(error.stack||error));
    } finally {
      await context.close();
    }
  }
} catch (error) {
  check(false,'harness execution',String(error.stack||error));
} finally {
  check(report.blocked.length===0,'no unexpected HTTP requests',report.blocked);
  check(report.websockets.length===0,'no WebSockets',report.websockets);
  check(report.pageErrors.length===0,'no page errors',report.pageErrors);
  check(report.consoleErrors.length===0,'no console errors',report.consoleErrors);
  for (const width of [390,1440]) {
    const light=report.matrices.find(matrix=>matrix.width===width && matrix.theme==='light')?.observed?.bodyColors;
    const dark=report.matrices.find(matrix=>matrix.width===width && matrix.theme==='dark')?.observed?.bodyColors;
    check(!!light && !!dark && light.backgroundColor!==dark.backgroundColor && light.color!==dark.color,`effective light/dark body colors differ ${width}`,{light,dark});
  }
  const expectedRequests=report.requests.every(request=>request.method==='GET' && [fixtureURL,cssURL].includes(request.url));
  check(expectedRequests,'request allowlist limited to fixture and real CSS',report.requests);
  check(report.servedCSS.length===4 && new Set(report.servedCSS.map(css=>css.sha256)).size===1,'one stable served CSS identity across matrices',report.servedCSS);
  if (browser) {
    try { await browser.close(); }
    catch (error) { check(false,'browser close',String(error)); }
  }
  report.finishedAt=new Date().toISOString();
  report.result=report.failures.length===0?'pass':'fail';
  fs.writeFileSync(reportPath,JSON.stringify(report,null,2)+'\n');
}
console.log(JSON.stringify({result:report.result,passes:report.passes.length,failures:report.failures,report:reportPath},null,2));
process.exitCode=report.failures.length?1:0;
