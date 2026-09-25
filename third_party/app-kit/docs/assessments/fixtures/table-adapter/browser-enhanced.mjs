import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import http from 'node:http'
import crypto from 'node:crypto'
import { pathToFileURL } from 'node:url'

const LIMIT = 1 << 20
const PLAYWRIGHT = '/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs'
const CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
const CASE = { archive_id:'application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8', path:'html/lists/tables/07-with-stacked-columns-on-mobile.html', sha256:'5d00e2755dcf060cceafa4c2dba4f15452719e54f5cd40646ab3155c89dc8288', size:8709 }
const PINS={cssInput:'7d92b4e13ebe3b82279aa67500ae4d2a383733ef394f12b4b24eda515af7802e',font:'693b77d4f32ee9b8bfc995589b5fad5e99adf2832738661f5402f9978429a8e3',license:'262481e844521b326f5ecd053e59b98c8b2da78c8ee1bdbb6e8174305e54935a',tailwind:'56b4bbc62dbdc4614a78930d9c6986423a2ec63e4e640144a59a5d95c914322e'}
const BASE_SHA='d49356678db9709574847f69871af18c86bb433290c64d6fc3c952f5f91805e7',CSS_SHA='ad956f99f0ff79c7a50698c5b63c98b09d1da5b9e5e02970a16849326d5a643c'
const DOC_SHA={light:'298f9f3aef81798760c3c91eed8598788c478b6b169de8e8d23844650634cbf4',dark:'f4d317ddff9b61c494cc31ac90ebb8a00bd471e1302bd3b5a74f9a866531ab1f'}
const SCENARIOS = ['baseline6','empty0','partial3','long6']
const CORE = ['source.html','artifact.json','enhanced-artifact.json','directory.templ','directory_model.go','directory_enhanced.templ','directory_enhanced_model.go','directory.css','directory_test.go','directory_templ.go','directory_enhanced_templ.go','directory_formatted.templ','directory_enhanced_formatted.templ','directory_formatted_templ.go','directory_enhanced_formatted_templ.go','unformatted.pass','formatted.pass']
const CODE = ['directory.go','directory_model.go.txt','directory.cue','directory_enhanced.go','directory_enhanced_model.go.txt','directory_enhanced.templ.txt','directory_enhanced.css','directory_enhanced.cue','directory_enhanced_live_test.go','directory_enhanced_test.go'].map(x=>`internal/designadapt/${x}`)
const FILES = [...CORE,...SCENARIOS.map(x=>`${x}.json`),...SCENARIOS.flatMap(s=>['faithful','enhanced'].flatMap(k=>['unformatted','formatted'].map(p=>`${s}-${k}-${p}.html`)))]
const sha = value => crypto.createHash('sha256').update(value).digest('hex')
const exactKeys = (value, keys) => value && typeof value === 'object' && !Array.isArray(value) && JSON.stringify(Object.keys(value).sort()) === JSON.stringify([...keys].sort())
const refuse = reason => { throw new Error(reason) }

