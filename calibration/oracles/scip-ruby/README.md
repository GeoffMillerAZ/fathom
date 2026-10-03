# scip-ruby — tier-A SCIP index, diagnostic on untyped Ruby (LB-Q-ruby)

Installed without sudo into a tools dir (nothing global): the release binary of sourcegraph/scip-ruby, a Sorbet build,

    gh release download scip-ruby-v0.5.0 -R sourcegraph/scip-ruby -p scip-ruby-arm64-darwin     # 19,470,600 bytes, MIT/Apache-2.0
    shasum -a 256 scip-ruby-arm64-darwin   # 4aa360b87be7f045e979a754450b586a8580e9a7b2d089a9a1d6f3b7819a54eb
    ./scip-ruby-arm64-darwin --version     # scip-ruby 0.5.0 git e847fe92 (Sorbet 0.6 e8f0eb82)

Run in each subject's root (no Sorbet config, no `sorbet/` directory, no bundle), 0.3 s each:

    ./scip-ruby-arm64-darwin --index-file <tools>/sinatra.scip --gem-metadata sinatra@4.2.1 --no-config .
    ./scip-ruby-arm64-darwin --index-file <tools>/jekyll.scip  --gem-metadata jekyll@4.4.1  --no-config .

`No errors! Great job.` on both. What the index carries: definitions for every class, module, method and constant with an
`enclosing_range` on most (129 of 4,670 carry none: the harness takes the mapped fathom definition's lines, D5), and references
only where Sorbet resolves the target. A file with no `# typed:` sigil is `typed: false` and Sorbet reports nothing about a call on
a receiver whose type it cannot infer: `params.fetch`, `settings.foo`, `x.each` on an untyped local, and any call through an
untyped `attr_reader`. The call truth is therefore incomplete on an ordinary Ruby codebase, which is what the queue entry means
by "Sorbet codebases only".

Scored by `indexbench lang-oracle -scip` on sinatra v4.2.1 + jekyll v4.4.1 (316 files), against the ruby dialect at LB-Q-ruby:
L2 `call_recall` .5382 and `call_precision` .3869 (7,355 fathom pairs, 5,289 in the truth: the dialect lists calls the index
cannot see), `def_mapping` .6139, `typed_recall` .4488, L3 `file_edge_precision` .8 (12 of 15 fathom file edges are in the
truth) and `file_edge_recall` .0124 (12 of 965 SCIP file edges). These are not the reading's L2 numbers: an index that omits every
untyped call cannot grade a syntactic call collector, so the reading's L2 oracle is the grammar's own tags query
(`../ts-tags/`, tier C), and the numbers above are disclosed in the reading's notes.

## Sorbet-typed subjects (wave lx, LB-F257, LB-F258)

The `-rest` trigger asked for a typed subject: two MIT gems at release tags, each indexed WITH its own
`sorbet/config` and RBIs (`sorbet/rbi/`), never `--no-config`, run in the subject root:

    ./scip-ruby-arm64-darwin --index-file <tools>/ruby-lsp.scip --gem-metadata ruby-lsp@0.26.11   # Shopify/ruby-lsp 6745c162
    ./scip-ruby-arm64-darwin --index-file <tools>/spoom.scip    --gem-metadata spoom@1.8.10      # Shopify/spoom a621b82c

Sorbet reports `No errors! Great job.` on both: 0 errors, 0.15 s and 0.1 s. Both configs pass `--parser=prism` and
`--enable-experimental-rbs-comments`, which Sorbet 0.6 e8f0eb82 accepts. ruby-lsp: 182 files carry a sigil (99
`strict`, 78 `true`, 5 `false`), and its config ignores `test/fixtures/`, `test/expectations/` and `jekyll/`. spoom:
all 165 files carry one (91 `strict`, 1 `strong`, 73 `true`). Index sizes are 4.6 MB and 2.6 MB.

Ruby's SCIP options (`SCIPOptionsFor("ruby")`, `internal/langbench/scip_ruby.go`) take the oracle-blind rule
(LB-F175): a fathom pair the truth lacks is left out when no written call site of it (`name(`) carries an
occurrence. Sorbet writes none on a T.untyped receiver or at `super`. That leaves out 493 of 16,965 ruby-lsp pairs
(2.9%) and 118 of 7,813 spoom pairs (1.5%), under LB-F203's 10% cap. Scored against the ruby dialect at LB-F254
(the locals rule):

| subject | call_recall | call_precision | def_mapping | typed_recall | file_edge_precision | file_edge_recall |
|---|---|---|---|---|---|---|
| ruby-lsp | .8355 (9,628 of 11,523) | .5845 (9,628 of 16,472) | .8336 | .7899 | 1 (63 of 63) | .0613 (63 of 1,027) |
| spoom | .8566 (5,780 of 6,748) | .7511 (5,780 of 7,695) | .8083 | .7381 | 1 (54 of 54) | .0794 (54 of 680) |

The misses, classified from `-unmatched` (O545, O546):

- **Constant references.** fathom-only pairs: ruby-lsp 4,931 of 6,844, spoom 1,544 of 1,915. The dialect records a
  constant reference as a call (upstream tags.scm's contract, which the tier-C grader uses). scip-ruby references the
  class as `Foo#`, which is not callable, so no truth pair can hold it.
- **`Foo.new`.** ruby-lsp 566 fathom-only `new` and 566 oracle-only `initialize`; spoom 189 and 210. Sorbet resolves
  `Foo.new` to `Foo#initialize` at the `new` it writes there.
- **Operators.** Oracle-only: ruby-lsp 1,039, spoom 393 (`[]`, `==`, `<<`, `+@`, `!`). Sorbet references the operator
  method. The dialect, like upstream, records no operator as a call.
- **Class-body calls.** Oracle-only: ruby-lsp 206, spoom 211. The index pairs a call written in a class or constant
  body (`private`, `include`, `CONST = [...].freeze`) with that class or constant. The dialect pairs a call with
  its enclosing method only.
- **Untyped receivers written without parentheses.** The rest of the fathom-only pairs: ruby-lsp 1,347 (970 in tests,
  `response`, `process_message`, `pop_response` on a test's server object), spoom 182. Sorbet writes no occurrence
  there, and with no `name(` the oracle-blind rule cannot find the site.
- **Attribute writes.** A few oracle-only pairs: `obj.x = v` is `x=` to the index and `x` to the dialect.

Reading constants as calls, `new` as the `initialize` the index names, and leaving out operators and class-body
calls are harness rules inside `ScoreSCIPWith`. That code is outside wave lx's item. The file-edge recall is the
resolver's. ruby-lsp and spoom require their files from entry files (`ruby_lsp/internal.rb` holds 77 requires;
`spoom.rb` requires each subsystem's entry, which requires its parts), and a constant that no require of its own file
brings binds nothing (O546). With the extractor and the resolver as
they are, the typed subjects would lose L3, so they are measured and not graded. The reading keeps sinatra and jekyll
(docs/plans/language-breadth.md F.26, the re-pin rule).

L1 on the typed subjects against universal-ctags 6.2.1 reads sym_recall .6752 (ruby-lsp) and .551 (spoom). ctags tags
the Ruby source that the tests embed in heredoc strings (`index(<<~RUBY) class A ... RUBY`): 1,544 of ruby-lsp's 1,574
oracle-only definitions and 1,922 of spoom's 1,940 sit inside a `heredoc_body` (O548).
