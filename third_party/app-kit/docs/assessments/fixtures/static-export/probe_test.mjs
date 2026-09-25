import test from 'node:test';
import assert from 'node:assert/strict';
import {
  assessAllowedRequest, assessCleanup, assessLinks, assessObservation, assessPresentationFacts,
  isConnectionRefused, resolveStaticRequest, selectedFile, validateContract,
  validateManifest,
} from './probe.mjs';

const files = ['index.html','index-light.html','index-dark.html','profile.html','profile-light.html','profile-dark.html','404.html','404-light.html','404-dark.html','app.css','manifest.json'];
const contract = {
  schema_version:'static-export-local-v1', manifest_schema_version:1, base_path:'/reading/', javascript:false, cache:'disabled', network:'unthrottled-owned-loopback',
  budgets:{decoded_bytes:131072,response_end_ms:100,dom_content_loaded_ms:250,load_end_ms:500}, files,
  lanes:[
    {width:390,height:1000,theme:'system',color_scheme:'dark'}, {width:390,height:1000,theme:'light',color_scheme:'light'}, {width:390,height:1000,theme:'dark',color_scheme:'dark'},
    {width:1440,height:1000,theme:'system',color_scheme:'dark'}, {width:1440,height:1000,theme:'light',color_scheme:'light'}, {width:1440,height:1000,theme:'dark',color_scheme:'dark'},
  ],
  paths:{index:'index.html',profile:'profile.html',not_found:'404.html',deep_missing:'/reading/missing/deep/page.html'},
  oracles:{index_heading:'Choose a profile before you integrate a capability',profile_heading:'Choosing a profile',problem_heading:'Content destination unavailable',faq_question:'Which profile should I choose?',faq_destination_label:'Read the related guide',return_label:'Return to the integration guide',resource_labels:['Match the runtime to the task','Review delivery evidence'],resource_fragments:['content-runtime-fit','content-delivery-evidence'],toc_fragments:['content-guide','start-with-task','keep-policy-local','content-faq']},
};
const site = Object.fromEntries(files.map((name,index)=>[name,{kind:'file',bytes:index+10,sha256:String(index).padStart(64,'a')}]))
const manifest = {schema_version:1,base_path:'/reading/',files:files.filter(name=>name!=='manifest.json').map(name=>({path:name,bytes:site[name].bytes,sha256:site[name].sha256})),manifest_excludes:['manifest.json']};
const origin='http://127.0.0.1:1234';
const good = {
  origin,url:`${origin}/reading/index.html`,status:200,expectedFile:'index.html',expectedStatus:200,html:{file:'index.html',retained:'390-system-index.html',bytes:10,sha256:site['index.html'].sha256},
  taskPassed:true,taskFacts:{noOverflow:true,controls44:true,paintedFocus:true,themeCorrect:true},scriptNodes:0,scriptAttempts:0,deniedRequests:[],
  resources:[
    {url:`${origin}/reading/index.html`,method:'GET',kind:'document',status:200,mime:'text/html; charset=utf-8',fromCache:false,terminal:true,bodyAvailable:true,decodedBytes:10,sha256:site['index.html'].sha256},
    {url:`${origin}/reading/app.css`,method:'GET',kind:'stylesheet',status:200,mime:'text/css; charset=utf-8',fromCache:false,terminal:true,bodyAvailable:true,decodedBytes:19,sha256:site['app.css'].sha256},
  ],timings:{responseEnd:10,domContentLoaded:20,loadEnd:30},
};

test('contract accepts only the pinned complete shape',()=>{
  assert.deepEqual(validateContract(structuredClone(contract)),contract);
  for(const mutation of [
    value=>value.budgets.load_end_ms=NaN,
    value=>value.files.pop(),
    value=>value.lanes[5]=value.lanes[0],
    value=>value.oracles.index_heading='copied from page',
  ]){const value=structuredClone(contract);mutation(value);assert.throws(()=>validateContract(value));}
});

test('selection and deep 404 mapping are exact',()=>{
  assert.equal(selectedFile('index','system'),'index.html');
  assert.equal(selectedFile('profile','dark'),'profile-dark.html');
  assert.deepEqual(resolveStaticRequest(contract,'/reading/missing/deep/page.html'),{failures:[],file:'404.html',status:404,mime:'text/html; charset=utf-8'});
  assert.ok(resolveStaticRequest(contract,'/reading/missing/deep/page.html?theme=light').failures.length);
  assert.ok(resolveStaticRequest(contract,'/reading/missing/deep/page.html?state=x').failures.length);
});

test('request policy refuses remote methods unknown paths and CSS query',()=>{
  assert.deepEqual(assessAllowedRequest(origin,`${origin}/reading/index.html`,'GET',contract),[]);
  assert.ok(assessAllowedRequest(origin,'https://example.test/reading/index.html','GET',contract).includes('request.origin'));
  assert.ok(assessAllowedRequest(origin,`${origin}/reading/index.html`,'POST',contract).includes('request.method'));
  assert.ok(assessAllowedRequest(origin,`${origin}/reading/no.html`,'GET',contract).includes('request.path'));
  assert.ok(assessAllowedRequest(origin,`${origin}/reading/app.css?v=1`,'GET',contract).includes('request.css-query'));
});

