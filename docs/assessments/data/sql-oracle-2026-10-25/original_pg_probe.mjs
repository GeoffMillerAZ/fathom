// Replay the superseded whole-file PostgreSQL probe without pgvector. Not the grading oracle.
import path from 'node:path';
import {readFileSync} from 'node:fs';
import {pathToFileURL} from 'node:url';
const [tools,subject]=process.argv.slice(2).map(p=>path.resolve(p));
const {PGlite}=await import(pathToFileURL(path.join(tools,'node_modules/@electric-sql/pglite/dist/index.js')).href);
const {btree_gist}=await import(pathToFileURL(path.join(tools,'node_modules/@electric-sql/pglite/dist/contrib/btree_gist.js')).href);
const results=[];
for(const file of ['pagila-schema.sql','pagila-schema-jsonb.sql','pagila-temporal-setup.sql']){
 const db=new PGlite({extensions:{btree_gist}});
 try {
  const version=(await db.query('select version()')).rows[0].version;
  let error=null;
  try{await db.exec(readFileSync(path.join(subject,file),'utf8'));}
  catch(e){error={message:e.message,code:e.code,position:e.position};}
  results.push({file,version,error});
 } finally {await db.close();}
}
console.log(JSON.stringify({results},null,2));
