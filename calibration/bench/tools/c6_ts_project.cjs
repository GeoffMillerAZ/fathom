const ts = require(process.env.BENCH_TYPESCRIPT || 'typescript');
const fs = require('fs'),path=require('path'),cp=require('child_process');
const root=path.resolve(process.argv[2]);
const paths=cp.execFileSync('git',['-C',root,'ls-files','-z'],{encoding:'utf8'}).split('\0').filter(p=>/\.(ts|tsx|mts|cts)$/.test(p));
const configPath=path.resolve(root,process.argv[3] || 'tsconfig.json');
const config=ts.readConfigFile(configPath,ts.sys.readFile);
if(config.error) throw new Error(ts.flattenDiagnosticMessageText(config.error.messageText,'\n'));
const parsed=ts.parseJsonConfigFileContent(config.config,ts.sys,path.dirname(configPath));
const options={...parsed.options,noEmit:true};
const program=ts.createProgram(paths.map(p=>path.join(root,p)),options);
const checker=program.getTypeChecker(),calls=new Map();
for(const sf of program.getSourceFiles()) {
 const rel=path.relative(root,sf.fileName);if(!paths.includes(rel))continue;
 function visit(n) {
  if(ts.isCallExpression(n)||ts.isNewExpression(n)) {
   const sig=checker.getResolvedSignature(n);const decl=sig&&sig.getDeclaration();
   if(decl && !ts.isMethodSignature(decl) && !ts.isCallSignatureDeclaration(decl) && !ts.isConstructSignatureDeclaration(decl)){const targetFile=path.relative(root,decl.getSourceFile().fileName);
    if(paths.includes(targetFile)){
     let name=decl.name?decl.name.getText():undefined;
     if(!name&&ts.isConstructorDeclaration(decl)&&decl.parent.name) name=decl.parent.name.getText()+'.constructor';
     if(!name){let p=decl.parent;if(ts.isVariableDeclaration(p)&&p.name)name=p.name.getText();}
     if(name){let target=targetFile+'#'+name+'@'+(decl.getSourceFile().getLineAndCharacterOfPosition(decl.getStart()).line+1);const loc=sf.getLineAndCharacterOfPosition(n.getStart());
      if(!calls.has(target))calls.set(target,{symbol:target,modules:new Set(),sites:new Set()});
      let row=calls.get(target);row.modules.add(rel);row.sites.add(rel+':'+(loc.line+1)+':'+(loc.character+1));
     }
    }
   }
  }
  ts.forEachChild(n,visit);
 }
 visit(sf);
}
const result=[...calls.values()].map(r=>({symbol:r.symbol,calls:r.sites.size,modules:[...r.modules].sort(),sites:[...r.sites].sort()})).sort((a,b)=>b.modules.length-a.modules.length||b.calls-a.calls||a.symbol.localeCompare(b.symbol));
process.stdout.write(JSON.stringify(result,null,2)+'\n');
