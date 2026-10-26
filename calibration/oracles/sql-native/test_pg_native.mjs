import test from 'node:test';
import assert from 'node:assert/strict';
import {runScenarios} from './pg_catalog.mjs';
import path from 'node:path';
const tools = path.resolve(process.env.SQL_NATIVE_TOOLS ?? 'workspace/tools/sql-runtime');
const stmt=(sql,kind,line)=>({sql,kind,path:'probe.sql',line,sha256:'test-receipt'});
test('native PG18 temporal DDL survives unrelated rejected FOR PORTION OF',async()=>{
 const r=await runScenarios([[{path:'probe.sql',statements:[
  stmt('CREATE EXTENSION btree_gist','create',1),
  stmt('CREATE TABLE parent(id integer primary key)','create_table',2),
  stmt('CREATE TABLE periods(id integer REFERENCES parent(id), span daterange, PRIMARY KEY(id,span WITHOUT OVERLAPS))','create_table',3),
  stmt('CREATE TABLE overrides(id integer, span daterange, FOREIGN KEY(id, PERIOD span) REFERENCES periods(id, PERIOD span))','create_table',4),
  stmt("UPDATE periods FOR PORTION OF span FROM '2024-01-01' TO '2024-02-01' SET id=2",'update',5),
 ]}]],tools);
 assert.equal(r.files[0].create_table_failed,0);
 assert.deepEqual(r.files[0].calls,[{path:'probe.sql',from:'overrides',to:'periods'},{path:'probe.sql',from:'periods',to:'parent'}]);
 assert.equal(r.files[0].errors.length,1);assert.equal(r.files[0].errors[0].code,'42601');
});
test('ALTER TABLE and inherited partition FKs are outside inline syntax truth',async()=>{
 const r=await runScenarios([[{path:'probe.sql',statements:[
  stmt('CREATE TABLE p(id integer primary key)','create_table',1),
  stmt('CREATE TABLE c(id integer REFERENCES p) PARTITION BY RANGE(id)','create_table',2),
  stmt('CREATE TABLE c1 PARTITION OF c FOR VALUES FROM(0) TO(10)','create_table',3),
  stmt('CREATE TABLE other(id integer)','create_table',4),
  stmt('ALTER TABLE other ADD FOREIGN KEY(id) REFERENCES p','alter',5),
 ]}]],tools);
 assert.deepEqual(r.files[0].calls,[{path:'probe.sql',from:'c',to:'p'}]);
 assert.ok(r.files[0].excluded_catalog_keys>=2);
});
test('a rejected CREATE TABLE prevents complete-coverage claims',async()=>{
 const r=await runScenarios([[{path:'bad.sql',statements:[stmt('CREATE TABLE c(id integer REFERENCES missing)','create_table',1)]}]],tools);
 assert.equal(r.files[0].create_table_failed,1);assert.equal(r.files[0].errors[0].code,'42P01');
});
test('each source file starts a fresh-session configuration after a pg_dump schema',async()=>{
 const r=await runScenarios([[
  {path:'schema.sql',statements:[stmt("SELECT set_config('search_path','',false)",'select',1),stmt('CREATE TABLE public.p(id integer primary key)','create_table',2)]},
  {path:'next.sql',statements:[stmt('CREATE TABLE child(id integer REFERENCES p)','create_table',1)]},
 ]],tools);
 assert.equal(r.files[1].create_table_failed,0);
 assert.deepEqual(r.files[1].calls,[{path:'next.sql',from:'child',to:'p'}]);
});
