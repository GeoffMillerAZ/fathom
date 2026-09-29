# ruby-require — tier-B Ruby import oracle (LB-F40)

`ruby_require_dump.rb` writes `indexbench lang-oracle -tool-oracle`'s neutral `imports` for a Ruby
subject, scored as L3 `import_resolution` (design §1.2, §10). Ruby's own parser (Ripper, the MRI
parser's event interface) finds every receiverless `require`, `require_relative`, `load` and
`autoload` whose path is a string literal, and Ruby's own feature lookup resolves it:
`RubyVM.resolve_feature_path` (Ruby 2.6) or `$LOAD_PATH.resolve_feature_path` (2.7 and later) for
`require`, `autoload` and `require_relative` (the path expanded against the requiring file), and
`load`'s rule (a `./`/`../` path from the subject root, else each load-path directory, then the
subject root, no extension added). The lookup runs with `$LOADED_FEATURES` emptied, so a feature the
script itself loaded (`json`, `ripper`) still resolves to a file. Only targets inside the subject are
written; `stderr` counts the rest per form (`require_outside`, `require_unresolved`,
`require_computed`, ...).

Tier B, not A: the lookup is Ruby's, but the load path is this run's (`-I`, default `lib` when it
exists) — the operator's configuration, standing in for a gemspec's `require_paths` and whatever the
program adds to `$LOAD_PATH` at run time, which no static run sees. A require with a computed path
(`require File.join(...)`, an interpolated string, `Dir[...].each { require it }`) is counted, never
resolved: fathom's file-symbol strategy cannot see it either.

    ruby ruby_require_dump.rb -subject <checkout> [-I lib]... > workspace/tmp/oracle/ruby-<subject>-imports.json
    go run ./cmd/indexbench lang-oracle -dialect ruby -subject <checkout> -tool-oracle workspace/tmp/oracle/ruby-<subject>-imports.json ...

System Ruby 2.6.10 on macOS runs it (no gems). `cmd/indexbench/testdata/ruby-require-rubyfixture.json`
is its output on `internal/fusion/testdata/rubyfixture` (`files 4 require_in_subject 1
require_outside 1 require_relative_in_subject 1`: `require 'json'` resolves to the standard library,
outside the subject). Bench-only; never a product dependency.
