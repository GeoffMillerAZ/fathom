// Isolated Chrome, synthetic loopback HTTP, no application JavaScript.
import fs from 'node:fs'
import path from 'node:path'
import crypto from 'node:crypto'
import { spawn } from 'node:child_process'
import { pathToFileURL } from 'node:url'

const dir=fs.realpathSync(process.argv[2]||'')
const read=name=>fs.readFileSync(path.join(dir,name))
const sha=b=>crypto.createHash('sha256').update(b).digest('hex')
const preparation=JSON.parse(read('prepare.json'))
if(preparation.status!=='passed'||sha(read('workflow.test'))!==preparation.binary_sha256||sha(read('form.css'))!==preparation.css_sha256)throw Error('prepared identity mismatch')
const entry='/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs'
const pwVersion=JSON.parse(fs.readFileSync(path.join(path.dirname(entry),'package.json'))).version
if(pwVersion!=='1.63.0')throw Error('unexpected Playwright pin')
const {chromium}=await import(pathToFileURL(entry).href)
const report={schema:1,scope:'P05-native-synthetic-workflow',verification:'not-verified',artifact_sha256:preparation.artifact_sha256,checks:[],screenshots:[],versions:{playwright:pwVersion},javascript:false}
const check=(name,ok,detail)=>{report.checks.push({name,ok:!!ok,...(detail===undefined?{}:{detail})})}
const out=fs.mkdtempSync(path.join(dir,'browser-'));fs.chmodSync(out,0o700)
const save=(name,data)=>fs.writeFileSync(path.join(out,name),data,{flag:'wx',mode:0o600})
const server=spawn(path.join(dir,'workflow.test'),['-test.run=^TestEmailFieldBrowserServer$','-test.v'],{cwd:path.join(dir,'build'),env:{...process.env,APPKIT_P05_SERVE:'1',APPKIT_P05_CSS:path.join(dir,'form.css'),APPKIT_P05_ORIGINAL:path.join(dir,'source.html'),APPKIT_P05_DEFAULTS:path.join(dir,'build/defaults.json')},stdio:['ignore','pipe','pipe']})
let output='',errors='',browser
server.stderr.on('data',b=>errors+=b)
const serverURL=new Promise((resolve,reject)=>{
  const timeout=setTimeout(()=>reject(Error('fixture startup timeout')),15000)
  server.stdout.on('data',b=>{output+=b;const match=output.match(/APPKIT_P05_URL=(http:\/\/127\.0\.0\.1:\d+)/);if(match){clearTimeout(timeout);resolve(match[1])}})
  server.once('error',e=>{clearTimeout(timeout);reject(e)})
  server.once('exit',code=>{clearTimeout(timeout);reject(Error(`fixture exited ${code}`))})
})
try {
  const base=await serverURL
  browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true})
  report.versions.chrome=browser.version()
  const context=async(width,theme)=>{
    const c=await browser.newContext({viewport:{width,height:1000},deviceScaleFactor:1,javaScriptEnabled:false,colorScheme:theme,reducedMotion:'reduce'})
    await c.route('**/*',route=>{const u=route.request().url();if(u.startsWith(base+'/'))return route.continue();check('no-external-request',false,u);return route.abort()})
    return c
  }
  const ready=async(page)=>{await page.evaluate(()=>document.fonts.ready);await page.locator('input[type=email]').waitFor()}
  const screenshot=async(page,name)=>{const b=await page.screenshot({animations:'disabled'});save(name,b);report.screenshots.push({name,sha256:sha(b)});return b}
  const geometry=page=>page.evaluate(()=>{
    const input=document.querySelector('input[type=email]'),label=document.querySelector('label'),button=document.querySelector('button[type=submit]')
    const box=e=>e?{width:e.getBoundingClientRect().width,height:e.getBoundingClientRect().height}:null
    const fonts=[...document.fonts].filter(f=>f.family.replaceAll('"','')==='InterVariable')
    return{overflow:document.documentElement.scrollWidth>innerWidth,input:box(input),button:box(button),labelClipped:label.scrollWidth>label.clientWidth+1,fonts:fonts.length>0&&fonts.every(f=>f.status==='loaded'),focus:document.activeElement?.id,outline:getComputedStyle(input).outlineWidth}
  })
  for(const width of [390,1440])for(const theme of ['light','dark']){
    const key=`${width}-${theme}`,c=await context(width,theme),p=await c.newPage()
    await p.goto(base+`/reference-original?theme=${theme}`);await ready(p)
    const originalDOM=await p.locator('.reference-specimen').innerHTML(),a=await screenshot(p,`${key}-original.png`)
    const originalGeometry=await geometry(p)
    await p.goto(base+`/reference-adapted?theme=${theme}`);await ready(p)
    // Sample both DOMs before screenshot tooling temporarily adjusts caret styles.
    const adaptedDOM=await p.locator('.reference-specimen').innerHTML()
    const b=await screenshot(p,`${key}-adapted.png`)
    check(`${key}:original-default-dom`,originalDOM===adaptedDOM)
    check(`${key}:original-default-pixels`,a.equals(b))
    check(`${key}:selected-font-loaded`,originalGeometry.fonts&&(await geometry(p)).fonts)
    check(`${key}:original-target-size`,originalGeometry.input.height>=44,{height:originalGeometry.input.height,inherited:true})
    await c.close()
  }
  for(const width of [390,1440])for(const theme of ['light','dark'])for(const inputMethod of ['keyboard','pointer'])for(const long of [false,true]){
    const key=`${width}-${theme}-${inputMethod}-${long?'long':'ordinary'}`,c=await context(width,theme),p=await c.newPage(),requests=[]
    p.on('request',r=>requests.push({method:r.method(),url:r.url()}))
    await p.goto(base+`/demo?theme=${theme}${long?'&long=1':''}`);await ready(p)
    const field=p.locator('input[type=email]'),button=p.getByRole('button',{name:'Save synthetic email'})
    const focusReady=async(selector)=>{const start=performance.now();let ok=true;try{await p.waitForFunction(s=>document.querySelector(s)===document.activeElement,selector,{timeout:1000})}catch{ok=false}check(`${key}:native-autofocus-ready:${selector}`,ok,{elapsed_ms:Math.round(performance.now()-start),synchronization_ceiling_ms:1000})}
    const activate=async()=>{if(inputMethod==='keyboard'){await p.keyboard.press('Tab');check(`${key}:native-tab-submit`,await button.evaluate(e=>e===document.activeElement));await p.keyboard.press('Enter')}else{await button.click()}}
    const submit=async()=>{const [response]=await Promise.all([p.waitForResponse(r=>r.url()===base+'/save'&&r.request().method()==='POST'),p.waitForEvent('framenavigated'),activate()]);await p.waitForLoadState('load');await p.evaluate(()=>document.fonts.ready);return response.status()}
    check(`${key}:initial-clean`,await field.getAttribute('aria-invalid')===null)
    await field.fill('not-an-email');const before=requests.filter(r=>r.method==='POST').length
    await activate()
    check(`${key}:browser-malformed-refusal`,await field.evaluate(e=>!e.validity.valid)&&requests.filter(r=>r.method==='POST').length===before)
    await field.fill('blocked@example.test')
    check(`${key}:server-rejection`,await submit()===422)
    await focusReady('input[type=email]')
    check(`${key}:error-retains-associates-focus`,await field.inputValue()==='blocked@example.test'&&await field.getAttribute('aria-invalid')==='true'&&await field.getAttribute('aria-describedby')==='contact-email-error'&&await field.evaluate(e=>e===document.activeElement))
    if(inputMethod==='keyboard'&&!long)await screenshot(p,`${key}-rejected.png`)
    const g=await geometry(p)
    check(`${key}:usable-geometry`,!g.overflow&&!g.labelClipped&&g.input.height>=44&&g.button.height>=44&&g.fonts,g)
    check(`${key}:focus-visible`,Number.parseFloat(g.outline)>=2)
    await field.fill('person@example.test')
    check(`${key}:save-failure`,await submit()===503)
    await focusReady('#save-error-summary')
    check(`${key}:failure-retains-neutral-field`,await field.inputValue()==='person@example.test'&&await field.getAttribute('aria-invalid')===null&&await field.getAttribute('aria-describedby')===null)
    check(`${key}:failure-summary-focus`,await p.locator('#save-error-summary').evaluate(e=>e===document.activeElement))
    check(`${key}:failure-page-title`,/could not save|save failed/i.test(await p.title()))
    if(inputMethod==='keyboard'&&!long)await screenshot(p,`${key}-save-failure.png`)
    await p.getByRole('link',{name:'Review the retained email'}).focus()
    if(inputMethod==='keyboard')await p.keyboard.press('Enter');else await p.getByRole('link',{name:'Review the retained email'}).click()
    check(`${key}:summary-link-focus`,await field.evaluate(e=>e===document.activeElement))
    check(`${key}:retry-redirect`,await submit()===303)
    check(`${key}:confirmation`,new URL(p.url()).pathname==='/done'&&await p.getByRole('heading',{name:'Email saved',exact:true}).count()===1&&(await p.locator('main').innerText()).includes('person@example.test'))
    const postCount=requests.filter(r=>r.method==='POST').length
    await p.reload();check(`${key}:refresh-no-resubmit`,requests.filter(r=>r.method==='POST').length===postCount&&postCount===3)
    await c.close()
  }
} catch(e){report.error=e.message;check('harness-completed',false,e.message)} finally {
  if(browser)await browser.close()
  const closed=new Promise(resolve=>{if(server.exitCode!==null)return resolve();server.once('exit',resolve)})
  server.kill('SIGINT')
  await Promise.race([closed,new Promise(resolve=>setTimeout(resolve,3000))])
  if(server.exitCode===null)server.kill('SIGKILL')
  save('server.txt',output+'\n'+errors)
  const inherited=report.checks.filter(c=>!c.ok&&c.detail?.inherited)
  const failures=report.checks.filter(c=>!c.ok&&!c.detail?.inherited)
  report.inherited_failures=inherited.length;report.failures=failures.length;report.status=failures.length?'failed':'passed-with-recorded-source-limitations'
  save('browser.json',JSON.stringify(report,null,2)+'\n')
  console.log(JSON.stringify({directory:out,checks:report.checks.length,failures:failures.length,inherited_failures:inherited.length,status:report.status,error:report.error}))
  process.exitCode=failures.length?1:0
}
