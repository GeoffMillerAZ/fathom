; dialects/bash/imports.scm — `source FILE` and `. FILE` with a literal path (LB-Q-bash L3, wave lf6).
; source: written for this dialect. bash(1), the `.` builtin: the first argument names the file,
; read and executed in the current shell; the rest are its positional parameters. `\.` is `.`
; after quote removal (nvm writes it to step around an alias), and `builtin source` / `command .`
; reach the same builtin. A literal path is an unquoted word, a single-quoted string, or a
; double-quoted string holding nothing but text; a word with a glob character, a tilde or a
; backslash, and any expansion ($VAR, ${X}, $(dirname "$0")/x, "$DIR/x.sh") names no file
; statically and is left out (the bash-source oracle counts them). The manifest's file-symbol
; strategy resolves the path: `./` and `../` against the script's directory, any other path
; against the script's directory and then the repository root (design §6). The whole file is
; brought into scope, as `source` executes every top-level definition of it. No pattern captures the
; statement: an import's line is its file argument's (the grammar defect below can glue a statement
; to the one before it).

((command
  name: (command_name (word) @_cmd)
  .
  argument: [(word) (raw_string)] @import.module)
 (#match? @_cmd "^\\s*(source|\\.|\\\\\\.)$")
 (#not-match? @import.module "[*?\\[\\]~\\\\$`]"))

((command
  name: (command_name (word) @_cmd)
  .
  argument: (string . (string_content) @import.module .))
 (#match? @_cmd "^\\s*(source|\\.|\\\\\\.)$")
 (#not-match? @import.module "[$`\\\\]"))

((command
  name: (command_name (word) @_via)
  .
  argument: (word) @_cmd
  .
  argument: [(word) (raw_string)] @import.module)
 (#any-of? @_via "builtin" "command")
 (#any-of? @_cmd "source" ".")
 (#not-match? @import.module "[*?\\[\\]~\\\\$`]"))

((command
  name: (command_name (word) @_via)
  .
  argument: (word) @_cmd
  .
  argument: (string . (string_content) @import.module .))
 (#any-of? @_via "builtin" "command")
 (#any-of? @_cmd "source" ".")
 (#not-match? @import.module "[$`\\\\]"))

; tree-sitter-bash v0.25.1 reads a line that opens with a backslash as part of the statement before it
; (no ERROR node): after a command with an argument, `set -ex` then `\. ../../nvm.sh` on a later line
; parse as one command whose third word is the newline, the blank lines and `\.`; after an assignment,
; `X="..."` then `\. ../common.sh` parse as a command named by that word (the first two patterns'
; leading `\s*`). Such a word, ending a line run with `\.`, is the `.` builtin and the word after it
; its file; the spec takes the file's line. A redirect's destination swallows the lines after it the
; same way (`cmd >> "$f"` then `\. x`), and no pattern reads that.
((command
  argument: (word) @_cmd
  .
  argument: [(word) (raw_string)] @import.module)
 (#match? @_cmd "^[ \t]*\n\\s*\\\\\\.$")
 (#not-match? @import.module "[*?\\[\\]~\\\\$`]"))

((command
  argument: (word) @_cmd
  .
  argument: (string . (string_content) @import.module .))
 (#match? @_cmd "^[ \t]*\n\\s*\\\\\\.$")
 (#not-match? @import.module "[$`\\\\]"))
