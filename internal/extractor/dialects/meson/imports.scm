; source: written for this dialect. subdir('dir') runs dir/meson.build next, in the same variable namespace,
; so the names it defines are in scope after the call: a whole-file import of that file, and a wildcard.
((normal_command
   command: (identifier) @_fn
   . (variableunit . (string) @import.module .)) @import.stmt @import.wildcard
 (#eq? @_fn "subdir"))
