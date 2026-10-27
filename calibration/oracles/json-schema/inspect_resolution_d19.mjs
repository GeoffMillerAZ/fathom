// Diagnostic counts only: contrast physical relative files with native reads.
// This never amends the canonical truth, rewrites $id, or fetches a URL.
import fs from 'node:fs';
import path from 'node:path';
import {createRequire} from 'node:module';
const flags=Object.fromEntries(process.argv.slice(2).reduce((rows,arg,index,args)=>index%2===0?[...rows,[arg,args[index+1]]]:rows,[]));
const subject=path.resolve(flags['-subject']);
const req=createRequire(path.join(path.resolve(flags['-tools']),'package.json'));
const {$RefParser}=req('@apidevtools/json-schema-ref-parser');
const counts={files:0,schemas:0,parse_refusals:0,resolve_refusals:0,native_read_events:0,native_url_read_events:0,native_subject_file_read_events:0,relative_file_reference_sites:0,physical_subject_file_sites:0,physical_sites_attested_as_file_reads:0};
const skip=new Set(['.git','vendor','node_modules','generated','testdata','.idea','.vscode','.claude']);
function* files(dir){for(const entry of fs.readdirSync(dir,{withFileTypes:true}).sort((a,b)=>a.name.localeCompare(b.name))){const full=path.join(dir,entry.name);if(entry.isDirectory()&&!skip.has(entry.name))yield*files(full);else if(entry.isFile()&&entry.name.endsWith('.json'))yield full;}}
function* refs(value){if(Array.isArray(value)){for(const item of value)yield*refs(item);}else if(value&&typeof value==='object'){for(const [key,item]of Object.entries(value)){if(key==='$ref'&&typeof item==='string')yield item;yield*refs(item);}}}
const url=/^[A-Za-z][A-Za-z0-9+.-]*:\/\//;
const resolve={external:true,http:{order:1,canRead:/^https?:/i,read:()=>"{}"},file:{order:2,canRead:true,read:file=>{try{return fs.readFileSync(file.url.replace(/^file:\/\//,''));}catch{return '{}';}}}};
for(const full of files(subject)){
 counts.files++;
 let api;try{api=await $RefParser.parse(full,{resolve:{external:false,http:false}});}catch{counts.parse_refusals++;continue;}
 if(!api||typeof api!=='object'||Array.isArray(api)||!(full.endsWith('.schema.json')||typeof api.$schema==='string'&&/^https?:\/\/json-schema\.org\//.test(api.$schema)))continue;
 counts.schemas++;
 let resolved;try{resolved=await $RefParser.resolve(full,{resolve});}catch{counts.resolve_refusals++;continue;}
 const reads=new Set(resolved.paths());
 for(const target of reads){counts.native_read_events++;if(url.test(target))counts.native_url_read_events++;else if(path.relative(subject,target)!=='..'&&!path.relative(subject,target).startsWith('..'+path.sep)&&fs.existsSync(target))counts.native_subject_file_read_events++;}
 for(const ref of refs(api)){
  const part=ref.split('#')[0];if(!part||url.test(part))continue;
  counts.relative_file_reference_sites++;
  const target=part.startsWith('/')?path.join(subject,part):path.resolve(path.dirname(full),part);
  if(path.relative(subject,target)==='..'||path.relative(subject,target).startsWith('..'+path.sep)||!fs.existsSync(target))continue;
  counts.physical_subject_file_sites++;
  if(reads.has(target))counts.physical_sites_attested_as_file_reads++;
 }
}
fs.writeFileSync(flags['-out'],JSON.stringify({tool:'@apidevtools/json-schema-ref-parser',version:req('@apidevtools/json-schema-ref-parser/package.json').version,counts},null,2)+'\n');
