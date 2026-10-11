# zsh grammars against the 5% ERROR-file gate (LB-F454, O792; wave lf6, item shell)

**Verdict: no maintained zsh-capable tree-sitter grammar passes the gate, so nothing routes `.zsh`,
`.ksh` or a zsh or ksh shebang. On ohmyzsh, the best grammar (georgeharker/tree-sitter-zsh v0.63.5)
still leaves an ERROR or MISSING node in 63 of 396 `.zsh` and zsh-shebang files, 15.9%, three times
the 5% kill line (§14.2). tree-sitter-bash, which the bash dialect parses with, leaves one in 263 of
396, 66.4%. A zsh script stays unextracted and disclosed, as before (O792). LB-F454 is written `[-]`
with this measurement as its reason; its trigger is a grammar under 5% on the same corpus.**

Measured 2026-10-10 at fathom e7bbd99e, before any routing change (LB-F454's order: the table first).

## Subject

ohmyzsh/ohmyzsh 42a4ccb1b14dbeffe81259105a5243b4f4cb618e (MIT, `LICENSE.txt` read at the pin; O792's
pin). The gate's file set is every regular file named `*.zsh` and every extensionless file whose `#!`
line names zsh (the dispatcher's rule: the interpreter, or env's first non-flag argument, a version
suffix allowed): 388 + 8 = 396 files. `.zsh-theme` (143 files) is reported beside it, because O792's
541 counted it; no dialect would route that extension, and the two symlinks O792 also counted
(`themes/macovsky-ruby.zsh-theme`, `plugins/per-directory-history/per-directory-history.plugin.zsh`)
are left out as copies. Four `.sh` files carry a zsh shebang (`tools/upgrade.sh`, `tools/changelog.sh`,
`tools/theme_chooser.sh`, `plugins/wd/wd.sh`); the extension claims them for bash today, and they are
not in the set.

## Method

tree-sitter CLI 0.27.0, `tree-sitter parse -p <grammar checkout> --paths <list> -q -j`, each grammar
compiled natively from its own `src/parser.c` and `src/scanner.c` at the pin. A file counts when its
summary says `successful: false`, which the CLI sets when the tree's root has an error
(`ts_node_has_error`: an ERROR or a MISSING node), the same test as the extractor's `tsHasError` and
`indexbench lang-oracle`'s `files_with_error`. Cross-check: tree-sitter-bash at the lock's pin counts
291 files over O792's set, the number O792 read with the harness's own probe. A parse that does not
finish is counted as a file with an error: `plugins/scd/scd` (16 KB) runs past a 5 s `--timeout` and
grows to 3.9 GB resident under both georgeharker's and zchee's grammars, and returns no tree.

Candidates: every grammar found by `gh search repos tree-sitter-zsh` (2026-10-10) that is maintained
(a commit in the twelve months before the measurement, not archived) and carries an OSI licence read at
its pin (`indexbench lang-pin -tree`), plus tree-sitter-bash.

| Grammar | Pin | Licence (lang-pin -tree) | `.zsh` | zsh shebang | **`.zsh` + shebang (the gate)** | `.zsh-theme` | O792's set |
|---|---|---|---|---|---|---|---|
| tree-sitter/tree-sitter-bash | v0.25.1 a06c2e44 (the lock's) | MIT | 256/388 66.0% | 7/8 | **263/396 66.4%** | 28/143 19.6% | 291/539 54.0% |
| georgeharker/tree-sitter-zsh | v0.63.5 7a593401 | MIT | 58/388 14.9% | 5/8 | **63/396 15.9%** | 18/143 12.6% | 81/539 15.0% |
| zchee/tree-sitter-zsh | 895890f6 (main, no tag) | Apache-2.0 | 60/388 15.5% | 6/8 | **66/396 16.7%** | 14/143 9.8% | 80/539 14.8% |
| jasonyukr/tree-sitter-zsh | 00f0e37e (main, no tag) | MIT | 156/388 40.2% | 6/8 | **162/396 40.9%** | 24/143 16.8% | 186/539 34.5% |

Not measured, with the reason:

| Repository | Why not |
|---|---|
| tree-sitter-grammars/tree-sitter-zsh | archived (last push 2023-08-26), no licence file |
| hexium310/tree-sitter-zsh | archived (last push 2021-10-23) |
| baodrate/tree-sitter-zsh, docwhat/tree-sitter-zsh | no licence; no commit since 2023 and 2022 |
| ES-SR/tree-sitter-zsh-grammar | no licence |
| unforgivenii147/tree_sitter_zsh-0.63.4 | one "initial" commit (2026-09-20) holding georgeharker's v0.63.4 package: a copy, not a grammar of its own |

## ksh

Nothing routes ksh either. No ksh grammar was measured: no maintained tree-sitter grammar names ksh, and
tree-sitter-bash's rate on a ksh corpus would decide only a bash route, which this item does not take
while zsh's does not pass. `.ksh` and ksh shebangs stay unextracted and disclosed, named in LB-F454's
disposition.

## Alternative zsh subject set (pinned before any reading of it)

LB-F454's brief lets tree-sitter-bash route zsh only on a committed alternative subject set. These four
were pinned and their licences read before any of their files was parsed; the reading below is added
after this commit and is read once:

| Subject | Pin | Licence |
|---|---|---|
| zsh-users/zsh-autosuggestions | 85919cd1ffa7d2d5412f6d3fe437ebdbeeec4fc5 | MIT |
| zsh-users/zsh-syntax-highlighting | e6683f20e5c38d3e5447f45845d04ddda75d0f23 | BSD-3-Clause |
| romkatv/powerlevel10k | d05a1b00f9a61f9578bf9dc19b8451942dde8734 | MIT |
| sorin-ionescu/prezto | cff2d01871425b1b80710f8ec6a475c5a53145b4 | MIT |

The same file set rule (regular `*.zsh` files and extensionless zsh-shebang files) and the same method;
the four subjects are pooled, and a grammar passes the alternative set at under 5% of the pooled files.
