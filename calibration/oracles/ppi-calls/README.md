# ppi_calls.pl — tier-C Perl call oracle

PPI (the pure-Perl parser library Perl::Critic is built on) over a checkout, run by the machine's own `perl`
(5.34.1, PPI 1.270 from `/System/Library/Perl/Extras`; no CPAN install). Never fathom's tree-sitter engine.

    perl ppi_calls.pl <checkout> > out/perl-<subject>-calls.json
    # mojolicious/mojo v9.50: 280 files / 4,207 pairs, Perl-Critic/Perl-Critic v1.154: 263 / 2,696; 0 parse failures

Output is the neutral oracle shape (`definitions` empty, `calls` as `{path, from, to}`), scored by `indexbench lang-oracle
-tool-oracle`, which takes definitions from universal-ctags (tier B). A pair is a call with the named sub that encloses it
(a call outside every sub, in a `BEGIN` block or in `use` is skipped), once per (file, sub, callee). A callee is:

- a Word followed by a parenthesised list: `name(...)`, `Pkg::name(...)`;
- a Word directly followed by a term (a variable, a quote, a number, `\`): `name $x, ...`. PPI cannot tell a predeclared sub
  from a bareword, so this is the heuristic part, and the part the dialect and the oracle can disagree on: the dialect reads
  a block call (`any { ... } @list`, `first { ... } @list`) and a `throw_x key => $v` call (a bareword hash key after the name) the oracle's term test misses (125 of the
  dialect's 7,023 pairs, 1.8%);
- the Word after an arrow: `$obj->name`, `Class->name(...)`.

A function named like a Perl builtin or keyword (perlfunc 5.34's entries minus its sub-headings, plus
try/catch/finally/defer/isa, with or without `CORE::`) is no callee, a method of that name is. The dialect's `tags.scm`
excludes the same list, so the oracle is independent in parser and runtime, not in what counts as a call.
