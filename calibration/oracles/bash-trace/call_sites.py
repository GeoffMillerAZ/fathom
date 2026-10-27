"""Independent shfmt literal-site scope; never native truth endpoints."""
from syntax_scope import parse


def collect(root,files,shfmt,syntax,max_sites=1_000_000):
    if not 1<=max_sites<=1_000_000:raise ValueError('whole-unit refusal: invalid call-site cap')
    sites=[]
    dynamic=0
    for relative in files:
        tree=parse(shfmt,(root/relative).read_bytes())
        def walk(node,caller):
            nonlocal dynamic
            if isinstance(node,dict):
                if node.get('Type')=='FuncDecl':
                    caller=node['Name']['Value']
                if node.get('Type')=='CallExpr' and node.get('Args'):
                    if len(sites)>=max_sites:raise ValueError('whole-unit refusal: literal call-site cap')
                    word=node['Args'][0]
                    value,kind=syntax.word_literal(word)
                    if kind=='literal' and value:
                        sites.append({'path':relative,'line':word['Pos']['Line'],'column':word['Pos']['Col'],'from':caller,'written_name':value,'literal':True})
                    else:
                        dynamic+=1
                        sites.append({'path':relative,'line':word['Pos']['Line'],'column':word['Pos']['Col'],'from':caller,'written_name':None,'literal':False})
                for value in node.values():walk(value,caller)
            elif isinstance(node,list):
                for value in node:walk(value,caller)
        walk(tree,'<module>')
    return {'sites':sites,'dynamic_sites':dynamic,'literal_sites':sum(s['literal'] for s in sites)}
