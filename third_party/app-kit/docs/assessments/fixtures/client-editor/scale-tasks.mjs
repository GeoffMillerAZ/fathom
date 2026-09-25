// Prospective P12 scale workload; no vendor source, arbitrary rich input or backend.
import { createHash } from 'node:crypto';

export function scaleFixture() {
  const paragraphs = Array.from({length:256},(_,i)=>{
    const prefix=`Paragraph ${String(i).padStart(3,'0')}: Synthetic catalog description. `;
    return (prefix+'Readable local editing content; '.repeat(12)).slice(0,255);
  });
  const initialText=paragraphs.join('\n');
  const sha256=createHash('sha256').update(initialText).digest('hex');
  if(Buffer.byteLength(initialText)!==65535||paragraphs.some(p=>Buffer.byteLength(p)!==255)||sha256!=='443d1a3aa090fda81133c5821064dbed9dca14ac6742483d8b1fff1e5ade41ab')throw new Error('Frozen scale fixture drifted');
  return {paragraphs,initialText,bytes:65535,sha256};
}

export async function runScaleTasks(page, context, tag, check, capture) {
  const fixture=scaleFixture();
  const result={phase:tag,fixture:{sha256:fixture.sha256,bytes:fixture.bytes,paragraphs:fixture.paragraphs.length},checks:[],latency:null,counters:null};
  const record=(name,pass,detail)=>{result.checks.push({name,pass:!!pass,detail});check(`${tag}: scale ${name}`,pass,detail);};
  const client=await context.newCDPSession(page);
  await client.send('HeapProfiler.collectGarbage');
  const before={dom:await client.send('Memory.getDOMCounters'),heap:await client.send('Runtime.getHeapUsage')};
  const mounted=await page.evaluate(async initialText=>{
    window.p12ScaleChanges=0;
    const start=performance.now();
    window.p12Scale=window.P12Specimen.mount(document.getElementById('secondary-slot'),{version:1,instanceId:'scale',label:'Large description',hint:'Synthetic 256-paragraph local draft. Nothing is saved.',initialText},()=>window.p12ScaleChanges++);
    await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));
    return {milliseconds:performance.now()-start,ready:window.p12Scale.ready,revision:window.p12Scale.snapshot().revision};
  },fixture.initialText);
  result.mount=mounted;
  const scope=page.locator('#secondary-slot');
  // Context deliberately changes the accessible name. Keep a stable locator and
  // test the name separately; a stale role locator must not make undo look empty.
  const editor=scope.locator('.tiptap');
  record('initial accessible editor exists',await editor.count()===1&&await scope.getByRole('textbox',{name:'Large description',exact:true}).count()===1);
  const preview=scope.getByRole('region',{name:'Local preview',exact:true}).locator(':scope > div');
  const firstText=await editor.locator(':scope > p').allTextContents(),firstPreview=await preview.locator(':scope > p').allTextContents();
  record('full initial document reaches editor and preview',mounted.ready&&mounted.revision===0&&JSON.stringify(firstText)===JSON.stringify(fixture.paragraphs)&&JSON.stringify(firstPreview)===JSON.stringify(fixture.paragraphs),{mount:mounted,editorParagraphs:firstText.length,previewParagraphs:firstPreview.length});
  await editor.scrollIntoViewIfNeeded();await editor.click();
  await page.keyboard.press('Meta+ArrowDown');
  const atEnd=await editor.evaluate(e=>{const selection=getSelection(),last=e.lastElementChild?.lastChild;return document.activeElement===e&&selection.isCollapsed&&selection.anchorNode===last&&selection.anchorOffset===last.textContent.length;});
  record('caret reaches document end before timing',atEnd);
  if(!atEnd)throw new Error('Scale timing prerequisite failed: caret is not at document end');
  const bounds=await editor.evaluate(e=>({clientHeight:e.clientHeight,scrollHeight:e.scrollHeight,scrollTop:e.scrollTop,overflowY:getComputedStyle(e).overflowY,pageWidth:document.documentElement.scrollWidth,viewportWidth:innerWidth}));
  record('long document scrolls locally without horizontal page overflow',bounds.scrollHeight>bounds.clientHeight&&bounds.scrollTop>0&&bounds.overflowY==='auto'&&bounds.pageWidth<=bounds.viewportWidth,bounds);
  await page.evaluate(()=>{
    window.p12ScaleSamples=[];window.p12ScaleLongTasks=[];window.p12ScaleLongSupported=PerformanceObserver.supportedEntryTypes.includes('longtask');
    if(window.p12ScaleLongSupported){window.p12ScaleLongObserver=new PerformanceObserver(list=>window.p12ScaleLongTasks.push(...list.getEntries().map(e=>({start:e.startTime,duration:e.duration}))));window.p12ScaleLongObserver.observe({type:'longtask'});}
    window.p12ScaleInput=()=>{const start=performance.now();requestAnimationFrame(()=>requestAnimationFrame(()=>window.p12ScaleSamples.push(performance.now()-start)));};
    document.querySelector('#secondary-slot .tiptap').addEventListener('input',window.p12ScaleInput);
    window.p12ScaleInputStart=performance.now();
  });
  for(let i=0;i<30;i++){await page.keyboard.type('x');await page.waitForFunction(n=>window.p12ScaleSamples.length>=n,i+1);}
  const measurements=await page.evaluate(async()=>{
    document.querySelector('#secondary-slot .tiptap').removeEventListener('input',window.p12ScaleInput);
    const end=performance.now();await new Promise(resolve=>setTimeout(resolve,0));
    const observer=window.p12ScaleLongObserver;
    if(observer){window.p12ScaleLongTasks.push(...observer.takeRecords().map(e=>({start:e.startTime,duration:e.duration})));observer.disconnect();}
    return {samples:window.p12ScaleSamples,longTasks:window.p12ScaleLongTasks.filter(e=>e.start>=window.p12ScaleInputStart&&e.start<end),longTaskSupported:window.p12ScaleLongSupported};
  });
  const sorted=[...measurements.samples].sort((a,b)=>a-b),p95=sorted[Math.ceil(sorted.length*.95)-1];
  result.latency={...measurements,p95};
  record('30 larger-document input samples meet unchanged 100ms screen',measurements.samples.length===30&&p95<=100,{p95,samples:measurements.samples});
  record('input window has no task-blocking work above 50ms',measurements.longTaskSupported&&measurements.longTasks.every(e=>e.duration<=50),{supported:measurements.longTaskSupported,tasks:measurements.longTasks});
  const expected=[...fixture.paragraphs];expected[255]+='x'.repeat(30);
  record('typing preserves all preceding paragraphs and live preview',JSON.stringify(await editor.locator(':scope > p').allTextContents())===JSON.stringify(expected)&&JSON.stringify(await preview.locator(':scope > p').allTextContents())===JSON.stringify(expected));
  await page.keyboard.press('Shift+ArrowLeft');
  await scope.getByRole('button',{name:'Bold',exact:true}).click();
  const boldBefore=await editor.locator('strong').textContent()==='x';
  const preserved=await page.evaluate(()=>{
    const handle=window.p12Scale,node=document.querySelector('#secondary-slot .tiptap'),s=getSelection();
    const before={node,active:document.activeElement,anchor:s.anchorNode,anchorOffset:s.anchorOffset,focus:s.focusNode,focusOffset:s.focusOffset,snapshot:JSON.stringify(handle.snapshot()),scrollTop:node.scrollTop};
    handle.update({version:1,instanceId:'scale',label:'Updated large description',hint:'Large local draft remains unchanged.'});
    const after=getSelection();
    return {sameNode:before.node===document.querySelector('#secondary-slot .tiptap'),sameFocus:before.active===document.activeElement,sameSelection:before.anchor===after.anchorNode&&before.anchorOffset===after.anchorOffset&&before.focus===after.focusNode&&before.focusOffset===after.focusOffset,sameDraft:before.snapshot===JSON.stringify(handle.snapshot()),sameScroll:before.scrollTop===node.scrollTop,label:node.getAttribute('aria-label')==='Updated large description'};
  });
  record('context preserves larger draft selection focus history position',Object.values(preserved).every(Boolean),preserved);
  const renamed=await editor.count()===1&&await scope.getByRole('textbox',{name:'Updated large description',exact:true}).count()===1;
  record('renamed editor remains available for history assertions',renamed);
  if(!renamed)throw new Error('Scale history prerequisite failed: renamed editor is missing');
  await page.keyboard.press('Meta+z');const undo=await editor.isVisible()&&await editor.locator('strong').count()===0&&await preview.locator('strong').count()===0;
  await page.keyboard.press('Meta+Shift+z');
  record('pre-update formatting history survives context and undo-redo',boldBefore&&undo&&await editor.locator('strong').textContent()==='x'&&await preview.locator('strong').textContent()==='x');
  await capture('large-document',editor);
  await page.evaluate(()=>{window.p12Scale.dispose();window.p12Scale=null;});
  const cycles=[];
  for(let i=0;i<10;i++){
    await page.evaluate(i=>{
      window.p12CycleChanges=0;
      window.p12Cycle=window.P12Specimen.mount(document.getElementById('secondary-slot'),{version:1,instanceId:'cycle',label:'Cycle editor',hint:'Synthetic teardown check',initialText:`Cycle ${i}`},()=>window.p12CycleChanges++);
    },i);
    const box=scope.getByRole('textbox',{name:'Cycle editor',exact:true});await box.click();await page.keyboard.press('Meta+ArrowRight');await page.keyboard.type(' edited');
    const facts=await page.evaluate(i=>{
      const handle=window.p12Cycle,before=handle.snapshot();const node=document.querySelector('#secondary-slot .tiptap');
      handle.dispose();const count=window.p12CycleChanges;let refused=false;try{handle.snapshot();}catch{refused=true;}
      node.dispatchEvent(new InputEvent('input',{bubbles:true,data:'late'}));document.dispatchEvent(new Event('selectionchange'));
      window.p12Cycle=null;
      return {text:before.text,changes:count,empty:document.getElementById('secondary-slot').childNodes.length===0,refused,noLateCallbacks:window.p12CycleChanges===count};
    },i);
    await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
    facts.noLateCallbacks=facts.noLateCallbacks&&await page.evaluate(count=>window.p12CycleChanges===count,facts.changes);
    cycles.push(facts);
  }
  record('ten independent mount-edit-dispose cycles retain no active instance',cycles.length===10&&cycles.every((c,i)=>c.text===`Cycle ${i} edited`&&c.changes>0&&c.empty&&c.refused&&c.noLateCallbacks),cycles);
  await page.evaluate(()=>{for(const name of ['p12ScaleInput','p12ScaleLongObserver','p12ScaleLongTasks','p12ScaleSamples','p12ScaleInputStart','p12Cycle','p12Scale'])delete window[name];});
  await client.send('HeapProfiler.collectGarbage');
  const after={dom:await client.send('Memory.getDOMCounters'),heap:await client.send('Runtime.getHeapUsage')};
  result.counters={before,after,delta:{nodes:after.dom.nodes-before.dom.nodes,listeners:after.dom.jsEventListeners-before.dom.jsEventListeners,usedHeap:after.heap.usedSize-before.heap.usedSize},scope:'explicit-GC Chrome diagnostics, not a leak certification'};
  await client.detach();
  return result;
}
