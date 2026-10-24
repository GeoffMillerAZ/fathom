### Fixed

- The Bash write hook now recognises BSD `sed -I` (alone, in a flag cluster such as `-EI`, or with an attached suffix) and abbreviated GNU `--in-place` forms (`--in-pl`, `--in-place=SUF`) as in-place writes and names their targets.
- The Bash write hook no longer reports `%`, glob patterns or `{}` placeholders as written files. When a `sed` also names such an operand, the rules of that operand's directory are served alongside the named files' rules.
- `FATHOM_TENANCY_BUDGET` accepts a plain byte count (`536870912`) as well as an IEC size (`512MiB`); any other value is rejected with a message naming both forms.
