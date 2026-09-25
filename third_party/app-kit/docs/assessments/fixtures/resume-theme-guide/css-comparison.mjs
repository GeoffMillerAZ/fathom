// Compare the same actual RAC specimens against old/new compiled CSS. No live
// app routes, external requests, fonts or signed-in browser profiles are used.
import { chromium } from '/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs';
import { mkdir, writeFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { resolve } from 'node:path';

const [baseline, candidate, output, renderer] = process.argv.slice(2);
if (!baseline || !candidate || !output || (renderer && renderer!=='--software') || process.argv.length>6)
  throw Error('usage: node css-comparison.mjs BASE_ORIGIN CANDIDATE_ORIGIN NEW_OUTPUT [--software]');
for (const value of [baseline, candidate]) {
  const u = new URL(value);
  if (u.protocol !== 'http:' || u.hostname !== '127.0.0.1' || !u.port || u.origin !== value) throw Error('exact loopback origins required');
}
const out = resolve(output);
await mkdir(out); // collision is a refusal, not overwrite
const sha = b => createHash('sha256').update(b).digest('hex');
// An explicit software control isolates GPU raster variability; never hide its
// use or reinterpret a default-renderer pixel failure as passing.
const args=renderer==='--software'?['--disable-gpu']:[];
const browser = await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless:true,args});
const report = {baseline, candidate, browser:browser.version(), launchArgs:args, cases:[], refusedRequests:[],failedRequests:[]};
try {
  for (const width of [390, 1440]) {
    for (const scheme of ['light', 'dark']) {
      const contexts = await Promise.all([baseline, candidate].map(async origin => {
        const context = await browser.newContext({viewport:{width,height:1000}, colorScheme:scheme, reducedMotion:'reduce', javaScriptEnabled:false});
        await context.route('**/*', async route => {
          const u = new URL(route.request().url());
          if (u.origin === origin && ['/theme-guide/preview','/theme-guide/app.css','/theme-guide/brand.css'].includes(u.pathname)) return route.continue();
          report.refusedRequests.push(u.href);
          await route.abort();
        });
        return context;
      }));
      const pages = await Promise.all(contexts.map(c=>c.newPage()));
      for(const page of pages) page.on('requestfailed',r=>report.failedRequests.push({url:r.url(),error:r.failure()}));
      for (const view of ['today','pipeline','people']) {
        for (const state of ['populated','empty','incomplete']) {
          const name = `${view}-${state}-${scheme}-${width}`;
          const snapshots=[];
          for (let side=0;side<2;side++) {
            const origin = [baseline,candidate][side];
            const response = await pages[side].goto(`${origin}/theme-guide/preview?specimen=${view}-${state}&scheme=${scheme}`, {waitUntil:'networkidle'});
            if (response.status() !== 200) throw Error(`${name}: HTTP ${response.status()}`);
            const styled=await pages[side].evaluate(()=>getComputedStyle(document.querySelector('.font-sans')).fontFamily.includes('ui-sans-serif') && getComputedStyle(document.querySelector('.mb-4')).marginBottom==='16px');
            if(!styled || report.failedRequests.length) throw Error(`${name}: app CSS did not load/apply; no appearance comparison is valid`);
            const metrics = await pages[side].evaluate(()=>Array.from(document.body.querySelectorAll('*')).map(el=>{
              const s=getComputedStyle(el), r=el.getBoundingClientRect();
              return {tag:el.tagName,classes:el.getAttribute('class'),bounds:[r.x,r.y,r.width,r.height],style:Object.fromEntries(['display','position','color','backgroundColor','fontFamily','fontSize','fontWeight','lineHeight','borderRadius','padding','margin','gap','overflowX','gridTemplateColumns'].map(k=>[k,s[k]]))};
            }));
            const png = await pages[side].screenshot({fullPage:true, animations:'disabled'});
            await writeFile(resolve(out, `${name}-${side===0?'baseline':'candidate'}.png`),png);
            snapshots.push({metrics, pngSHA256:sha(png)});
          }
          const differences=[];
          for(let i=0;i<Math.max(...snapshots.map(s=>s.metrics.length));i++) {
            if(JSON.stringify(snapshots[0].metrics[i])!==JSON.stringify(snapshots[1].metrics[i])) differences.push({index:i,baseline:snapshots[0].metrics[i],candidate:snapshots[1].metrics[i]});
          }
          report.cases.push({name,elements:snapshots[0].metrics.length,differences,pixelsEqual:snapshots[0].pngSHA256===snapshots[1].pngSHA256,baselinePNG:snapshots[0].pngSHA256,candidatePNG:snapshots[1].pngSHA256});
        }
      }
      await Promise.all(contexts.map(c=>c.close()));
    }
  }
  report.passed = report.cases.length===36 && report.refusedRequests.length===0 && report.failedRequests.length===0 && report.cases.every(c=>c.differences.length===0 && c.pixelsEqual);
  await writeFile(resolve(out,'report.json'),JSON.stringify(report,null,2)+'\n');
  process.stdout.write(JSON.stringify({passed:report.passed,cases:report.cases.length,differing:report.cases.filter(c=>c.differences.length||!c.pixelsEqual).map(c=>({name:c.name,elements:c.differences.length,pixelsEqual:c.pixelsEqual})),refusedRequests:report.refusedRequests})+'\n');
  if(!report.passed) process.exitCode=1;
} catch(error) {
  report.passed=false;
  report.error=String(error);
  await writeFile(resolve(out,'report.json'),JSON.stringify(report,null,2)+'\n');
  throw error;
} finally {
  await browser.close();
}
