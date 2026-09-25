// Named-font evidence for this controlled Inter specimen, not a generic a11y API.
// FontFaceSet.check is diagnostic: unused eligible fallback faces can make it
// false, and missing families can make it true. Inspect rendered glyphs instead.
export function inspectPaintedFonts(nodes) {
  const failures=[];
  if(!Array.isArray(nodes)||nodes.length===0||nodes.length>512)return ['expected 1..512 visible text-parent records'];
  const seen=new Set();
  for(const node of nodes){
    if(!node||typeof node.selector!=='string'||!node.selector||seen.has(node.selector))failures.push('missing or duplicate text-parent selector');
    else seen.add(node.selector);
    if(!Array.isArray(node?.fonts)||!node.fonts.length){failures.push('no painted font for '+node?.selector);continue}
    for(const font of node.fonts)if(font?.isCustomFont!==true||font.familyName!=='Inter Variable'||typeof font.postScriptName!=='string'||!font.postScriptName.startsWith('InterVariable')||!Number.isSafeInteger(font.glyphCount)||font.glyphCount<=0)failures.push('unexpected painted font for '+node.selector+': '+JSON.stringify(font));
  }
  return failures;
}

export async function paintedFonts(page,cdp) {
  const selectors=await page.evaluate(()=>{
    const parents=new Set(),walker=document.createTreeWalker(document.body,NodeFilter.SHOW_TEXT);
    while(walker.nextNode()){
      const text=walker.currentNode,el=text.parentElement;
      if(!text.textContent.trim()||!el||['SCRIPT','STYLE'].includes(el.tagName)||getComputedStyle(el).visibility!=='visible')continue;
      const range=document.createRange();range.selectNodeContents(text);
      if([...range.getClientRects()].some(r=>r.width>0&&r.height>0))parents.add(el);
    }
    return [...parents].map(el=>{
      const parts=[];
      while(el!==document.body){parts.unshift(el.localName+':nth-child('+([...el.parentElement.children].indexOf(el)+1)+')');el=el.parentElement}
      return ['body',...parts].join(' > ');
    });
  });
  if(!selectors.length||selectors.length>512)return {nodes:[],failures:['expected 1..512 visible text parents']};
  await cdp.send('DOM.enable');await cdp.send('CSS.enable');
  const doc=await cdp.send('DOM.getDocument'),nodes=[];
  for(const selector of selectors){
    const {nodeId}=await cdp.send('DOM.querySelector',{nodeId:doc.root.nodeId,selector});
    const result=nodeId?await cdp.send('CSS.getPlatformFontsForNode',{nodeId}):{fonts:[]};
    nodes.push({selector,fonts:result.fonts??[]});
  }
  return {nodes,failures:inspectPaintedFonts(nodes)};
}
