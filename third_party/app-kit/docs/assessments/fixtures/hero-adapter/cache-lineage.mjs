// Prospective synthetic validator. It performs no I/O and cannot authenticate facts.
import {inspectPaintedFonts} from './font-painted.mjs';

const immutable='private,max-age=3600,immutable', noStore='no-store';
const missingBodyError={name:'Error',command:'Network.getResponseBody',message:'cdpSession.send: Protocol error (Network.getResponseBody): No data found for resource with given identifier'};
const sha=value=>typeof value==='string'&&/^[0-9a-f]{64}$/.test(value);
const plain=value=>value!==null&&typeof value==='object'&&!Array.isArray(value);
const text=value=>typeof value==='string'&&value.length>0&&value.length<=8192;
const same=(a,b)=>JSON.stringify(a)===JSON.stringify(b);
const sameMap=(a,b)=>headers(a)&&headers(b)&&same(Object.keys(a).sort(),Object.keys(b).sort())&&Object.keys(a).every(key=>a[key]===b[key]);
const pathOf=(origin,url)=>{try{const parsed=new URL(url);return parsed.origin===origin&&!parsed.search&&!parsed.hash?parsed.pathname:null;}catch{return null;}};
const headers=value=>plain(value)&&Object.keys(value).length>0&&Object.keys(value).length<=16&&Object.entries(value).every(([key,item])=>key===key.toLowerCase()&&text(key)&&typeof item==='string'&&item.length<=8192);
const event=value=>Number.isSafeInteger(value)&&value>=0;
const rect=value=>plain(value)&&Number.isFinite(value.x)&&Number.isFinite(value.y)&&Number.isFinite(value.width)&&Number.isFinite(value.height)&&value.width>0&&value.height>0;
const forbiddenRequest=new Set(['range','cookie','authorization','proxy-authorization']);
const permittedResponse=new Set(['etag','content-type','content-length','cache-control','content-security-policy','x-content-type-options','connection']);

/**
 * Expected is caller-owned synthetic data only:
 * {schema:1,origin:'http://127.0.0.1:PORT',contextId:string,browserPid:positive int,
 * manifestSHA:sha256,font:{url,sha256,bytes,mime:'font/woff2',cacheControl:'private,max-age=3600,immutable'},
 * allowedPaths:[seven exact paths],missingBodyError:{name,command,message}}.
 * Pair contains two named browser observations. This function never contacts a browser,
 * filesystem, network, or collector, and a match remains verification=not-verified.
 */
