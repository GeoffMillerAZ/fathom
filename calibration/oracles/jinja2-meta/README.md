# jinja2-meta — Jinja's own parse and template lookup (LB-Q-jinja2)

Tier A for definitions and import edges (the tool's own evaluation), tier A for calls
only as far as Jinja's AST names a callee; one script, no product dependency.

Install (pinned, under `workspace/tools`, never global):

    pip install --target workspace/tools/jinja2pkg jinja2==3.1.6

Run, one subject at a time:

    python3 -I calibration/oracles/jinja2-meta/jinja2_meta_dump.py \
      -subject workspace/calibration/jinja2/<subject> \
      -pythonpath workspace/tools/jinja2pkg -out workspace/tmp/oracle/jinja2-<subject>.json

- `definitions`: `{% macro %}`, `{% block %}` and `{% set name = … %}` nodes of Jinja's AST, at
  the opening tag's lines (`line` of its `{%`, `end` of its `%}`, read from Jinja's lexer: the AST has no end line).
- `imports`: the constant template names `jinja2.meta.find_referenced_templates` reports
  (extends, include, import, from-import), each resolved by `jinja2.FileSystemLoader` over
  the importing file's directory and each ancestor directory up to the subject root. The
  search path is this run's configuration, as `-I` is for the Ruby require dump; a name
  that resolves to no file of the subject is dropped. A computed name (`{% include x %}`)
  is counted on stderr, never a pair.
- `calls`: a call whose callee is a bare name (a macro, a global) or an attribute (`forms.input()`, `x.items()`), by the callee's name.
  The caller is `<module>`: a macro's body is not a definition range in the tags dialect.
- Files Jinja cannot parse (a Salt `import_yaml`, an Ansible-only tag) are counted on
  stderr and left out.

`runners.json` for `task lang:loop`, keyed by the queue entry's exact oracle name:

    {"jinja2.meta.find_referenced_templates (A, extends, include and import edges)":
      {"command": "python3", "args": ["-I", "calibration/oracles/jinja2-meta/jinja2_meta_dump.py",
       "-subject", "{subject}", "-pythonpath", "workspace/tools/jinja2pkg", "-out", "{out}"],
       "format": "tool"}}
