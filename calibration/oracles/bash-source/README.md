# bash-source — tier-B Bash import oracle (LB-Q-bash L3, wave lf6)

`bash_source_dump.py` writes `indexbench lang-oracle -tool-oracle`'s neutral `imports` for a Bash
subject, scored as L3 `import_resolution` and, with no SCIP index, as the file-edge truth (design
§1.2, LB-F40, LB-F57). Bash has no SCIP producer, and no static tool prints the file a `source`
opens, so the statement and its file are read by two independent parts and checked by a third:

- **Statements**: shfmt `--to-json` (mvdan.cc/sh/v3, BSD-3-Clause, `-ln bash`; the parser
  `calibration/oracles/parser-ranges` already reads) gives every simple command. A command word
  that is `source` or `.` after quote removal (`\.` is `.`), or `builtin`/`command` before one of
  them, makes a source statement; its first argument is the file. Literal text, single quotes and
  double quotes holding no expansion are literal; any expansion (`$VAR`, `${X}`, `$(dirname "$0")/x`,
  `~`) is dynamic and an unquoted `*`, `?` or `[` a glob. Dynamic and glob statements are counted,
  never resolved: no static reader can, fathom's included.
- **Lookup**: bash 5's own rule for the `.` builtin (bash(1)). A name with a slash opens relative to
  the current directory, an absolute one is outside the subject; a slash-less name is searched in
  PATH (which holds no subject directory) and then, outside POSIX mode only, in the current
  directory. A script whose `#!` names `sh` runs in POSIX mode (bash as sh, or dash), so its
  slash-less names are counted (`source_posix_path_only`) and never resolved.
- **Current directory**: the operator's choice, as ruby-require's load path is: a static run cannot
  know where a script is started. Each literal path is looked up from the script's own directory,
  then from the subject root, the first regular file inside the subject winning. That is
  ShellCheck's `-x -P SCRIPTDIR` order run from the root.
- **Check**: with `--shellcheck`, ShellCheck 0.11.0 (GPL-3.0, a release binary run as a tool, never
  linked; `--extended-analysis=false`, which skips the data-flow pass and leaves source following
  alone) runs `-x -P SCRIPTDIR -f json1` from the subject root. ShellCheck prints no file it
  follows, only SC1091 for a statement it cannot (measured 2026-10-10: `-a` names a sourced file
  only when that file has a finding of its own), so the check is per statement: each one this
  script resolves must be followed, each relative literal one it leaves unresolved must draw
  SC1091. The agreement is in the file's `disclose` and the reading's notes. An absolute path is
  left out of the check (a `# shellcheck disable=SC1091` directive silences it: nvm.sh:3374).

Tier B, not A: the parser is not bash's, and the current directory is the operator's (§1.3). Tier B
caps L3 at good, the queue's stated ceiling for Bash without a tier-A static oracle.

    GOBIN=<tools>/bin go install mvdan.cc/sh/v3/cmd/shfmt@v3.14.1
    curl -sLO https://github.com/koalaman/shellcheck/releases/download/v0.11.0/shellcheck-v0.11.0.darwin.aarch64.tar.gz
    #   sha256 339b930feb1ea764467013cc1f72d09cd6b869ebf1013296ba9055ab2ffbd26f
    python3 -I bash_source_dump.py --shfmt <tools>/bin/shfmt --shellcheck <dir>/shellcheck \
        --ext .sh --ext .bash --shebang bash --shebang sh <clone> > out/bash-<subject>-imports.json
    go run ./cmd/indexbench lang-oracle -dialect bash -subject <clone> -subject-name <owner/repo> \
        -tool-oracle out/bash-<subject>.json -tool-oracle out/bash-<subject>-imports.json -out <reading>

`out/bash-<subject>.json` is the L2 calls oracle (`calibration/oracles/ts-tags`, run with
`--module-caller '<module>'` since the bash dialect mints a module symbol, K18), with its
`.ranges.json` companion (`calibration/oracles/parser-ranges`). Files are the bash manifest's
(`.sh`, `.bash`, an extensionless file whose `#!` names bash or sh), as shfmt_ranges.py selects them.

Measured 2026-10-10 on the tuning subjects: nvm-sh/nvm 1889911f, 349 statements in 277 files, 342 in
the subject (`\. ../../nvm.sh` and `\. ../../common.sh` from test scripts), 6 dynamic, 1 absolute;
ShellCheck agrees on 342 of 342 relative literal statements. bats-core/bats-core 713504bc, 34
statements, every one dynamic (`source "$BATS_ROOT/$BATS_LIBDIR/..."`), so bats-core gives no
denominator. Bench-only; never a product dependency.
