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
