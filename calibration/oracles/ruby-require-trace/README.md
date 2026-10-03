# ruby-require-trace — a runtime require trace, tier A for what it attests (LB-F256)

`ruby_require_trace.rb` writes `indexbench lang-oracle -tool-oracle`'s neutral `imports` for a Ruby subject, as
`../ruby-require/` does, but Ruby evaluates the subject instead of a static lookup over a load path the
operator chooses. Each subject file is loaded in its own child process. The child runs under the subject's own
bundle (the driver runs under `bundle exec` in the subject root, and a nested gem with a Gemfile of its own runs
under that one), with the load path its test task gives (`-I`) and the features its runner loads first (`-r`).
`trace_hook.rb`, prepended to `Kernel` and `Module`, records every `require`, `require_relative`, `load` and
`autoload` a subject file executes. Each record carries the file Ruby's own lookup
(`$LOAD_PATH.resolve_feature_path`) names at that moment, under the program's `$LOAD_PATH`. A child exits with
`exit!`, so no test suite or at_exit hook runs, and its process group is killed after it (a test that boots a
server at load, as sinatra's `test/integration_test.rb` does, leaves nothing running). The trace runs the
subject's code, so use a scratch checkout.

The tier rule is design §1.3. Only executed statements with a literal path are written, and only those whose
target is a subject file: the statements an extractor of import statements can see. The file is tier A when they
are at least 90% of the subject's in-repo literal statements, and tier B below that. A statement no child
executed counts as in-repo when the same lookup from the driver's load path names a subject file. stderr names
the counts and the ratio. A `require` with a computed path that ran (`Dir[...].each { |f| require f }`) is
counted (`computed_executed`) and never written.

Installed without sudo, under the worker's scratch directory only: Homebrew's relocatable Ruby
(`gh release download 3.3.8 -R Homebrew/homebrew-portable-ruby -p 'portable-ruby-3.3.8.arm64_big_sur.bottle.tar.gz'`,
sha256 `7645e2d6…57d63e`, ruby 3.3.8), with `GEM_HOME`, `BUNDLE_PATH` and `BUNDLE_USER_HOME` under the same
directory. Each subject then gets `bundle install`, and so does each nested gem with its own Gemfile.

    cd <sinatra> && bundle exec ruby ruby_require_trace.rb -subject . -I lib -I test -I . \
        -unit sinatra-contrib:lib,spec:rspec/core,spec_helper -unit rack-protection:lib,spec:rspec/core,spec_helper > out/trace-sinatra.json
    cd <jekyll>   && bundle exec ruby ruby_require_trace.rb -subject . -I lib -I test -r jekyll > out/trace-jekyll.json
    cd <ruby-lsp> && bundle exec ruby ruby_require_trace.rb -subject . -I test -I lib -r ruby_lsp/internal > out/trace-ruby-lsp.json
    cd <spoom>    && bundle exec ruby ruby_require_trace.rb -subject . -I test -I lib -r spoom > out/trace-spoom.json

Each load path is the subject's own. Minitest's `TestTask` gives `lib test .` (sinatra) and the Rakefiles' `t.libs`
give `lib test` (jekyll, ruby-lsp, spoom). RSpec's `-I lib -I spec` and the `.rspec` file's `--require spec_helper`
apply to sinatra's nested gems. `-r` loads the gem's own entry, so a lib file loaded alone finds the constants it
reopens.

Measured 2026-10-02 (wave lx), 10 to 30 s each with 8 jobs:

| subject | files | loaded | raised / timed out | in-repo statements attested | tier |
|---|---|---|---|---|---|
| sinatra/sinatra v4.2.1 | 147 | 144 | 3 / 0 | 145 of 147 (.9864) | A |
| jekyll/jekyll v4.4.1 | 159 | 142 | 17 / 0 | 114 of 119 (.9580) | A |
| Shopify/ruby-lsp v0.26.11 | 371 | 291 | 76 / 4 | 189 of 193 (.9793) | A |
| Shopify/spoom v1.8.10 | 165 | 165 | 0 / 0 | 201 of 201 (1) | A |

Most of the files that raise are not meant to load alone: ruby-lsp's `test/fixtures/` (code samples for the server
under test), jekyll's `benchmark/` (benchmark-ips is outside its bundle unless `BENCHMARK` is set) and its RuboCop
cops and Cucumber steps. A statement left unexecuted sits inside a method (jekyll's `serve.rb` loads its servlet
when it starts), in a file that raised first, or in an autoload a method registers.

Scored by `indexbench lang-oracle -tool-oracle <trace>` without `-scip` (LB-F57: the trace's pairs are the file-edge
truth too), against the ruby dialect at LB-F254. The trace names files the static dump's load path never reaches, and
the resolver misses those (O547):

| subjects | import_resolution | file_edge_precision | file_edge_recall |
|---|---|---|---|
| sinatra + jekyll | .8031 (208 of 259) | .9457 (122 of 129) | .4710 (122 of 259) |
| ruby-lsp | .7513 (142 of 189) | .9524 (60 of 63) | .3175 (60 of 189) |
| spoom | .5417 (104 of 192) | 1 (54 of 54) | .2813 (54 of 192) |

Every import the resolver misses names a load-path root that `imports.source_roots` (`lib`, the repository root)
does not hold:

- sinatra's 51 are its nested gems' `lib/`: `rack-protection/lib` 39 and `sinatra-contrib/lib` 12. Bundler puts
  both on the load path as path gems.
- ruby-lsp's 47 and spoom's 88 are `test/`, the load path the Rakefile's `t.libs << "test"` gives:
  `require "test_helper"` from a nested test directory.

Five of jekyll's six fathom-only file edges follow statements the trace did not execute. They were in the static
dump's truth. Bench-only: never a product dependency.