function safeRoot(value) {
  if (!path.isAbsolute(value)) refuse('absolute-root')
  const real = fs.realpathSync(value)
  if (!real.startsWith(fs.realpathSync(os.tmpdir()) + path.sep) || !fs.statSync(real).isDirectory() || fs.lstatSync(value).isSymbolicLink()) refuse('unsafe-root')
  return real
}
function readBounded(file) {
  const before=fs.lstatSync(file)
  if (!before.isFile() || before.isSymbolicLink() || before.size>LIMIT) refuse('unsafe-input')
  const fd=fs.openSync(file,fs.constants.O_RDONLY|(fs.constants.O_NOFOLLOW||0))
  try {
    const opened=fs.fstatSync(fd)
    if(opened.dev!==before.dev||opened.ino!==before.ino||opened.size!==before.size)refuse('input-changed')
    const data=Buffer.alloc(before.size);let offset=0
    while(offset<data.length){const n=fs.readSync(fd,data,offset,data.length-offset,null);if(!n)refuse('input-short');offset+=n}
    const extra=Buffer.alloc(1),n=fs.readSync(fd,extra,0,1,null),after=fs.fstatSync(fd)
    if(n||after.dev!==before.dev||after.ino!==before.ino||after.size!==before.size)refuse('input-changed')
    return {data,size:data.length,sha256:sha(data)}
  } finally { fs.closeSync(fd) }
}
function snapshot(file){const data=fs.readFileSync(file);return{data,size:data.length,sha256:sha(data)}}
function expectedLong() {
  const repeat=(s,n)=>s.repeat(n), labels={}
  for(const key of ['Add','Edit','Name','Title','Email','Role'])labels[`${key}Label`]=repeat(key,18)
  return {Heading:repeat('Directory',17),Introduction:repeat('Detailed role and contact information for this synthetic directory. ',16)+'<literal & text>',...labels}
}
function validateModel(name, model, baseline) {
  const keys=['Heading','Introduction','AddLabel','EditLabel','NameLabel','TitleLabel','EmailLabel','RoleLabel','AddURL','Rows']
  if(!exactKeys(model,keys)||model.AddURL!=='/directory/new'||!Array.isArray(model.Rows))refuse('scenario-model')
  const lengths={Heading:160,Introduction:1200,AddLabel:100,EditLabel:100,NameLabel:100,TitleLabel:100,EmailLabel:100,RoleLabel:100}
  for(const [key,max]of Object.entries(lengths))if(typeof model[key]!=='string'||!model[key].trim()||Buffer.byteLength(model[key])>max)refuse('scenario-bounds')
  for(const [i,row]of model.Rows.entries()){if(!exactKeys(row,['Name','Title','Email','Role','EditURL'])||row.EditURL!==`/directory/edit/${i+1}`)refuse('scenario-row');for(const [key,max]of Object.entries({Name:240,Title:240,Email:320,Role:100}))if(typeof row[key]!=='string'||!row[key].trim()||Buffer.byteLength(row[key])>max)refuse('scenario-bounds')}
  if(name==='baseline6'&&model.Rows.length!==6)refuse('baseline')
  if(name==='empty0'&&(model.Rows.length||model.Introduction!=='No people yet. Add a user to begin.'))refuse('empty')
  if(name==='partial3'&&(model.Rows.length!==3||model.Introduction!=='Showing 3 available records from a partial result. Other records are not included.'||JSON.stringify(model.Rows)!==JSON.stringify(baseline.Rows.slice(0,3))))refuse('partial')
  if(name==='long6'){
    const want=expectedLong();if(model.Rows.length!==6||Object.entries(want).some(([k,v])=>model[k]!==v))refuse('long')
    model.Rows.forEach((row,i)=>{const n=i+1;if(row.Name!==`LongMember${'x'.repeat(220)}${n}`||row.Title!==`${'PlatformEngineering'.repeat(12)}${n}`||row.Email!==`member${n}.${'x'.repeat(285)}@example.test`||row.Role!==`${'Contributor'.repeat(9)}${n}`)refuse('long-row')})
  }
}

