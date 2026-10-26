# Controlled Git read research

GT6, 2026-10-24. [0040](../intent/decisions/0040-git-reads-require-registered-metadata.md)
and [GT-G1..5](../designs/command-git.md) retain decisions and executable acceptance.
Two independent read-only reviews covered existing kit code and primary Git docs.

| Source | Consequence for the fixed profile |
|---|---|
| [Repository layout](https://git-scm.com/docs/gitrepository-layout), [worktrees](https://git-scm.com/docs/git-worktree) | Gitfile/common-dir/object/index topology must be registered before discovery; linked worktrees legitimately split admin and common state. |
| [Config](https://git-scm.com/docs/git-config) | Includes redirect reads; environment scrubbing alone cannot protect metadata. Parse pinned private copies with `--no-includes`, then reject unqualified source/execution settings. |
| [Status](https://git-scm.com/docs/git-status) | Porcelain v2 and NUL separators are machine formats; optional locks may update the index. Ignored paths, submodules and live drift require explicit coverage limits. |
| [Attributes](https://git-scm.com/docs/gitattributes), [diff options](https://git-scm.com/docs/diff-options) | Attributes can select filters/textconv/external diff. Fixed raw object reads and qualified config avoid silently invoking them. |
| [cat-file](https://git-scm.com/docs/git-cat-file), [diff-tree](https://git-scm.com/docs/git-diff-tree) | Use raw full-ID change records and sized unfiltered object reads, then verify type/size/hash before disclosure. |
| [Log](https://git-scm.com/docs/git-log), [rev-list](https://git-scm.com/docs/git-rev-list) | Pretty log can verify signatures and expose notes/mailmap behavior. Fixed traversal plus raw commit reads preserves exact bounded source evidence. |
| [Git environment](https://git-scm.com/docs/git) | Strip ambient redirection; explicitly disable lazy fetch, replacements, protocols, helpers, pager, optional locks and system/global config. |

Existing `cmd/quality-gate/git.go` supplies trusted-executable resolution, bounded
raw NUL parsing, 40/64-character OIDs, blob size preflight and poisoned-environment/
linked-worktree/partial-clone fixtures. Its write-tree/hash-object writes, arbitrary
internal argument surface, caller index override and inherited non-Git environment
are inappropriate here. Mine the read safeguards; retain separate command ownership.
No exported library or generic Git/shell executor is needed for these three recipes.

## Native protocol preflight

Observed installed Git `2.54.0` on macOS arm64. A private copy of its canonical Mach-O
executable ran `version` with empty PATH/minimal environment and fixed no-pager,
no-optional-locks, no-lazy-fetch and no-replace-objects flags. Executable SHA-256:
`90f3250b0bd80bc9d6026e1f4a10d175e04a8e1302fdaf662acb28b9bf5ebc24`.

The private config parser returned `key\nvalue\0` records with `--no-includes`,
including a deliberately missing include path without following it. That is a
preflight witness, not permission to accept includes in registered repository config.
Pinned Gitleaks 8.30.1 observed all 71 bytes of a synthetic normalized Git metadata
JSON sample; required-scanner composition does not need a MIME-forcing prefix.

No project metadata/history, keys, global config or external model was inspected in
these protocol probes. Qualification still requires actual typed-entrypoint hostile
and no-write cases. Copied-binary execution, clean source output or a stable pre/post
hash cannot establish OS isolation, atomic live status, complete history or authority.
