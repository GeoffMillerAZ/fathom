import { createHash } from 'node:crypto';

// Actual pinned-runtime HTTP morph, plus separately labeled transport fault injection.
export async function runMorphTasks(page, base, tag, check, capture, requests, expectedFaults) {
  const result={phase:tag,checks:[],wire:null,faults:[]};
  const record=(name,pass,detail)=>{result.checks.push({name,pass:!!pass,detail});check(`${tag}: morph ${name}`,pass,detail);};
  await page.waitForFunction(()=>window.P12Morph && window.P12Specimen.primary.ready);
  const editor=page.locator('#editor-slot .tiptap');
  const draft='Datastar preserves rich local drafts';
  await editor.click();await page.keyboard.press('Meta+a');await page.keyboard.type(draft);
  await page.keyboard.press('Meta+a');await page.getByRole('button',{name:'Bold',exact:true}).click();
  const initial=await page.locator('#fragment-state').textContent();
  await page.evaluate(()=>{
    const selection=getSelection();
    window.p12MorphBefore={slot:document.getElementById('editor-slot'),node:document.querySelector('#editor-slot .tiptap'),active:document.activeElement,
      anchor:selection.anchorNode,anchorOffset:selection.anchorOffset,focus:selection.focusNode,focusOffset:selection.focusOffset,
      snapshot:JSON.stringify(window.P12Specimen.primary.snapshot()),changes:window.P12Specimen.changes};
  });
  const responsePromise=page.waitForResponse(response=>response.url()===base+'/morph-fragment');
  const requestStart=requests.length;
  const outcome=await page.evaluate(()=>window.P12Morph.request());
  const response=await responsePromise,body=await response.body(),request=response.request();
  const headers=await response.allHeaders(),requestHeaders=await request.allHeaders();
  result.wire={url:request.url(),method:request.method(),body:request.postData(),requestHeaders,status:response.status(),headers,bytes:body.length,sha256:createHash('sha256').update(body).digest('hex'),text:body.toString('utf8'),outcome};
  const expected='<section id="host-fragment"><p id="fragment-state">Updated surrounding context.</p><div id="editor-slot" data-ignore-morph></div></section>';
  record('one actual GET delivers the declared outer fragment',requests.slice(requestStart).length===1&&request.method()==='GET'&&request.postData()===null&&request.url()===base+'/morph-fragment'&&requestHeaders['datastar-request']==='true'&&response.status()===200&&headers['content-type']==='text/html; charset=utf-8'&&headers['datastar-selector']==='#host-fragment'&&headers['datastar-mode']==='outer'&&body.toString('utf8')===expected,result.wire);
  const preserved=await page.evaluate(()=>{
    const before=window.p12MorphBefore,s=getSelection();
    return {slot:before.slot.isConnected&&before.slot===document.getElementById('editor-slot'),node:before.node.isConnected&&before.node===document.querySelector('#editor-slot .tiptap'),
      focus:before.active===document.activeElement,selection:before.anchor===s.anchorNode&&before.anchorOffset===s.anchorOffset&&before.focus===s.focusNode&&before.focusOffset===s.focusOffset,
      snapshot:before.snapshot===JSON.stringify(window.P12Specimen.primary.snapshot()),callbacks:before.changes===window.P12Specimen.changes};
  });
  record('surrounding context changes without touching live editing state',initial==='Initial surrounding context.'&&await page.locator('#fragment-state').textContent()==='Updated surrounding context.'&&outcome.outcome==='updated'&&Object.values(preserved).every(Boolean),preserved);
  await page.keyboard.press('Meta+z');
  const undone=await editor.evaluate(e=>e.isConnected&&e.querySelectorAll('strong').length===0);
  await page.keyboard.press('Meta+Shift+z');
  record('pre-morph formatting remains undoable and redoable',undone&&await editor.locator('strong').textContent()===draft&&await page.locator('#editor-slot').getByRole('region',{name:'Local preview',exact:true}).locator('strong').textContent()===draft);
  const localStart=requests.length;
  await page.keyboard.press('Meta+ArrowRight');await page.keyboard.type('!');
  await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
  record('continued typing stays local after actual morph',requests.length===localStart&&await editor.textContent()===draft+'!');
  await capture('datastar-morph');

  const snapshot=()=>page.evaluate(()=>JSON.stringify(window.P12Specimen.primary.snapshot()));
  const stable=await snapshot();
  const button=page.getByRole('button',{name:'Update surrounding context',exact:true});
  const clicked=page.waitForResponse(response=>response.url()===base+'/morph-fragment');
  await button.focus();await page.keyboard.press('Enter');await clicked;
  await page.waitForFunction(()=>!window.P12Morph.pending);
  record('keyboard button uses the same runtime without changing draft',await snapshot()===stable&&await page.locator('#host-state').textContent()==='Surrounding context updated. Local draft unchanged.');

  const url=base+'/morph-fragment';
  const fault={phase:tag,url,status:503,fulfilled:0};expectedFaults.push(fault);
  await page.route(url,async route=>{await route.fulfill({status:503,contentType:'text/plain',body:'synthetic unavailable\n'});fault.fulfilled++;});
  const errorStart=requests.length;
  try {
    const failed=await page.evaluate(()=>window.P12Morph.request());result.faults.push({kind:'intercepted-503',result:failed});
    record('synthetic 503 leaves draft intact with no retries',fault.fulfilled===1&&failed.outcome==='failed'&&failed.events.some(e=>e.type==='error'&&e.args.status==='503')&&await snapshot()===stable&&requests.length-errorStart===1,failed);
  } finally { await page.unroute(url); }

  let release,arrived,delivered,timer;
  const held=new Promise(resolve=>arrived=resolve),delivery=new Promise(resolve=>release=resolve);
  const deliveryDone=new Promise(resolve=>delivered=resolve);
  await page.route(url,async route=>{arrived();await delivery;await route.fulfill({status:200,contentType:'text/html',headers:{'Datastar-Selector':'#host-fragment','Datastar-Mode':'outer'},body:expected}).catch(()=>{});delivered();});
  try {
    const pending=page.evaluate(()=>window.P12Morph.request());
    await Promise.race([held,new Promise((_,reject)=>{timer=setTimeout(()=>reject(new Error('Held morph request did not arrive')),5000);})]);clearTimeout(timer);
    const busy=await page.evaluate(()=>window.P12Morph.request());
    await page.evaluate(()=>window.P12Morph.cancel());
    const cancelled=await pending;result.faults.push({kind:'intercepted-held-response-cancel',result:cancelled});
    release();await deliveryDone;await page.unroute(url);
    await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
    record('explicit cancellation and duplicate refusal preserve draft',busy.outcome==='busy'&&cancelled.outcome==='cancelled'&&await snapshot()===stable&&!await page.evaluate(()=>window.P12Morph.pending),{busy,cancelled});
  } finally {clearTimeout(timer);release();await page.unroute(url);}

  await page.evaluate(()=>window.P12Specimen.primary.dispose());
  record('explicit disposal still empties the preserved owner slot',await page.locator('#editor-slot').evaluate(e=>e.childNodes.length===0));
  await page.getByRole('button',{name:'Mount editor',exact:true}).click();
  await page.waitForFunction(()=>window.P12Specimen.primary.ready);
  record('post-morph remount starts the explicit initial document',await editor.textContent()==='Start typing...'&&await page.evaluate(()=>window.P12Specimen.primary.snapshot().revision===0));
  await page.evaluate(()=>{delete window.p12MorphBefore;});
  return result;
}