export function checkCacheLineage(expected,pair) {
  const failures=[],fail=(code,detail)=>failures.push({code,detail}); let coldRequestId=null,warmRequestId=null;
  const originOK=value=>typeof value==='string'&&/^http:\/\/127\.0\.0\.1:(?:[1-9][0-9]{0,4})$/.test(value)&&Number(value.split(':').at(-1))<=65535;
  const expectedOK=plain(expected)&&expected.schema===1&&originOK(expected.origin)&&text(expected.contextId)&&Number.isSafeInteger(expected.browserPid)&&expected.browserPid>0&&sha(expected.manifestSHA)&&plain(expected.font)&&text(expected.font.url)&&sha(expected.font.sha256)&&Number.isSafeInteger(expected.font.bytes)&&expected.font.bytes>0&&expected.font.mime==='font/woff2'&&expected.font.cacheControl===immutable&&Array.isArray(expected.allowedPaths)&&expected.allowedPaths.length===7&&new Set(expected.allowedPaths).size===7&&expected.allowedPaths.every(path=>typeof path==='string'&&/^\/[^?#]*$/.test(path))&&sameMap(expected.missingBodyError,missingBodyError);
  if(!expectedOK){fail('expected.invalid','expected is not the required fail-closed cache-lineage schema');return result();}
  const fontPath=pathOf(expected.origin,expected.font.url);
  if(!fontPath||!expected.allowedPaths.includes(fontPath)) fail('expected.font','font URL must be an exact allowed same-origin path');
  if(!plain(pair)||pair.origin!==expected.origin||pair.contextId!==expected.contextId||pair.browserPid!==expected.browserPid) fail('pair.identity','origin, context and browser PID must match expected');
  if(pair?.manifestBefore!==expected.manifestSHA||pair?.manifestAfter!==expected.manifestSHA) fail('pair.manifest','manifest must match before and after collection');
  const setup=pair?.setup;
  if(!plain(setup)||setup.freshContext!==true||setup.cacheClearedBeforeCold!==true||setup.serviceWorkersBlocked!==true||setup.interceptionEnabled!==false||setup.applicationScripts!==0||setup.priorRequests!==0) fail('pair.setup','isolation setup is incomplete');
  if(pair?.navigationCount!==2) fail('pair.navigationCount','exactly cold then warm navigation is required');
  const ids=new Set(),events=new Set(), facts={cold:checkPhase('cold',pair?.cold),warm:checkPhase('warm',pair?.warm)};
  if(facts.cold?.frameId&&facts.warm?.frameId&&facts.cold.frameId!==facts.warm.frameId) fail('pair.frame','cold and warm must use one frame');
  if(facts.cold?.loaderId&&facts.warm?.loaderId&&facts.cold.loaderId===facts.warm.loaderId) fail('pair.loader','cold and warm must use distinct loaders');
  if(facts.cold?.lastFinish>=facts.warm?.firstRequest) fail('pair.order','all cold finishes must precede every warm request');
  if(facts.cold?.render&&facts.warm?.render){
    if(facts.cold.render.pngSHA!==facts.warm.render.pngSHA) fail('render.png','cold and warm PNG identities differ');
    if(!same(facts.cold.render.geometry,facts.warm.render.geometry)) fail('render.geometry','cold and warm geometry differs');
    if(!same(facts.cold.render.paintedFonts,facts.warm.render.paintedFonts)) fail('render.fonts','cold and warm painted-font facts differ');
  }
  return result();

  function result(){return {identityMethod:'isolated-cache-lineage',directWarmBodyObserved:false,verification:'not-verified',matched:failures.length===0,failures,coldRequestId,warmRequestId};}
  function checkPhase(name,phase){
    if(!plain(phase)||phase.navigationType!=='navigate'||!text(phase.frameId)||!text(phase.loaderId)||phase.documentURL!==expected.origin+'/enhanced'||!Array.isArray(phase.requests)||phase.requests.length!==7||!Array.isArray(phase.serverRequests)){fail(`phase.${name}.shape`,'phase requires fixed /enhanced navigation, frame, loader, seven requests and server records');return null;}
    const paths=new Set(),requestIDs=new Set(),byPath=new Map();let documentRequest=null,fontRequest=null,lastFinish=-1,firstRequest=Number.MAX_SAFE_INTEGER;
    for(const request of phase.requests){
      if(!plain(request)||!text(request.id)||requestIDs.has(request.id)||ids.has(request.id)){fail(`phase.${name}.requestId`,'request IDs must be unique across both phases');continue;}
      requestIDs.add(request.id);ids.add(request.id);const path=pathOf(expected.origin,request.url);
      const ordered=event(request.requestSequence)&&event(request.responseSequence)&&event(request.finishSequence)&&request.requestSequence<request.responseSequence&&request.responseSequence<request.finishSequence;
      const cacheWellFormed=request.cacheSequence===null||(event(request.cacheSequence)&&request.requestSequence<request.cacheSequence&&request.cacheSequence<request.finishSequence);
      if(!path||paths.has(path)||!expected.allowedPaths.includes(path)||request.method!=='GET'||request.frameId!==phase.frameId||request.loaderId!==phase.loaderId||request.status!==200||request.redirect!==false||typeof request.fromDiskCache!=='boolean'||request.fromPrefetchCache!==false||request.fromServiceWorker!==false||!headers(request.requestHeaders)||!headers(request.responseHeaders)||!ordered||!cacheWellFormed) fail(`phase.${name}.request`,'request is malformed, redirected, cross-lineage, or event-order-invalid');
      if(headers(request.requestHeaders)&&(Object.keys(request.requestHeaders).some(key=>forbiddenRequest.has(key)||key.startsWith('if-')))) fail(`phase.${name}.requestHeaders`,'range, credential, proxy and validator request headers are refused');
      for(const item of [request.requestSequence,request.responseSequence,request.finishSequence,request.cacheSequence])if(item!==null&&event(item)){if(events.has(item))fail(`phase.${name}.event`,`event sequence ${item} is not globally unique`);events.add(item);}
      if(name==='cold'&&(request.fromDiskCache!==false||request.cacheSequence!==null)) fail('cold.cache','every cold resource must be un-cached and event-free');
      const isDocument=path==='/enhanced'; checkBody(name,path,request);checkResponse(name,path,request,isDocument);
      if(path){paths.add(path);byPath.set(path,request);} firstRequest=Math.min(firstRequest,event(request.requestSequence)?request.requestSequence:Number.MAX_SAFE_INTEGER);lastFinish=Math.max(lastFinish,event(request.finishSequence)?request.finishSequence:-1);
      if(request.type==='Document'){if(documentRequest)fail(`phase.${name}.document`,'exactly one document is required');documentRequest=request;}
      if(path===fontPath){if(fontRequest)fail(`phase.${name}.font`,'exactly one font request is required');fontRequest=request;}
    }
    if(!same([...paths].sort(),[...expected.allowedPaths].sort())) fail(`phase.${name}.paths`,'request paths must equal the exact seven-path manifest');
    if(!documentRequest||documentRequest.url!==phase.documentURL||documentRequest.fromDiskCache!==false) fail(`phase.${name}.document`,'document must be the uncached fixed navigation');
    if(!fontRequest||fontRequest.type!=='Font') fail(`phase.${name}.font`,'font request is missing or not Font'); else checkFont(name,fontRequest);
    checkServer(name,phase.serverRequests,byPath);
    return {frameId:phase.frameId,loaderId:phase.loaderId,lastFinish,firstRequest,render:checkRender(name,phase.render)};
  }
  function checkBody(name,path,request){
    const body=request.body;
    if(!plain(body)||typeof body.available!=='boolean'){fail(`phase.${name}.body`,'each request needs an explicit body observation');return;}
    if(path!==fontPath&&(!body.available||!Number.isSafeInteger(body.bytes)||body.bytes<0||!sha(body.sha256)||Object.hasOwn(body,'error'))) fail(`phase.${name}.body`,'non-font resources require exact available body facts');
  }
  function checkResponse(name,path,request,isDocument){
    const h=request.responseHeaders,body=request.body,bytes=path===fontPath?expected.font.bytes:body?.bytes,digest=path===fontPath?expected.font.sha256:body?.sha256;
    if(!headers(h)||Object.keys(h).some(key=>!permittedResponse.has(key))||h['x-content-type-options']!=='nosniff'||h.connection!=='close'||!text(h['content-security-policy'])||h.etag!==`"sha256-${digest}"`||h['content-length']!==String(bytes)||!text(h['content-type'])||h['cache-control']!==(isDocument?noStore:immutable)) fail(`phase.${name}.responseHeaders`,'response headers must be the exact permitted identity representation policy');
    if(path===fontPath&&(!headers(h)||h['content-type']!==expected.font.mime)) fail(`phase.${name}.fontHeaders`,'font MIME differs from its pin');
  }
  function checkFont(name,request){
    if(name==='cold'){
      if(!plain(request.body)||request.body.available!==true||request.body.bytes!==expected.font.bytes||request.body.sha256!==expected.font.sha256||Object.hasOwn(request.body,'error')) fail('cold.fontBody','cold font must expose the exact pinned body');
      coldRequestId=request.id;
    }else{
      if(!event(request.cacheSequence)||request.requestSequence>=request.cacheSequence||request.cacheSequence>=request.finishSequence||!plain(request.body)||request.body.available!==false||Object.hasOwn(request.body,'bytes')||Object.hasOwn(request.body,'sha256')||!same(request.body.error,missingBodyError)) fail('warm.fontBody','warm font must have its own cache event and only the exact unavailable-body error');
      warmRequestId=request.id;const cold=pair.cold?.requests?.find(item=>item?.id===coldRequestId);
      if(cold&&(!sameMap(cold.requestHeaders,request.requestHeaders)||!sameMap(cold.responseHeaders,request.responseHeaders))) fail('warm.fontHeaders','cold/warm font header maps must match independent of insertion order');
    }
  }
  function checkServer(name,records,byPath){
    const count=name==='cold'?7:1;if(records.length!==count){fail(`server.${name}.count`,`expected ${count} server requests`);return;}const paths=new Set();
    for(const record of records){const path=pathOf(expected.origin,record?.url),request=byPath.get(path);
      if(!plain(record)||!path||paths.has(path)||record.method!=='GET'||record.status!==200||!Number.isSafeInteger(record.bytes)||record.bytes<0||!sha(record.sha256)||!request||record.bytes!==request.body?.bytes||record.sha256!==request.body?.sha256) fail(`server.${name}.record`,'server record is invalid, duplicate, or differs from its observed request body');
      if(path)paths.add(path);if(path===fontPath&&(record.bytes!==expected.font.bytes||record.sha256!==expected.font.sha256))fail(`server.${name}.font`,'server font identity differs from the pin');
    }
    const wanted=name==='cold'?expected.allowedPaths:['/enhanced'];if(!same([...paths].sort(),[...wanted].sort()))fail(`server.${name}.paths`,'server records must be cold-all or warm-document-only');
  }
  function checkRender(name,render){
    const painted=render?.paintedFonts,fontsBounded=Array.isArray(painted)&&painted.length>0&&painted.length<=512&&painted.every(node=>plain(node)&&Array.isArray(node.fonts)&&node.fonts.length>0&&node.fonts.length<=64);
    if(!plain(render)||!sha(render.pngSHA)||!Array.isArray(render.geometry)||render.geometry.length===0||render.geometry.length>512||!render.geometry.every(rect)||render.fontReady!==true||!fontsBounded||inspectPaintedFonts(painted).length){fail(`render.${name}`,'PNG, 1..512 finite positive geometry records, bounded font facts, readiness and custom painted fonts are required');return null;}return render;
  }
}
