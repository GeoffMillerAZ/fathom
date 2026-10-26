// Contract probe only; this fixture never enters a calibration corpus.
import fs from 'node:fs';
import path from 'node:path';
import {createRequire} from 'node:module';
import {fileURLToPath,pathToFileURL} from 'node:url';
const argv=process.argv.slice(2);
const tools=path.resolve(argv[argv.indexOf('--tools')+1]);
const out=path.resolve(argv[argv.indexOf('--out')+1]);
fs.mkdirSync(out,{recursive:false});
const require=createRequire(path.join(tools,'package.json'));
const here=path.dirname(fileURLToPath(import.meta.url));
const source=fs.readFileSync(path.join(here,'probe.tsx'),'utf8');
const babel=require('@babel/core');
const compiler=require('babel-plugin-react-compiler');
const summary={versions:{babel:require('@babel/core/package.json').version,
  compiler:require('babel-plugin-react-compiler/package.json').version,
  typescript:require('typescript/package.json').version,
  docgenTypescript:require('react-docgen-typescript/package.json').version},modes:{}};
for(const mode of ['infer','annotation','all']){
 const events=[];
 babel.transformSync(source,{filename:'probe.tsx',configFile:false,babelrc:false,
  parserOpts:{plugins:['typescript','jsx']},
  plugins:[[compiler,{compilationMode:mode,logger:{logEvent(_file,event){
    events.push({kind:event.kind,name:event.fnName??null,line:event.fnLoc?.start.line??null});
  }}}]]});
 summary.modes[mode]=events;
}
const rd=await import(pathToFileURL(require.resolve('react-docgen')).href);
summary.versions.docgen='8.0.4';
for(const [label,resolver] of [['exported',new rd.builtinResolvers.FindExportedDefinitionsResolver()],['all',new rd.builtinResolvers.FindAllDefinitionsResolver()]]){
 try{summary['docgen_'+label]=rd.parse(source,{filename:'probe.tsx',resolver}).map(x=>({name:x.displayName,props:Object.keys(x.props??{})}));}
 catch(error){summary['docgen_'+label+'_failure']=error.message;}
}
const ts=require('typescript');
summary.typescriptLegacyAPI={createProgram:typeof ts.createProgram,createCompilerHost:typeof ts.createCompilerHost};
const fixture=path.join(out,'probe.tsx');fs.writeFileSync(fixture,source);
try{summary.docgenTypescript=require('react-docgen-typescript').parse(fixture).map(x=>x.displayName);}
catch(error){summary.docgenTypescriptFailure=error.message.replaceAll(out,'<probe>');}
fs.writeFileSync(path.join(out,'summary.json'),JSON.stringify(summary,null,2)+'\n');
process.stdout.write(JSON.stringify(summary)+'\n');
