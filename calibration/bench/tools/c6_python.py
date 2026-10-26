import os
import ast, pathlib, subprocess, collections, json
JOB=pathlib.Path(os.environ['BENCH_JOB']); ROOT=pathlib.Path.cwd()
def analyze(d):
 root=pathlib.Path(os.environ['BENCH_SUBJECTS'])/d['id']; paths=subprocess.check_output(['git','-C',str(root),'ls-files','-z'],text=True).split('\0')[:-1]
 trees={}; defs=set(); aliases={}; imports={}
 def module(p):
  p=p.removeprefix('src/');return p.removesuffix('.py').removesuffix('/__init__').replace('/','.')
 for p in paths:
  if not p.endswith('.py'):continue
  try:trees[p]=ast.parse((root/p).read_text())
  except (SyntaxError,UnicodeError):continue
  mod=module(p); local={}
  for n in trees[p].body:
   if isinstance(n,(ast.FunctionDef,ast.AsyncFunctionDef,ast.ClassDef)):defs.add(mod+'.'+n.name)
   elif isinstance(n,ast.Import):
    for x in n.names:local[x.asname or x.name.split('.')[0]]=x.name if x.asname else x.name.split('.')[0]
   elif isinstance(n,ast.ImportFrom):
    base=(mod if p.endswith('__init__.py') else mod.rpartition('.')[0]).split('.')
    prefix='.'.join(base[:len(base)-n.level+1]) if n.level else ''
    target='.'.join(x for x in [prefix,n.module] if x)
    for x in n.names:
     if x.name!='*':local[x.asname or x.name]=target+'.'+x.name
  imports[p]=local
  for k,v in local.items(): aliases[mod+'.'+k]=v
  for n in trees[p].body:
   if isinstance(n,ast.Assign):
    v=n.value
    if isinstance(v,ast.Name):
     for target in n.targets:
      if isinstance(target,ast.Name):aliases[mod+'.'+target.id]=local.get(v.id,mod+'.'+v.id)
 def resolve(name):
  seen=set()
  for _ in range(15):
   if name in defs:return name
   if name in seen:return None
   seen.add(name)
   if name in aliases:name=aliases[name];continue
   found=False
   for k in sorted(aliases,key=len,reverse=True):
    if name.startswith(k+'.'):name=aliases[k]+name[len(k):];found=True;break
   if not found:return None
  return None
 calls=collections.defaultdict(list);mods=collections.defaultdict(set)
 for p,tree in trees.items():
  mod=module(p);local=imports[p]
  def visit(n,shadow=frozenset()):
   if isinstance(n,ast.Call):
    f=n.func;parts=[]
    while isinstance(f,ast.Attribute):parts.append(f.attr);f=f.value
    if isinstance(f,ast.Name) and f.id not in shadow:
     name='.'.join([local.get(f.id,mod+'.'+f.id)]+list(reversed(parts)));target=resolve(name)
     if target:calls[target].append(f'{p}:{n.lineno}:{n.col_offset+1}');mods[target].add(mod)
   if isinstance(n,(ast.FunctionDef,ast.AsyncFunctionDef,ast.Lambda)):
    args=n.args; assigned={a.arg for a in args.posonlyargs+args.args+args.kwonlyargs}
    if args.vararg:assigned.add(args.vararg.arg)
    if args.kwarg:assigned.add(args.kwarg.arg)
    # Decorators execute in the outer scope.
    for child in getattr(n,'decorator_list',[]):visit(child,shadow)
    for child in n.body if isinstance(n.body,list) else [n.body]: visit(child,shadow|assigned)
   else:
    for child in ast.iter_child_nodes(n):visit(child,shadow)
  visit(tree)
 rows=[{'symbol':s,'calls':len(set(sites)),'modules':sorted(mods[s]),'sites':sorted(set(sites))} for s,sites in calls.items()]
 rows.sort(key=lambda x:(-len(x['modules']),-x['calls'],x['symbol']))
 (JOB/'receipts'/f"{d['id']}-fanin.json").write_text(json.dumps(rows,indent=2)+'\n')
 print(d['name'],[(x['symbol'],x['calls'],len(x['modules'])) for x in rows[:4]],flush=True)
for f in (JOB/'receipts').glob('*.json'):
 d=json.loads(f.read_text())
 if isinstance(d,dict) and d.get('language')=='Python':analyze(d)
