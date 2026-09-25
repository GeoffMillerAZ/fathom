// Synthetic browser interactions only. No vendor source, writes or backend task.
export const editorCheckNames = [
  'initial document, accessible name and disabled history agree',
  'all thirteen native controls named and touch-sized without overlap',
  'native Tab order and visible focus cover enabled controls',
  'keyboard Space applies bold and keeps selected document',
  'keyboard undo and redo agree with preview and button state',
  'italic, underline, headings and lists roundtrip with pressed state',
  'code defaults to plaintext with marks disabled',
  'five explicit code languages serialize; supported syntax is highlighted',
  'long code scrolls locally without page overflow',
  'link cancel, set and remove preserve selection and preview',
  'external image URL rejected without document mutation',
  'image cancellation and explicit alt insertion agree with preview',
  'image undo and redo remove and restore both surfaces',
  'long prose wraps without page overflow',
  'empty document and preview agree',
  'changed draft demonstrably resets on reload',
];
const names = ['Bold','Italic','Underline','Heading 1','Heading 2','Heading 3','Bulleted list','Numbered list','Set or remove link','Insert image','Code block','Undo','Redo'];
export async function runEditorTasks(page, base, phase, check, captureState) {
  const editor = page.getByRole('textbox', { name: 'Product description', exact: true });
  const preview = page.getByRole('region', { name: 'Local preview', exact: true }).locator(':scope > div');
  const button = name => page.locator('.toolbar').getByRole('button', { name, exact: true });
  const facts = [];
  const record = (name, pass, detail) => { facts.push({name,pass:!!pass,detail}); check(`${phase}: ${name}`,pass,detail); };
  const agree = async text => (await editor.textContent()) === text && (await preview.textContent()) === text;
  const selectAll = async () => { await editor.click(); await page.keyboard.press('Meta+a'); };
  const type = async text => { await selectAll(); await page.keyboard.type(text); };
  // The code samples are single lines. Select their text, not the entire
  // document node (native Select All + typing correctly replaces the block).
  const typeCode = async text => { await editor.locator('pre code').click(); await page.keyboard.press('Meta+ArrowLeft'); await page.keyboard.press('Meta+Shift+ArrowRight'); await page.keyboard.type(text); };
  const editorFocused = () => page.waitForFunction(()=>document.activeElement===document.querySelector('.tiptap'));
  const attr = async (name, value) => await button(name).getAttribute('aria-pressed') === String(value);
  const dialogAction = async (name, replies) => {
    const messages = [], errors = [];
    const handler = async dialog => {
      const index = messages.length; messages.push(dialog.message());
      try { if (index >= replies.length) { errors.push('unexpected dialog'); await dialog.dismiss(); }
        else if (replies[index] === null) await dialog.dismiss(); else await dialog.accept(replies[index]);
      } catch (error) { errors.push(String(error)); }
    };
    page.on('dialog', handler);
    try { await button(name).click(); } finally { page.off('dialog', handler); }
    if (messages.length !== replies.length || errors.length) throw new Error('Dialog sequence differs: ' + JSON.stringify({messages,errors}));
    return messages;
  };
  record(editorCheckNames[0], await agree('Start typing...') && await button('Undo').isDisabled() && await button('Redo').isDisabled()
    && await editor.getAttribute('aria-multiline') === 'true' && await editor.evaluate(e=>!!document.getElementById(e.getAttribute('aria-describedby'))));
  const controls = await page.locator('.toolbar button').evaluateAll(bs => bs.map(b => {
    const r = b.getBoundingClientRect(); return {name:b.getAttribute('aria-label'),type:b.type,x:r.x,y:r.y,width:r.width,height:r.height,right:r.right,bottom:r.bottom};
  }));
  const overlap = controls.some((a,i) => controls.slice(i+1).some(b => Math.min(a.right,b.right)-Math.max(a.x,b.x)>0.5 && Math.min(a.bottom,b.bottom)-Math.max(a.y,b.y)>0.5));
  const toolbarFits = await page.locator('.toolbar').evaluate(t => t.scrollWidth <= t.clientWidth && document.documentElement.scrollWidth <= innerWidth);
  record(editorCheckNames[1], JSON.stringify(controls.map(b=>b.name)) === JSON.stringify(names) && controls.every(b=>b.type==='button'&&b.width>=44&&b.height>=44) && !overlap && toolbarFits, controls);
  await page.keyboard.press('Tab'); await button('Bold').focus();
  const focus = [];
  for (const name of names.slice(0,11)) {
    focus.push(await page.evaluate(() => { const el=document.activeElement,s=getComputedStyle(el); return {name:el.getAttribute('aria-label'),outline:s.outlineStyle,width:s.outlineWidth}; }));
    await page.keyboard.press('Tab');
  }
  record(editorCheckNames[2], JSON.stringify(focus.map(f=>f.name)) === JSON.stringify(names.slice(0,11)) && focus.every(f=>f.outline!=='none'&&parseFloat(f.width)>=2) && await editor.evaluate(e=>e===document.activeElement), focus);
  const text = 'Synthetic catalog note';
  await type(text); await selectAll(); await button('Bold').focus(); await page.keyboard.press('Space');
  await editorFocused();
  record(editorCheckNames[3], await agree(text) && await editor.locator('strong').textContent() === text && await preview.locator('strong').textContent() === text && await attr('Bold',true));
  await page.keyboard.press('Meta+z');
  const undone = await agree(text) && await editor.locator('strong').count()===0 && await preview.locator('strong').count()===0 && await attr('Bold',false) && await button('Redo').isEnabled();
  await button('Undo').focus(); await page.keyboard.press('Tab');
  const redoFocus = await button('Redo').evaluate(e=>e===document.activeElement&&getComputedStyle(e).outlineStyle!=='none');
  await page.keyboard.press('Space');
  await editorFocused();
  const toolbarRedo = await editor.locator('strong').count()===1&&await preview.locator('strong').count()===1;
  await page.keyboard.press('Meta+z');
  await page.keyboard.press('Meta+Shift+z');
  const history={undone,redoFocus,toolbarRedo,editorRestored:await editor.locator('strong').count()===1,previewRestored:await preview.locator('strong').count()===1,pressed:await attr('Bold',true),redoDisabled:await button('Redo').isDisabled()};
  record(editorCheckNames[4],Object.values(history).every(Boolean),history);
  await button('Bold').click();
  const formatting = [];
  for (const [name,tag] of [['Italic','em'],['Underline','u'],['Heading 1','h1'],['Heading 2','h2'],['Heading 3','h3'],['Bulleted list','ul'],['Numbered list','ol']]) {
    await selectAll(); await button(name).click();
    const on = await agree(text) && await editor.locator(tag).count()===1 && await preview.locator(tag).count()===1 && await attr(name,true);
    await button(name).click();
    formatting.push({name,on,off:await agree(text)&&await editor.locator(tag).count()===0&&await preview.locator(tag).count()===0&&await attr(name,false)});
  }
  record(editorCheckNames[5], formatting.every(f=>f.on&&f.off), formatting);
  await type('const answer = 42;'); await button('Code block').click();
  const language = page.getByRole('combobox', {name:'Code language',exact:true});
  await button('Code block').focus(); await page.keyboard.press('Tab');
  const undoFocus = await button('Undo').evaluate(e=>e===document.activeElement&&getComputedStyle(e).outlineStyle!=='none');
  await page.keyboard.press('Tab');
  const languageFocus = await language.evaluate(e=>e===document.activeElement&&getComputedStyle(e).outlineStyle!=='none');
  const codeInitial={value:await language.inputValue(),serialized:await preview.locator('pre code').getAttribute('class'),undoFocus,languageFocus,
    plain:await editor.locator('pre code .hljs-keyword').count()===0,pressed:await attr('Code block',true),boldDisabled:await button('Bold').isDisabled(),italicDisabled:await button('Italic').isDisabled(),underlineDisabled:await button('Underline').isDisabled()};
  record(editorCheckNames[6], codeInitial.value==='plaintext'&&codeInitial.serialized==='language-plaintext'&&Object.values(codeInitial).filter(v=>typeof v==='boolean').every(Boolean),codeInitial);
  const languages = [];
  for (const [value,sample] of [['javascript','const answer = 42;'],['typescript','const answer: number = 42;'],['json','{"answer": 42}'],['go','package main'],['plaintext','plain text']]) {
    await typeCode(sample); await language.selectOption(value);
    const spans = await editor.locator('pre code [class^="hljs-"]').count();
    languages.push({value,agrees:await agree(sample),serialized:await preview.locator('pre code').getAttribute('class')===`language-${value}`,highlighted:value==='plaintext'?spans===0:spans>0});
  }
  record(editorCheckNames[7], languages.every(l=>l.agrees&&l.serialized&&l.highlighted), languages);
  await typeCode('const veryLong = "'+'x'.repeat(800)+'";'); await language.selectOption('javascript');
  const codeBounds = await page.locator('.editor-container pre').evaluateAll(es=>({page:document.documentElement.scrollWidth,viewport:innerWidth,blocks:es.map(e=>({width:e.clientWidth,scroll:e.scrollWidth,overflow:getComputedStyle(e).overflowX}))}));
  record(editorCheckNames[8], codeBounds.blocks.length===2&&codeBounds.page<=codeBounds.viewport&&codeBounds.blocks.every(b=>b.scroll>b.width&&b.overflow==='auto'),codeBounds);
  await captureState('code');
  await button('Code block').click(); await type(text); await selectAll();
  const beforeLink = await preview.innerHTML();
  await dialogAction('Set or remove link',[null]);
  const cancelled = await preview.innerHTML()===beforeLink;
  await dialogAction('Set or remove link',['#editor-reference']);
  const linked = await editor.locator('a').getAttribute('href')==='#editor-reference' && await preview.locator('a').getAttribute('href')==='#editor-reference' && await attr('Set or remove link',true);
  await dialogAction('Set or remove link',['']);
  record(editorCheckNames[9], cancelled&&linked&&await agree(text)&&await editor.locator('a').count()===0&&await preview.locator('a').count()===0&&await attr('Set or remove link',false));
  const beforeImage = await preview.innerHTML();
  const refused=[];
  for(const input of ['https://example.invalid/never-requested.png',base.replace('http://','http://synthetic:synthetic@')+'/image.png','data:image/png;base64,AAAA','javascript:void(0)']){
    await dialogAction('Insert image',[input]);
    refused.push(await preview.innerHTML()===beforeImage && await page.getByRole('status').filter({hasText:'Image not inserted.'}).count()===1);
  }
  record(editorCheckNames[10], refused.length===4&&refused.every(Boolean),{cases:refused});
  await dialogAction('Insert image',[null]);
  const localImage = base + '/assets/dash-prd-2-C01gIwt2.jpg';
  await dialogAction('Insert image',[localImage,null]);
  const imageCancelled = await preview.innerHTML()===beforeImage;
  await dialogAction('Insert image',[localImage,'Synthetic local product']);
  await page.waitForFunction(()=>[...document.querySelectorAll('.editor-container img')].length===2&&[...document.querySelectorAll('.editor-container img')].every(i=>i.complete&&i.naturalWidth>0));
  record(editorCheckNames[11], imageCancelled && await editor.locator('img').getAttribute('src')===localImage && await preview.locator('img').getAttribute('src')===localImage
    && await editor.locator('img').getAttribute('alt')==='Synthetic local product' && await preview.locator('img').getAttribute('alt')==='Synthetic local product');
  await button('Undo').click();
  const imageUndo = await editor.locator('img').count()===0&&await preview.locator('img').count()===0&&await button('Redo').isEnabled();
  await button('Redo').click();
  record(editorCheckNames[12], imageUndo&&await editor.locator('img').count()===1&&await preview.locator('img').count()===1&&await button('Redo').isDisabled()
    &&await editor.locator('img').getAttribute('src')===localImage&&await preview.locator('img').getAttribute('src')===localImage
    &&await editor.locator('img').getAttribute('alt')==='Synthetic local product'&&await preview.locator('img').getAttribute('alt')==='Synthetic local product');
  await captureState('image');
  const long = 'LongProse' + 'x'.repeat(800); await type(long);
  const proseBounds = await page.locator('.editor-container').evaluate(e=>({page:document.documentElement.scrollWidth,viewport:innerWidth,width:e.clientWidth,scroll:e.scrollWidth}));
  record(editorCheckNames[13],await agree(long)&&proseBounds.page<=proseBounds.viewport&&proseBounds.scroll<=proseBounds.width,proseBounds);
  await selectAll(); await page.keyboard.press('Backspace');
  record(editorCheckNames[14], await agree('')&&await preview.innerHTML()==='<p></p>');
  await type('Changed draft before reload');
  const dirty = await agree('Changed draft before reload')&&await button('Undo').isEnabled();
  await page.reload({waitUntil:'networkidle'});
  record(editorCheckNames[15], dirty&&await agree('Start typing...')&&await button('Undo').isDisabled()&&await button('Redo').isDisabled());
  return facts;
}
