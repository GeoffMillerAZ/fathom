import assert from 'node:assert/strict';
import test from 'node:test';
import fs from 'node:fs';
import {availableMenuBlockSize, mountMenuRepair} from './menu_repair.mjs';

class Events {
  listeners = new Map();
  addEventListener(name, fn) {const list=this.listeners.get(name)||new Set();list.add(fn);this.listeners.set(name,list);}
  removeEventListener(name, fn) {this.listeners.get(name)?.delete(fn);}
  emit(name, event = {}) {for(const fn of this.listeners.get(name)||[])fn(event);}
  get count() {return [...this.listeners.values()].reduce((n,s)=>n+s.size,0);}
}
function fixture() {
  const win=new Events(),doc=new Events(),root=new Events(),menu=new Events(),visual=new Events();
  const frames=new Map(),microtasks=[],style=new Map(),attrs=new Map(),observers=[];let next=0;
  win.innerHeight=500;visual.offsetTop=0;visual.height=500;win.visualViewport=visual;
  win.requestAnimationFrame=fn=>{frames.set(++next,fn);return next;};win.cancelAnimationFrame=id=>frames.delete(id);
  win.queueMicrotask=fn=>microtasks.push(fn);
  win.ResizeObserver=class {nodes=[];constructor(fn){this.callback=fn;observers.push(this);}observe(n){this.nodes.push(n);}disconnect(){this.nodes=[];}};
  win.MutationObserver=class {nodes=[];constructor(fn){this.callback=fn;observers.push(this);}observe(n,options){this.nodes.push(n);this.options=options;}disconnect(){this.nodes=[];}};
  const trigger={rect:{top:72,bottom:128},getBoundingClientRect(){return this.rect;}};
  const items=Array.from({length:4},(_,i)=>({tabIndex:i===0?0:-1,disabled:false,aria:null,focusCalls:0,
    getAttribute(name){return name==='aria-disabled'?this.aria:null;},focus(options){this.focusCalls++;this.options=options;doc.activeElement=this;}}));
  menu.open=true;menu.matches=s=>s===':popover-open'&&menu.open;menu.querySelectorAll=()=>items;
  menu.style={getPropertyValue:p=>style.get(p)?.value||'',getPropertyPriority:p=>style.get(p)?.priority||'',setProperty:(p,value,priority='')=>style.set(p,{value,priority}),removeProperty:p=>style.delete(p)};
  const dropdown={querySelector:s=>s===':scope > button'?trigger:s===':scope > el-menu'?menu:null};
  root.matches=s=>s==='[data-appkit-menu]';root.isConnected=true;root.ownerDocument=doc;
  root.querySelectorAll=()=>[dropdown];root.getAttribute=p=>attrs.get(p)??null;
  root.setAttribute=(p,v)=>attrs.set(p,v);root.removeAttribute=p=>attrs.delete(p);
  doc.defaultView=win;doc.activeElement=trigger;
  const flush=()=>{const queue=[...frames.values()];frames.clear();for(const fn of queue)fn();};
  const drain=()=>{while(microtasks.length)microtasks.shift()();};
  return {win,doc,root,menu,visual,trigger,items,frames,style,attrs,observers,flush,drain,
    containerFocus(){doc.activeElement=menu;root.emit('focusin',{target:menu});}};
}
test('source long-menu geometry reserves the anchored gap and edge',()=>{
  assert.equal(availableMenuBlockSize({top:72,bottom:128},{top:0,height:500}),348);
  assert.equal(availableMenuBlockSize({top:400,bottom:444},{top:0,height:500}),376);
  assert.equal(availableMenuBlockSize({top:172,bottom:228},{top:100,height:500}),348);
  assert.equal(availableMenuBlockSize({top:0,bottom:20},{top:0,height:20}),0);
});
test('geometry refuses nonfinite, negative and inverted inputs',()=>{
  for(const n of [NaN,Infinity,-Infinity])assert.throws(()=>availableMenuBlockSize({top:n,bottom:128},{top:0,height:500}),RangeError);
  for(const args of [[{top:2,bottom:1},{top:0,height:500}], [{top:0,bottom:1},{top:0,height:-1}], [{top:0,bottom:1},{top:0,height:5},-1], [{top:0,bottom:1},{top:0,height:5},8,-1]])assert.throws(()=>availableMenuBlockSize(...args),RangeError);
});
test('mount is explicit and changing only the scoped marker and size',()=>{
  const f=fixture(),dispose=mountMenuRepair(f.root);
  assert.equal(f.style.get('--appkit-menu-available-block').value,'348px');
  assert.equal(f.attrs.get('data-appkit-menu-repair'),'active');
  assert.deepEqual(f.items.map(x=>x.tabIndex),[0,-1,-1,-1]);assert.equal(f.doc.activeElement,f.trigger);
  assert.equal(f.root.listeners.has('keydown'),false);assert.equal(f.root.listeners.has('click'),false);dispose();
});
test('late container focus restores the selection Elements already owns',()=>{
  const f=fixture(),dispose=mountMenuRepair(f.root);f.containerFocus();assert.equal(f.doc.activeElement,f.menu);
  f.drain();assert.equal(f.doc.activeElement,f.items[0]);assert.deepEqual(f.items[0].options,{preventScroll:true});dispose();
});
test('pending focus reads current selection, never an old item index',()=>{
  const f=fixture(),dispose=mountMenuRepair(f.root);f.containerFocus();f.items[0].tabIndex=-1;f.items[2].tabIndex=0;
  f.drain();assert.equal(f.doc.activeElement,f.items[2]);dispose();
});
for(const state of ['closed','disconnected','focus-moved','no-selection','ambiguous','disabled','aria-disabled','disposed'])test('focus repair refuses '+state,()=>{
  const f=fixture(),dispose=mountMenuRepair(f.root);f.containerFocus();
  if(state==='closed')f.menu.open=false;if(state==='disconnected')f.root.isConnected=false;
  if(state==='focus-moved')f.doc.activeElement=f.trigger;if(state==='no-selection')f.items[0].tabIndex=-1;
  if(state==='ambiguous')f.items[1].tabIndex=0;if(state==='disabled')f.items[0].disabled=true;
  if(state==='aria-disabled')f.items[0].aria='true';if(state==='disposed')dispose();
  f.drain();assert.equal(f.items.reduce((n,x)=>n+x.focusCalls,0),0);dispose();
});
test('resize, scroll, toggles and observers coalesce to one cancellable frame',()=>{
  const f=fixture(),dispose=mountMenuRepair(f.root);
  f.trigger.rect={top:400,bottom:444};f.win.emit('resize');f.doc.emit('scroll');f.visual.emit('resize');f.visual.emit('scroll');f.menu.emit('toggle');f.menu.emit('beforetoggle');f.observers[0].callback();
  assert.equal(f.frames.size,1);f.flush();assert.equal(f.style.get('--appkit-menu-available-block').value,'376px');
  f.win.emit('resize');dispose();assert.equal(f.frames.size,0);
  assert.equal(f.root.count+f.menu.count+f.win.count+f.doc.count+f.visual.count,0);assert.equal(f.observers[0].nodes.length,0);
});
test('runtime position-only updates refresh the cap without observing the document',()=>{
  const f=fixture(),dispose=mountMenuRepair(f.root),position=f.observers[1];
  assert.deepEqual(position.nodes,[f.menu]);assert.deepEqual(position.options,{attributes:true,attributeFilter:['style']});
  f.trigger.rect={top:400,bottom:444};position.callback();assert.equal(f.frames.size,1);f.flush();
  assert.equal(f.style.get('--appkit-menu-available-block').value,'376px');
  dispose();assert.equal(position.nodes.length,0);position.callback();assert.equal(f.frames.size,0);
});
test('disposal restores exact prior value, priority and marker and permits remount',()=>{
  const f=fixture();f.menu.style.setProperty('--appkit-menu-available-block','123px','important');f.root.setAttribute('data-appkit-menu-repair','previous');
  const dispose=mountMenuRepair(f.root);assert.throws(()=>mountMenuRepair(f.root),TypeError);dispose();dispose();
  assert.deepEqual(f.style.get('--appkit-menu-available-block'),{value:'123px',priority:'important'});assert.equal(f.attrs.get('data-appkit-menu-repair'),'previous');
  mountMenuRepair(f.root)();assert.equal(f.attrs.get('data-appkit-menu-repair'),'previous');
});
test('unowned attributes and styles survive cleanup; absent owned values stay absent',()=>{
  const f=fixture();f.style.set('color',{value:'red',priority:''});f.attrs.set('data-app','keep');const dispose=mountMenuRepair(f.root);dispose();
  assert.deepEqual([...f.style.keys()],['color']);assert.deepEqual([...f.attrs.keys()],['data-app']);
});
test('two mounted roots keep selection and viewport state separate',()=>{
  const a=fixture(),b=fixture(),da=mountMenuRepair(a.root),db=mountMenuRepair(b.root);a.containerFocus();a.drain();
  assert.equal(a.doc.activeElement,a.items[0]);assert.equal(b.doc.activeElement,b.trigger);da();assert.equal(b.attrs.get('data-appkit-menu-repair'),'active');db();
});
test('bad markup and unavailable observers fail before mutating ownership',()=>{
  for(const mutate of [f=>f.root.isConnected=false,f=>f.root.querySelectorAll=()=>[],f=>f.win.ResizeObserver=undefined]){
    const f=fixture();mutate(f);assert.throws(()=>mountMenuRepair(f.root),TypeError);assert.equal(f.attrs.size,0);assert.equal(f.style.size,0);
  }
});
test('setup failure rolls back listeners, marker and variable',()=>{
  const f=fixture();f.visual.addEventListener=()=>{throw Error('injected listener failure');};
  assert.throws(()=>mountMenuRepair(f.root),/injected listener failure/);
  assert.equal(f.attrs.size,0);assert.equal(f.style.size,0);assert.equal(f.root.count+f.menu.count+f.win.count+f.doc.count,0);
});
test('companion CSS is scoped and does not replace source positioning or selection',()=>{
  const css=fs.readFileSync(new URL('./menu_repair.css',import.meta.url),'utf8');
  assert.match(css,/\[data-appkit-menu\]\[data-appkit-menu-repair="active"\] el-menu/);
  assert.match(css,/max-block-size: min\(calc\(100dvh - 2rem\), var\(--appkit-menu-available-block\)\)/);
  assert.doesNotMatch(css,/(?:position|top|left|inset|display|visibility)\s*:/);
});
