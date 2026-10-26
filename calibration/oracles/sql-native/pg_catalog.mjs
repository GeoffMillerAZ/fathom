// Bench-only PostgreSQL execution; stdin holds untouched source-statement slices.
import {readFileSync} from 'node:fs';
import path from 'node:path';
import {pathToFileURL} from 'node:url';

export async function runScenarios(scenarios, tools) {
  const moduleAt = p => import(pathToFileURL(path.join(tools,'node_modules',p)).href);
  const {PGlite} = await moduleAt('@electric-sql/pglite/dist/index.js');
  const {btree_gist} = await moduleAt('@electric-sql/pglite/dist/contrib/btree_gist.js');
  const {vector} = await moduleAt('@electric-sql/pglite-pgvector/dist/index.js');
  const versionOf = name => JSON.parse(readFileSync(path.join(tools,'node_modules',name,'package.json'),'utf8')).version;
  const pglite = versionOf('@electric-sql/pglite');
  const pgvector = versionOf('@electric-sql/pglite-pgvector');
  if (pglite !== '0.5.11' || pgvector !== '0.0.12') throw Error('unpinned SQL runtime');
  const catalog = `SELECT c.oid::text AS oid, s.relname AS source, t.relname AS target,
                  c.conislocal AS is_local
                  FROM pg_catalog.pg_constraint c
                  JOIN pg_catalog.pg_class s ON s.oid=c.conrelid
                  JOIN pg_catalog.pg_class t ON t.oid=c.confrelid
                  WHERE c.contype='f' ORDER BY c.oid`;
  const files=[];let postgresql;
  for (const scenario of scenarios) {
    const db=new PGlite({extensions:{btree_gist,vector}});
    try {
      postgresql=(await db.query('SHOW server_version')).rows[0].server_version;
      for (const file of scenario) {
        // Each input file models a fresh psql session over the same database.
        // RESET ALL changes session configuration, never the source SQL.
        await db.exec('RESET ALL');
        const pairs=new Map(),errors=[],receipts=[];
        let create_table_failed=0, excluded_catalog_keys=0;
        for (const statement of file.statements) {
          const {sql,...event}=statement;
          let before;
          try {before=new Set((await db.query(catalog)).rows.map(r=>r.oid));}
          catch {before=new Set();} // an aborted transaction is still named by exec's error
          try {await db.exec(sql);}
          catch(e) {
            if(statement.kind==='create_table')create_table_failed++;
            errors.push({path:file.path,line:statement.line,kind:statement.kind,
                         code:e.code??'runtime',message:e.message});
            receipts.push({...event,error:e.code??'runtime'});
            continue;
          }
          receipts.push({...event,accepted:true});
          const after=(await db.query(catalog)).rows;
          for(const key of after) {
            if(before.has(key.oid))continue;
            if(statement.kind!=='create_table'||!key.is_local){excluded_catalog_keys++;continue;}
            const pair={path:file.path,from:key.source,to:key.target};
            pairs.set(JSON.stringify(pair),pair);
          }
        }
        files.push({path:file.path,source_sha256:file.source_sha256,source_bytes:file.source_bytes,engine:'postgresql',calls:[...pairs.values()].sort((a,b)=>JSON.stringify(a)<JSON.stringify(b)?-1:JSON.stringify(a)>JSON.stringify(b)?1:0),
                    errors,create_table_failed,excluded_catalog_keys,statements:receipts.length,receipts});
      }
    } finally {await db.close();}
  }
  return {pglite,pgvector,postgresql,files};
}

if(process.argv[1]&&import.meta.url===pathToFileURL(path.resolve(process.argv[1])).href) {
  const requests=JSON.parse(readFileSync(0,'utf8'));
  process.stdout.write(JSON.stringify(await runScenarios(requests,path.resolve(process.argv[2])))+'\n');
}
