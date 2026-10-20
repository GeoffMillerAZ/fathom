#!/usr/bin/env python3
"""Emit tags.scm (python3 gen_tags.py > tags.scm): the definitions of a Nix file are the file's root expression's own names.

The root expression is the file's expression peeled through at most three wrappers (a function,
a let, a with); its let bindings, its attribute set's bindings (depth one) and the formals of
its functions are the definitions. Tree-sitter queries cannot say "the nearest enclosing binding
is none", so the chains are enumerated here.
"""
import itertools, sys

WRAP = {"fn": "function_expression", "let": "let_expression", "with": "with_expression"}
OPTION_HEADS = "mkOption|mkEnableOption|mkPackageOption"
PACKAGE_HEADS = ("callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|"
                 "buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin")
MAXW = 3

NAME1 = "attrpath: (attrpath . attr: (identifier) @name .)"        # a one-identifier key
NAMEN = "attrpath: (attrpath . attr: (identifier) @name)"          # first identifier of a dotted key

def headre(heads):
    return '(#match? @_v "^([A-Za-z0-9_\'-]+[.])*(%s)\\\\s")' % heads

def binding(kind):
    if kind == "function":
        return ("(binding %s expression: (function_expression body: (_) @body)) @definition.function" % NAME1, "")
    if kind == "option-value":
        return ("(binding %s expression: (apply_expression) @_v @body) @definition.option" % NAME1, headre(OPTION_HEADS))
    if kind == "option-key":
        return ("(binding %s expression: (_) @body) @definition.option" % NAMEN, '(#eq? @name "options")')
    if kind == "package":
        return ("(binding %s expression: (apply_expression) @_v @body) @definition.package" % NAME1, headre(PACKAGE_HEADS))
    if kind in ("attribute", "variable"):
        return ("(binding %s expression: (_) @body) @definition.%s" % (NAMEN, kind), "")
    raise SystemExit(kind)

def chain_text(chain, last):
    def build(i):
        t = WRAP[chain[i]]
        if i == len(chain) - 1:
            return "(%s %s)" % (t, last)
        return "(%s body: %s)" % (t, build(i + 1))
    return build(0)

patterns = {"narrow": [], "variable": [], "broad": [], "param": []}

def add(bucket, label, body, pred):
    pr = ("\n " + pred) if pred else ""
    patterns[bucket].append("; %s\n(%s%s)\n" % (label, body, pr))

chains = [()]
for k in range(1, MAXW + 1):
    chains += list(itertools.product(WRAP, repeat=k))

def cname(c):
    return "/".join(c) if c else "root"

for kind in ("function", "option-value", "option-key", "package", "attribute", "variable"):
    pat, pred = binding(kind)
    bucket = {"attribute": "broad", "variable": "variable"}.get(kind, "narrow")
    for c in chains:
        if kind == "variable":
            if c and c[-1] != "let":
                continue
            if not c:
                continue
            term = "(binding_set binding: %s)" % pat
            body_ = "source_code expression: " + chain_text(c, term)
        else:
            term = "(_ (binding_set binding: %s))" % pat
            if c:
                body_ = "source_code expression: " + chain_text(c, "body: " + term)
            else:
                body_ = "source_code expression: " + term
        add(bucket, "%s in %s" % (kind, cname(c)), body_, pred)

for c in chains:
    if not c or c[-1] != "fn":
        continue
    for last in ("formals: (formals formal: (formal name: (identifier) @name) @definition.param)",
                 "universal: (identifier) @name @definition.param"):
        add("param", "param in %s" % cname(c), "source_code expression: " + chain_text(c, last), "")

REFS = """; calls: the head of an application, the identifier or the last attribute of the selected name
(apply_expression function: (variable_expression name: (identifier) @name)) @reference.call

(apply_expression function: (select_expression) @name) @reference.call
"""

HEADER = """; source: written for this dialect (the grammar's own queries/tags.scm names only functions and calls). GENERATED:
; `python3 gen_tags.py > tags.scm` in this directory, do not edit by hand.
;
; The definitions of a Nix file are its root expression's own names: the root is the file's expression peeled
; through at most three wrappers (a function, a let, a with), and its let bindings, the bindings of its attribute
; set (depth one, rec or not) and the formals of its functions are the definitions. A deeper key (config.x.y, a
; nested set, a function's inner let) is not. A dotted key a.b.c = v is the definition `a` (the key at depth one,
; as `nix-instantiate --parse` merges them), written once per binding. Kinds: function (the key's value is a
; function), option (the key is `options`, or its value is mkOption / mkEnableOption / mkPackageOption), package
; (its value is callPackage / mkDerivation / a build* helper applied), else attribute (an attribute set's key)
; or variable (a let binding). A function's formals and its single argument are `param`s. An application's
; head is a call: the identifier, or the selected name as written (lib.mkIf).
"""

def main():
    out = [HEADER]
    out += patterns["narrow"] + patterns["variable"] + patterns["broad"] + patterns["param"]
    out.append(REFS)
    sys.stdout.write("\n".join(out))

main()
