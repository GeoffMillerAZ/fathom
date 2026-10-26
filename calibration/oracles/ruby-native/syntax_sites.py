#!/usr/bin/env python3
"""Full upstream C-query site denominator; never native truth endpoints."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tree_sitter


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('subject', type=Path)
    ap.add_argument('--fixture', action='store_true')
    args = ap.parse_args()
    root = args.subject.resolve()
    repo = Path(__file__).resolve().parents[3]
    spec = importlib.util.spec_from_file_location('upstream_tags', repo/'calibration/oracles/ts-tags/tags_oracle.py')
    upstream = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(upstream)
    language = upstream.load_language('ruby')
    parser = tree_sitter.Parser(language)
    query = tree_sitter.Query(language, (repo/'calibration/oracles/ts-tags/ruby-tags.scm').read_text())
    lq = tree_sitter.Query(language, (repo/'calibration/oracles/ts-tags/ruby-locals.scm').read_text())
    cursor, lc = tree_sitter.QueryCursor(query), tree_sitter.QueryCursor(lq)
    nonlocal_patterns = {i for i in range(query.pattern_count) if 'local' in query.pattern_assertions(i)
                         and not query.pattern_assertions(i)['local'][1]}
    sites, files, blocked, total_bytes = [], 0, [], 0
    limits = {name:int(os.environ.get("RUBY_SITE_"+name+"_CAP",str(default)))
              for name,default in [("FILE",2048),("BYTE",64<<20),("SITE",1_000_000)]}
    if any(not 1 <= limits[name] <= maximum for name,maximum in [("FILE",2048),("BYTE",64<<20),("SITE",1_000_000)]):
        raise ValueError("whole-unit refusal: invalid syntax caps")
    tracked = [b'native.rb'] if args.fixture else subprocess.check_output(['git', '-C', str(root), 'ls-files', '-z']).split(b'\0')
    for raw in tracked:
        if not raw:
            continue
        rel = raw.decode()
        path = root/rel
        if path.is_symlink() or not path.is_file() or any(p in {'vendor','node_modules','.git','testdata','generated'} for p in path.relative_to(root).parts):
            continue
        if not upstream.candidate(str(path), rel, ['.rb','.rake','.ru'], ['ruby','jruby'], ['Rakefile']):
            continue
        if files+1 > limits["FILE"]:raise ValueError("whole-unit refusal: syntax file cap")
        total_bytes += path.stat().st_size
        if total_bytes > limits["BYTE"]:raise ValueError("whole-unit refusal: syntax byte cap")
        source = path.read_bytes()
        tree = parser.parse(source)
        if tree.root_node.has_error:
            raise ValueError('refuse incomplete syntax population')
        files += 1
        local_scopes, local_defs = upstream.read_locals(lq, lc, tree.root_node, len(source), source)
        matches = list(cursor.matches(tree.root_node))
        claimed = {(n.start_byte,n.end_byte) for _, caps in matches for n in caps.get('ignore',[])}
        funcs, calls = [], []
        for pattern, caps in matches:
            if 'name' in caps:
                node = caps['name'][0]
                key = (node.start_byte,node.end_byte)
                if key in claimed or pattern in nonlocal_patterns and upstream.is_local(node, source, local_scopes, local_defs):
                    continue
                claimed.add(key)
            for kind in ['definition.function','definition.method']:
                if kind in caps:
                    funcs.append((caps[kind][0], caps['name'][0].text.decode()))
                    break
            else:
                if 'reference.call' in caps:
                    calls.append(caps['name'][0])
        for call in calls:
            enclosing = [(n,name) for n,name in funcs if n.start_byte <= call.start_byte and call.end_byte <= n.end_byte]
            if not enclosing:
                continue
            node, caller = min(enclosing, key=lambda p:p[0].end_byte-p[0].start_byte)
            sites.append({'path':rel,'line':call.start_point.row+1,'end':call.end_point.row+1,
                          'from':caller,'written_name':call.text.decode(),'kind':call.type})
            if len(sites)>limits['SITE']:raise ValueError('whole-unit refusal: syntax site cap')
        # Reflective dispatch can otherwise impersonate an unexecuted literal
        # call on the same line. Reject the complete enclosing line/span.
        stack = [tree.root_node]
        while stack:
            node = stack.pop()
            stack.extend(node.named_children)
            if node.type == 'call':
                method = node.child_by_field_name('method')
                if method is not None and method.text in [b'send',b'public_send',b'__send__',b'eval',b'instance_eval',b'class_eval',b'module_eval']:
                    blocked.append({'path':rel,'line':node.start_point.row+1,'end':node.end_point.row+1})
    if not files:
        raise ValueError('refuse empty syntax population')
    print(json.dumps({'tier':'C denominator/scope only','files':files,'sites':sites,'nonliteral_lines':blocked},sort_keys=True))


if __name__ == '__main__':
    main()
