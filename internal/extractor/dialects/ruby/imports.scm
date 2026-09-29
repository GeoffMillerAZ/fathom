; dialects/ruby/imports.scm — require, require_relative, load and autoload with a literal path.
; A string with interpolation, or a non-string argument, names no file statically and is left out,
; and a call with a receiver (YAML.load 'a: 1') is not Kernel#load.

((call
  !receiver
  method: (identifier) @_m
  arguments: (argument_list . (string . (string_content) @import.module .))) @import.stmt
 (#any-of? @_m "require" "require_relative" "load"))

((call
  !receiver
  method: (identifier) @_m
  arguments: (argument_list (simple_symbol) . (string . (string_content) @import.module .))) @import.stmt
 (#eq? @_m "autoload"))