if(process.argv.length!==4)refuse('usage')
const referenceDir=safeRoot(process.argv[2]), enhancedDir=safeRoot(process.argv[3])
if(referenceDir===enhancedDir)refuse('distinct-roots')
const inputs=new Map()
for(const name of ['reference.json','source-light.html','source-dark.html'])inputs.set(`reference/${name}`,readBounded(path.join(referenceDir,name)))
inputs.set('enhanced/enhanced-reference.json',readBounded(path.join(enhancedDir,'enhanced-reference.json')))
for(const name of FILES)inputs.set(`enhanced/${name}`,readBounded(path.join(enhancedDir,name)))
const reference=JSON.parse(inputs.get('reference/reference.json').data), manifest=JSON.parse(inputs.get('enhanced/enhanced-reference.json').data)
if(manifest.schema!==1||manifest.case_id!=='P04-table'||manifest.scope!=='enhanced-target-fixtures'||manifest.verification!=='not-verified'||JSON.stringify(manifest.source)!==JSON.stringify(CASE)||JSON.stringify(manifest.scenarios)!==JSON.stringify(SCENARIOS)||!exactKeys(manifest.files,FILES)||!exactKeys(manifest.code,CODE))refuse('manifest')
for(const name of FILES){const got=inputs.get(`enhanced/${name}`),want=manifest.files[name];if(!exactKeys(want,['sha256','size'])||got.sha256!==want.sha256||got.size!==want.size)refuse('file-identity')}
for(const name of CODE){const got=readBounded(path.join(process.cwd(),name)),want=manifest.code[name];if(!exactKeys(want,['sha256','size'])||got.sha256!==want.sha256||got.size!==want.size)refuse('code-identity');inputs.set(`code/${name}`,got)}
if(inputs.get('enhanced/source.html').sha256!==CASE.sha256||inputs.get('enhanced/source.html').size!==CASE.size||inputs.get('enhanced/unformatted.pass').data.toString()!=='passed\n'||inputs.get('enhanced/formatted.pass').data.toString()!=='passed\n')refuse('compiler-proof')
const base=JSON.parse(inputs.get('enhanced/artifact.json').data),enhanced=JSON.parse(inputs.get('enhanced/enhanced-artifact.json').data)
if(inputs.get('enhanced/artifact.json').sha256!==BASE_SHA)refuse('base-identity')
if(enhanced.schema_version!==1||enhanced.adapter!=='p04-directory-enhanced-v1'||enhanced.verification!=='not-verified'||JSON.stringify(enhanced.base)!==JSON.stringify(base)||sha(Buffer.from(enhanced.templ))!==enhanced.templ_sha256||sha(Buffer.from(enhanced.model))!==enhanced.model_sha256||sha(Buffer.from(enhanced.css))!==enhanced.css_sha256||inputs.get('enhanced/directory_model.go').data.toString()!==base.model||inputs.get('enhanced/directory_enhanced_model.go').data.toString()!==enhanced.model||inputs.get('enhanced/directory.css').data.toString()!==enhanced.css)refuse('artifact')
if(inputs.get('enhanced/directory.templ').data.toString()!==base.templ||inputs.get('enhanced/directory_enhanced.templ').data.toString()!==enhanced.templ)refuse('artifact-template')
const models=new Map()
for(const name of SCENARIOS){const model=JSON.parse(inputs.get(`enhanced/${name}.json`).data);models.set(name,model)}
for(const name of SCENARIOS)validateModel(name,models.get(name),models.get('baseline6'))
const expectedBaseline=structuredClone(base.defaults);expectedBaseline.AddURL='/directory/new';expectedBaseline.Rows.forEach((row,i)=>row.EditURL=`/directory/edit/${i+1}`)
if(JSON.stringify(models.get('baseline6'))!==JSON.stringify(expectedBaseline))refuse('baseline-lineage')
for(const name of ['empty0','partial3'])for(const key of ['Heading','AddLabel','EditLabel','NameLabel','TitleLabel','EmailLabel','RoleLabel','AddURL'])if(models.get(name)[key]!==models.get('baseline6')[key])refuse('scenario-lineage')
for(const scenario of SCENARIOS)for(const kind of ['faithful','enhanced'])if(!inputs.get(`enhanced/${scenario}-${kind}-unformatted.html`).data.equals(inputs.get(`enhanced/${scenario}-${kind}-formatted.html`).data))refuse('phase-replay')