test('manifest binds exact closure hashes and byte counts',()=>{
  assert.deepEqual(validateManifest(contract,structuredClone(manifest),structuredClone(site)),[]);
  for(const mutation of [
    (m,s)=>m.files[0].sha256='b'.repeat(64),
    (m,s)=>m.files[0].bytes++,
    (m,s)=>m.manifest_excludes=[],
    (m,s)=>s.extra={kind:'file',bytes:1,sha256:'c'.repeat(64)},
    (m,s)=>s['app.css'].kind='symlink',
  ]){const m=structuredClone(manifest),s=structuredClone(site);mutation(m,s);assert.ok(validateManifest(contract,m,s).length);}
});

test('normal links stay relative and deep 404 recovery is absolute',()=>{
  assert.deepEqual(assessLinks(contract,'index-dark.html',['#static-reference-main','profile-dark.html#content-runtime-fit']),[]);
  assert.deepEqual(assessLinks(contract,'profile-dark.html',['index-dark.html#content-guide']),[]);
  assert.deepEqual(assessLinks(contract,'404-dark.html',['#static-reference-main','/reading/index-dark.html']),[]);
  assert.ok(assessLinks(contract,'index.html',['/reading/profile.html#content-runtime-fit']).includes('link.normal-path'));
  assert.ok(assessLinks(contract,'404.html',['index.html']).includes('link.not-found-path'));
  assert.ok(assessLinks(contract,'index.html',['profile.html#unknown']).includes('link.fragment'));
});

test('FAQ destination admits only the fixed profile title fragment',()=>{
  const completeIndexLinks=['profile.html#content-runtime-fit','profile.html#content-profile-title'];
  assert.deepEqual(assessLinks(contract,'index.html',completeIndexLinks),[]);
  assert.ok(assessLinks(contract,'index.html',['profile.html#content-runtime-fit','profile.html#arbitrary-title']).includes('link.fragment'));
  assert.ok(assessLinks(contract,'index.html',['profile.html#content-runtime-fit','index.html#content-profile-title']).includes('link.fragment'));
});

test('appearance and ToC presentation facts fail closed',()=>{
  const facts={appearanceSelected:true,tocUnderlined:true,tocLinkCount:4};
  assert.deepEqual(assessPresentationFacts(facts,{requireToc:true}),[]);
  assert.ok(assessPresentationFacts({...facts,appearanceSelected:false},{requireToc:true}).includes('presentation.appearance'));
  assert.ok(assessPresentationFacts({...facts,tocUnderlined:false},{requireToc:true}).includes('presentation.toc'));
  assert.ok(assessPresentationFacts({...facts,tocLinkCount:3},{requireToc:true}).includes('presentation.toc'));
});

test('link grammar rejects schemes escapes queries traversal and nested files',()=>{
  for(const href of ['#','profile.html#','https://example.test/','//example.test/x','mailto:x@example.test','javascript:alert(1)','data:text/plain,x','profile%2ehtml','profile.html?x=1','../profile.html','nested/profile.html','profile\\.html']) {
    assert.ok(assessLinks(contract,'index.html',[href]).length,href);
  }
  assert.ok(assessLinks(contract,'404.html',['/reading/nested/index.html']).includes('link.not-found-path'));
});

test('complete source-independent observation passes',()=>assert.deepEqual(assessObservation(contract,structuredClone(good),site),[]));

test('status document CSS cache and exact resource closure reject mismatches',()=>{
  const cases=[
    [value=>value.status=404,'document.status'],
    [value=>value.resources[0].sha256='f'.repeat(64),'resource.identity'],
    [value=>value.resources[1].status=404,'resource.identity'],
    [value=>value.resources[1].mime='text/plain','resource.identity'],
    [value=>value.resources[0].fromCache=true,'resource.cache'],
    [value=>value.resources.pop(),'resources.exact'],
    [value=>value.resources.push(structuredClone(value.resources[1])),'resources.exact'],
    [value=>value.html.sha256='e'.repeat(64),'document.artifact'],
  ];
  for(const [mutation,want] of cases){const value=structuredClone(good);mutation(value);assert.ok(assessObservation(contract,value,site).includes(want),want);}
});

test('network body scripts task facts and budgets fail closed',()=>{
  const cases=[
    value=>value.deniedRequests=['remote'], value=>value.resources[0].bodyAvailable=false,
    value=>value.scriptAttempts=1, value=>value.taskFacts.themeCorrect=undefined,
    value=>value.resources[0].decodedBytes=131072, value=>value.timings.loadEnd=NaN,
    value=>{value.timings.responseEnd=30;value.timings.domContentLoaded=20;},
  ];
  for(const mutation of cases){const value=structuredClone(good);mutation(value);assert.ok(assessObservation(contract,value,site).length);}
});

test('fixed task oracle cannot be replaced with page-derived values',()=>{
  const value=structuredClone(good); value.taskPassed=false; value.taskFacts.heading='whatever the page rendered';
  assert.ok(assessObservation(contract,value,site).includes('task'));
});

test('cleanup requires browser exit and exact listener refusal',()=>{
  const goodCleanup={browser:{exitCode:0,signalCode:null},serverClosed:true,portRefused:true,refusalCode:'ECONNREFUSED'};
  assert.deepEqual(assessCleanup(goodCleanup),[]);
  assert.ok(assessCleanup({...goodCleanup,browser:{exitCode:null,signalCode:null}}).includes('cleanup.browser'));
  assert.ok(assessCleanup({...goodCleanup,refusalCode:'ETIMEDOUT'}).includes('cleanup.server'));
  assert.equal(isConnectionRefused({code:'ECONNREFUSED'}),true);
  assert.equal(isConnectionRefused({code:'EACCES'}),false);
});
