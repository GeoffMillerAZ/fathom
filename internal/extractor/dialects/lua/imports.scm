; dialects/lua/imports.scm — require with a literal module name (LB-Q-lua L3, wave lang1).
; source: written for this dialect. Lua 5.4 manual 6.3: `require (modname)` searches package.loaded and then
; package.path with the dots of modname read as directory separators. A literal is `require("a.b")`,
; `require "a.b"`, `require 'a.b'` and `require [[a.b]]`; `pcall(require, "a.b")`, the guarded form every
; optional dependency uses, names the same module. A string with an escape or a concatenation
; (`require("pkg." .. name)`), a variable and a call made by any other name name no module statically and are
; left out (the oracle counts them). No pattern binds an alias: whichever way the call's value is kept
; (`local x = require "m"`, `M.x = require "m"`, `require("m").f()`), the statement brings the module in.
; The manifest's file-symbol strategy resolves the name (design section 6).

((function_call
   name: (identifier) @_f
   arguments: (arguments . (string . content: (string_content) @import.module .) .))
 (#eq? @_f "require"))

((function_call
   name: (identifier) @_p
   arguments: (arguments . (identifier) @_f . (string . content: (string_content) @import.module .) .))
 (#eq? @_p "pcall")
 (#eq? @_f "require"))