const splitReference=(theme)=>{const text=inputs.get(`reference/source-${theme}.html`).data.toString(),head=`<!doctype html>\n<html lang="en" class="${theme}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>P04 table: controlled source reference</title><style>`,middle='</style></head><body>',tail='</body></html>\n';if(!text.startsWith(head)||!text.endsWith(tail))refuse('reference-envelope');const at=text.indexOf(middle,head.length);if(at<0||at!==text.lastIndexOf(middle))refuse('reference-envelope');return{text,style:text.slice(head.length,at)}}
if(reference.schema!==1||reference.case_id!=='P04-table'||reference.scope!=='controlled-source-reference'||reference.verification!=='not-verified'||JSON.stringify(reference.source)!==JSON.stringify(CASE)||reference.css_sha256!==CSS_SHA||reference.css_input_sha256!==PINS.cssInput||reference.font_sha256!==PINS.font||reference.license_sha256!==PINS.license||reference.tailwind_version!=='v4.3.0'||reference.tailwind_sha256!==PINS.tailwind)refuse('reference-contract')
for(const theme of ['light','dark']){const name=`source-${theme}.html`,got=inputs.get(`reference/${name}`),want=reference.documents[name];if(got.sha256!==want.sha256||got.sha256!==DOC_SHA[theme]||got.size!==want.size)refuse('reference-document')}
const docs=new Map();let sourceStyle=''
for(const theme of ['light','dark']){const source=splitReference(theme);if(sourceStyle&&sourceStyle!==source.style)refuse('reference-style');sourceStyle=source.style;for(const scenario of SCENARIOS)for(const kind of ['faithful','enhanced']){const body=inputs.get(`enhanced/${scenario}-${kind}-formatted.html`).data.toString();docs.set(`/${scenario}-${kind}-${theme}`,`<!doctype html>\n<html lang="en" class="${theme}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>P04 enhanced target</title><style>${sourceStyle}\n${kind==='enhanced'?enhanced.css:''}</style></head><body>${body}</body></html>\n`)}}
const styleHashes=new Map([...docs].map(([route,doc])=>{const style=doc.slice(doc.indexOf('<style>')+7,doc.indexOf('</style>'));return[route,crypto.createHash('sha256').update(style).digest('base64')]}))
const out=fs.mkdtempSync(path.join(enhancedDir,'browser-enhanced-'));fs.chmodSync(out,0o700)
const destinations=new Map([['/directory/new','Create user'],...Array.from({length:6},(_,i)=>[`/directory/edit/${i+1}`,`Edit user ${i+1}`])])
const requests=[],boundary=[],fidelity=[],quality=[],inherited=[],observations=[],checks=[],screens=[]
const check=(name,ok,detail,kind='boundary')=>{checks.push({name,ok:!!ok,kind,...(detail===undefined?{}:{detail})});if(!ok)({boundary,fidelity,quality,inherited}[kind]).push(name)}
const server=http.createServer((req,res)=>{const a=server.address(),host=a&&`${a.address}:${a.port}`,doc=docs.get(req.url),title=destinations.get(req.url),exact=req.method==='GET'&&req.headers.host===host&&req.socket.localAddress==='127.0.0.1'&&req.socket.remoteAddress==='127.0.0.1'&&(doc||title);if(!exact){boundary.push('server-refused');res.writeHead(403,{'Connection':'close'}).end('refused');return}requests.push(req.url);if(title){res.writeHead(200,{'Content-Type':'text/html; charset=utf-8','Content-Security-Policy':"default-src 'none'",'Cache-Control':'no-store'}).end(`<!doctype html><html><body><h1>${title}</h1><p data-route>${req.url}</p></body></html>`);return}const csp=`default-src 'none'; style-src 'sha256-${styleHashes.get(req.url)}'; font-src data:; script-src 'none'; connect-src 'none'; img-src 'none'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'`;res.writeHead(200,{'Content-Type':'text/html; charset=utf-8','Content-Security-Policy':csp,'Cache-Control':'no-store','X-Content-Type-Options':'nosniff','Referrer-Policy':'no-referrer'}).end(doc)})
server.on('clientError',(error,socket)=>{boundary.push(error?.code||'client-error');socket.destroy()})
const inspect=page=>page.evaluate(()=>{const visible=e=>!!e&&e.getClientRects().length>0&&!e.classList.contains('sr-only'),direct=e=>[...e.childNodes].filter(n=>n.nodeType===3&&n.textContent.trim()).map(n=>n.textContent.trim()),rect=e=>{const r=e.getBoundingClientRect();return{width:r.width,height:r.height}},rows=[...document.querySelectorAll('tbody tr')],links=[...document.querySelectorAll('a')],wrapper=document.querySelector('div[data-appkit-directory]'),textNodes=[...document.querySelectorAll('h1,p,th,td,dd,a')].filter(visible),describe=e=>({tag:e.tagName,class:e.className,width:e.clientWidth,scrollWidth:e.scrollWidth});return{wrapperCount:document.querySelectorAll('div[data-appkit-directory]').length,wrapperChildren:wrapper?.children.length,wrapperHTML:wrapper?.innerHTML,bodyHTML:document.body.innerHTML,headings:[...document.querySelectorAll('h1,h2,h3')].map(e=>e.textContent.trim()),introduction:document.querySelector('h1')?.nextElementSibling?.textContent.trim(),headers:[...document.querySelectorAll('thead th')].map(e=>({text:e.textContent.trim(),scope:e.getAttribute('scope')})),rows:rows.map(row=>({texts:[...row.querySelectorAll('*')].filter(visible).flatMap(direct),labels:[...row.querySelectorAll('.sr-only')].map(e=>e.textContent.trim()),links:[...row.querySelectorAll('a')].map(e=>({name:e.textContent.trim().replace(/\s+/g,' '),href:e.getAttribute('href')}))})),links:links.map(e=>({name:e.textContent.trim().replace(/\s+/g,' '),href:e.getAttribute('href'),rect:rect(e)})),overflow:document.documentElement.scrollWidth>innerWidth,containerOverflow:[...document.querySelectorAll('body *')].filter(e=>visible(e)&&(e.scrollWidth>e.clientWidth+1||e.scrollHeight>e.clientHeight+1)).map(describe),clipped:textNodes.filter(e=>e.scrollWidth>e.clientWidth+1||e.scrollHeight>e.clientHeight+1).map(describe),font:{status:document.fonts.status,family:getComputedStyle(document.body).fontFamily,faces:[...document.fonts].filter(f=>f.family.replaceAll('"','')==='InterVariable').map(f=>f.status)}}})
const contrast=page=>page.evaluate(()=>{
  const canvas=document.createElement('canvas');canvas.width=canvas.height=1
  const context=canvas.getContext('2d',{willReadFrequently:true})
  const rgba=value=>{context.clearRect(0,0,1,1);context.fillStyle=value;context.fillRect(0,0,1,1);const p=[...context.getImageData(0,0,1,1).data];p[3]/=255;return p}
  const over=(a,b)=>{const alpha=a[3]+b[3]*(1-a[3]);return[(a[0]*a[3]+b[0]*b[3]*(1-a[3]))/alpha,(a[1]*a[3]+b[1]*b[3]*(1-a[3]))/alpha,(a[2]*a[3]+b[2]*b[3]*(1-a[3]))/alpha,alpha]}
  const luminance=color=>{const linear=value=>(value/=255)<=.04045?value/12.92:((value+.055)/1.055)**2.4;return .2126*linear(color[0])+.7152*linear(color[1])+.0722*linear(color[2])}
  const background=element=>{let result=[255,255,255,1],chain=[];for(let node=element;node;node=node.parentElement)chain.unshift(node);for(const node of chain){const style=getComputedStyle(node);if(+style.opacity!==1||style.backgroundImage!=='none')return null;result=over(rgba(style.backgroundColor),result)}return result}
  const ratio=(a,b)=>(Math.max(luminance(a),luminance(b))+.05)/(Math.min(luminance(a),luminance(b))+.05)
  let min=Infinity,failed=0,sampled=0,unknown=0
  for(const element of document.querySelectorAll('h1,p,th,td,dd,a')){
    if(!element.getClientRects().length||element.classList.contains('sr-only'))continue
    const style=getComputedStyle(element),bg=background(element)
    if(!bg){unknown++;continue}
    const fg=over(rgba(style.color),bg),value=ratio(fg,bg)
    const large=parseFloat(style.fontSize)>=24||(parseFloat(style.fontSize)>=18.66&&+style.fontWeight>=700)
    sampled++;min=Math.min(min,value);if(value<(large?3:4.5))failed++
  }
  return{sampled,failed,unknown,min:Number.isFinite(min)?min:0}
})
const pwEntry=fs.realpathSync(PLAYWRIGHT),pwPackage=snapshot(path.join(path.dirname(pwEntry),'package.json')),pw=JSON.parse(pwPackage.data),chrome=fs.realpathSync(CHROME);if(pw.version!=='1.63.0')refuse('playwright-version');const{chromium}=await import(pathToFileURL(pwEntry).href)
const focusDetails = page => page.evaluate(() => {
  const element = document.activeElement
  const style = getComputedStyle(element)
  const canvas = document.createElement('canvas')
  canvas.width = canvas.height = 1
  const context = canvas.getContext('2d', {willReadFrequently:true})
  const rgba = value => {
    context.clearRect(0, 0, 1, 1)
    context.fillStyle = value
    context.fillRect(0, 0, 1, 1)
    const pixels = [...context.getImageData(0, 0, 1, 1).data]
    pixels[3] /= 255
    return pixels
  }
  const over = (a, b) => {
    const alpha = a[3] + b[3] * (1 - a[3])
    return [0,1,2].map(i => (a[i]*a[3] + b[i]*b[3]*(1-a[3]))/alpha).concat(alpha)
  }
  const luminance = color => {
    const linear = value => (value /= 255) <= .04045 ? value/12.92 : ((value+.055)/1.055)**2.4
    return .2126*linear(color[0]) + .7152*linear(color[1]) + .0722*linear(color[2])
  }
  // The offset outline is outside the link's background. Composite every
  // ancestor background, and refuse effects this bounded sampler cannot model.
  const chain = []
  for (let node=element.parentElement; node; node=node.parentElement) chain.unshift(node)
  let background = [255,255,255,1]
  let unknown = +style.opacity !== 1
  for (const node of chain) {
    const ancestor = getComputedStyle(node)
    if (+ancestor.opacity !== 1 || ancestor.backgroundImage !== 'none') unknown = true
    background = over(rgba(ancestor.backgroundColor), background)
  }
  const foreground = over(rgba(style.outlineColor), background)
  const contrast = (Math.max(luminance(foreground),luminance(background))+.05) /
    (Math.min(luminance(foreground),luminance(background))+.05)
  return {href:element.getAttribute('href'), outline:style.outlineStyle,
    width:parseFloat(style.outlineWidth), color:style.outlineColor, contrast, unknown}
})
let browser,browserVersion='',timedOut=false;const watchdog=setTimeout(async()=>{timedOut=true;try{await browser?.close()}catch{}server.closeAllConnections?.();server.close()},120000);watchdog.unref();const close=p=>Promise.race([p,new Promise((_,reject)=>setTimeout(()=>reject(new Error('close-timeout')),5000))])
const wire=async(context,origin,tag)=>{context.setDefaultTimeout(10000);context.setDefaultNavigationTimeout(10000);context.on('serviceworker',()=>boundary.push(`${tag}:serviceworker`));context.on('page',page=>{page.on('pageerror',()=>boundary.push(`${tag}:pageerror`));page.on('console',m=>{if(['error','warning'].includes(m.type()))boundary.push(`${tag}:console`)});page.on('dialog',async d=>{boundary.push(`${tag}:dialog`);await d.dismiss().catch(()=>{})})});await context.route('**/*',route=>{const q=route.request(),u=new URL(q.url());if(u.origin===origin&&q.method()==='GET'&&!u.search&&(docs.has(u.pathname)||destinations.has(u.pathname)))return route.continue();boundary.push(`${tag}:blocked`);return route.abort('blockedbyclient')});await context.routeWebSocket('**/*',ws=>{boundary.push(`${tag}:ws`);ws.close()})}
const shot=async(page,name)=>{const file=path.join(out,name),data=await page.screenshot({path:file,fullPage:true});fs.chmodSync(file,0o600);screens.push({name,size:data.length,sha256:sha(data)})}
const mappedFaithfulBody = page => page.evaluate(() => {
  const body = document.body.cloneNode(true)
  const tail = body.lastChild
  // The exact document envelope adds one LF after </html>. HTML parsing puts
  // that host-owned LF in body. Leave it OUTSIDE the new component wrapper.
  // No component whitespace, text, attribute, class or node is normalized.
  if (!tail || tail.nodeType !== Node.TEXT_NODE || !tail.data.endsWith('\n')) return null
  tail.data = tail.data.slice(0, -1)
  if (!tail.data) tail.remove()
  const wrapper = document.createElement('div')
  wrapper.setAttribute('data-appkit-directory', '')
  while (body.firstChild) wrapper.append(body.firstChild)
  body.append(wrapper, document.createTextNode('\n'))
  return body.innerHTML
})
const textLineCounts = (page, selector) => page.evaluate(selector => {
  return [...document.querySelectorAll(selector)].filter(cell=>cell.getClientRects().length).map(cell => {
    const lines = new Set()
    for (const node of cell.childNodes) {
      if (node.nodeType !== Node.TEXT_NODE || !node.textContent.trim()) continue
      const range = document.createRange()
      range.selectNodeContents(node)
      for (const rect of range.getClientRects()) {
        if (rect.width > 1 && rect.height > 1) lines.add(Math.round(rect.top * 10))
      }
    }
    return lines.size
  })
}, selector)
const escapeRegex=value=>value.replace(/[.*+?^${}()|[\]\\]/g,'\\$&')
async function runJourney(origin, scenario, model, js, width, theme, mode, action) {
  const tag=`${scenario}-${js?'js':'nojs'}-${width}-${theme}-${mode}`
  const context=await browser.newContext({viewport:{width,height:900},colorScheme:theme,javaScriptEnabled:js,serviceWorkers:'block'})
  await wire(context,origin,tag)
  const page=await context.newPage()
  await page.goto(`${origin}/${scenario}-enhanced-${theme}`)
  const row=action.index?model.Rows[action.index-1]:null
  const accessible=row?new RegExp(`^${escapeRegex(model.EditLabel)}\\s*,\\s*${escapeRegex(row.Name)}$`):action.name
  const locator=page.getByRole('link',{name:accessible,exact:!row})
  const count=await locator.count(),href=count===1?await locator.getAttribute('href'):null
  check(`${tag} action identity ${action.index}`,count===1&&href===action.url,{count,href})
  if(count!==1||href!==action.url)refuse('action-identity')
  if(mode==='keyboard'){
    for(let index=0;index<=action.index;index++)await page.keyboard.press('Tab')
    const focus=await focusDetails(page)
    check(`${tag} focus ${action.index}`,focus.href===action.url&&focus.outline!=='none'&&focus.width>=2&&focus.contrast>=3&&!focus.unknown,focus,'quality')
    await page.keyboard.press('Enter')
  }else{
    await locator.click()
  }
  await page.waitForURL(`${origin}${action.url}`)
  const final=await page.evaluate(()=>({heading:document.querySelector('h1')?.textContent,route:document.querySelector('[data-route]')?.textContent}))
  const title=action.index?`Edit user ${action.index}`:'Create user'
  check(`${tag} activation ${action.index}`,final.heading===title&&final.route===action.url,final)
  await context.close()
}
try{
  await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve)})
  const origin=`http://127.0.0.1:${server.address().port}`;browser=await chromium.launch({headless:true,executablePath:chrome});browserVersion=browser.version()
  for (const scenario of SCENARIOS) {
    for (const width of [390,1440]) {
      for (const theme of ['light','dark']) {
        const tag=`${scenario}-${width}-${theme}`
        const context=await browser.newContext({viewport:{width,height:900},colorScheme:theme,serviceWorkers:'block'})
        await wire(context,origin,tag)
        const pages={}
        for (const kind of ['faithful','enhanced']) {
          const page=await context.newPage()
          await page.goto(`${origin}/${scenario}-${kind}-${theme}`)
          await page.evaluate(()=>document.fonts.ready)
          pages[kind]=page
          await shot(page,`${tag}-${kind}.png`)
        }
        const faithful=await inspect(pages.faithful)
        const result=await inspect(pages.enhanced)
        const model=models.get(scenario)
        const headers=[model.NameLabel,model.TitleLabel,model.EmailLabel,model.RoleLabel,model.EditLabel]
        const mapped = await mappedFaithfulBody(pages.faithful)
        check(`${tag} wrapper`,mapped!==null&&result.wrapperCount===1&&result.wrapperChildren===1&&result.bodyHTML===mapped,
          {mapped_sha256:mapped===null?null:sha(Buffer.from(mapped)), target_sha256:sha(Buffer.from(result.bodyHTML))},'fidelity')
        const whitespaceCanary = await pages.enhanced.evaluate(() => {
          const wrapper = document.querySelector('[data-appkit-directory]')
          const extra = document.createTextNode(' ')
          wrapper.prepend(extra)
          const changed = document.body.innerHTML
          extra.remove()
          return {changed, restored:document.body.innerHTML}
        })
        check(`${tag} whitespace canary`,whitespaceCanary.changed!==mapped&&whitespaceCanary.restored===result.bodyHTML,undefined,'fidelity')
        check(`${tag} semantics`,result.headings.length===1&&result.headings[0]===model.Heading&&result.introduction===model.Introduction&&result.headers.length===5&&result.headers.every((header,index)=>header.text===headers[index]&&header.scope==='col')&&result.rows.length===model.Rows.length&&result.links.length===model.Rows.length+1,undefined,'fidelity')
        check(`${tag} rows`,model.Rows.every((row,index)=>[row.Name,row.Title,row.Email,row.Role].every(value=>result.rows[index].texts.filter(text=>text===value).length===1)&&result.rows[index].labels.includes(model.TitleLabel)&&result.rows[index].labels.includes(model.EmailLabel)&&result.rows[index].links.length===1&&result.rows[index].links[0].href===row.EditURL&&result.rows[index].links[0].name.replace(/\s*,\s*/,', ')===`${model.EditLabel}, ${row.Name}`),undefined,'fidelity')
        const accessibleActions = [{name:model.AddLabel, href:model.AddURL, index:0}]
        for (const [index,row] of model.Rows.entries()) {
          accessibleActions.push({name:new RegExp(`^${escapeRegex(model.EditLabel)}\\s*,\\s*${escapeRegex(row.Name)}$`), href:row.EditURL, index:index+1})
        }
        const identities = []
        for (const action of accessibleActions) {
          const locator = pages.enhanced.getByRole('link', {name:action.name, exact:action.index===0})
          const count = await locator.count()
          identities.push({index:action.index, count, matches:count===1&&await locator.getAttribute('href')===action.href})
        }
        check(`${tag} all accessible actions`,identities.every(action=>action.matches),identities,'fidelity')
        check(`${tag} faithful inherited targets`,faithful.links.every(link=>link.rect.width>=44&&link.rect.height>=44),faithful.links,'inherited')
        check(`${tag} enhanced targets`,result.links.every(link=>link.rect.width>=44&&link.rect.height>=44),result.links,'quality')
        if (scenario !== 'long6') {
          const lines = await textLineCounts(pages.enhanced, 'thead th:nth-child(4), tbody td:nth-child(4)')
          check(`${tag} ordinary roles stay readable`,lines.length===model.Rows.length+1&&lines.every(count=>count===1),{line_counts:lines},'quality')
          const emailLines = await textLineCounts(pages.enhanced, 'tbody td:nth-child(3), tbody dd:nth-of-type(2)')
          check(`${tag} ordinary emails stay readable`,emailLines.length===model.Rows.length&&emailLines.every(count=>count===1),{line_counts:emailLines},'quality')
        }
        check(`${tag} no overflow or clipping`,!result.overflow&&!result.containerOverflow.length&&!result.clipped.length,{containers:result.containerOverflow.length,clipped:result.clipped},'quality')
        check(`${tag} font`,result.font.status==='loaded'&&result.font.family.includes('InterVariable')&&result.font.faces.length>0&&result.font.faces.every(status=>status==='loaded'),result.font,'quality')
        const ratio=await contrast(pages.enhanced)
        check(`${tag} contrast`,ratio.sampled>0&&!ratio.failed&&!ratio.unknown,ratio,'quality')
        observations.push({name:`${tag} geometry intentionally independent`,faithful:faithful.links,enhanced:result.links})
        await context.close()
      }
    }
  }
  const counters=new Map(SCENARIOS.map(x=>[x,0]))
  for(const scenario of SCENARIOS)for(const js of [false,true])for(const width of [390,1440])for(const theme of ['light','dark'])for(const mode of ['keyboard','pointer']){
    const model=models.get(scenario),actions=[{url:model.AddURL,name:model.AddLabel,index:0}]
    if(model.Rows.length){const index=counters.get(scenario)%model.Rows.length;counters.set(scenario,counters.get(scenario)+1);actions.push({url:model.Rows[index].EditURL,name:`${model.EditLabel}, ${model.Rows[index].Name}`,index:index+1})}
    for(const action of actions)await runJourney(origin,scenario,model,js,width,theme,mode,action)
  }
}catch(error){boundary.push(`operational:${error?.name||'Error'}`)}finally{clearTimeout(watchdog);try{if(browser)await close(browser.close())}catch{boundary.push('browser-close')}server.closeAllConnections?.();try{await close(new Promise(resolve=>server.close(resolve)))}catch{boundary.push('server-close')}}
if(timedOut)boundary.push('global-timeout');for(const[key,before]of inputs)try{const [group,...rest]=key.split('/'),root=group==='reference'?referenceDir:group==='code'?process.cwd():enhancedDir,after=readBounded(path.join(root,rest.join('/')));if(after.sha256!==before.sha256||after.size!==before.size)boundary.push('input-changed')}catch{boundary.push('input-changed')}
const expectedRequests=new Map([['/directory/new',64],['/directory/edit/1',12],['/directory/edit/2',11],['/directory/edit/3',11],['/directory/edit/4',6],['/directory/edit/5',4],['/directory/edit/6',4]])
for(const scenario of SCENARIOS)for(const theme of ['light','dark']){expectedRequests.set(`/${scenario}-faithful-${theme}`,2);expectedRequests.set(`/${scenario}-enhanced-${theme}`,scenario==='empty0'?10:18)}
const actualRequests=new Map();for(const request of requests)actualRequests.set(request,(actualRequests.get(request)||0)+1)
if(JSON.stringify([...actualRequests].sort())!==JSON.stringify([...expectedRequests].sort()))boundary.push('request-multiset')
screens.sort((a,b)=>a.name.localeCompare(b.name));const harness=snapshot(new URL(import.meta.url)),receipt={schema:1,case_id:'P04-table',scope:'enhanced-target-browser-probe',verification:'not-verified',generated_at:new Date().toISOString(),source:CASE,inputs:Object.fromEntries([...inputs].map(([k,v])=>[k,{size:v.size,sha256:v.sha256}])),harness:{sha256:harness.sha256,playwright_version:pw.version,playwright_entry_sha256:snapshot(pwEntry).sha256,playwright_package_sha256:pwPackage.sha256,browser_version:browserVersion},totals:{checks:checks.length,passed:checks.filter(x=>x.ok).length,boundary_failures:boundary.length,fidelity_failures:fidelity.length,enhanced_quality_failures:quality.length,inherited_failures:inherited.length,requests:requests.length,activations:checks.filter(x=>x.name.includes(' activation ')).length},boundary_failures:boundary,fidelity_failures:fidelity,enhanced_quality_failures:quality,inherited_failures:inherited,checks,observations,screenshots:screens,non_claims:['certified target','consumer adoption','real account backend','large-dataset performance','mixed-script or assistive-technology certification']};const receiptPath=path.join(out,'receipt.json');fs.writeFileSync(receiptPath,JSON.stringify(receipt,null,2)+'\n',{mode:0o600});fs.chmodSync(receiptPath,0o600);console.log(JSON.stringify({output:out,receipt:receiptPath,receipt_sha256:snapshot(receiptPath).sha256,totals:receipt.totals}));if(boundary.length||fidelity.length||quality.length)process.exitCode=1
