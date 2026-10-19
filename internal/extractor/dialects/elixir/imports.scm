; dialects/elixir/imports.scm — alias, import, require and use of a module named by an alias (LB-Q-elixir L3).
; source: written for this dialect from the Elixir reference (Kernel.SpecialForms alias/2, import/2, require/2,
; Kernel.use/2). The grammar parses each as an ordinary call whose first argument is the module's alias node
; (`Foo.Bar` is one node). The manifest's dotted-module strategy maps the dotted name to lib/foo/bar.ex
; (case "snake"). Left out, because no static reader can name the module: `alias __MODULE__.Foo`, `use unquote(m)`,
; `alias Mod.unquote(x)`, a module held in a variable or atom (`import :erlang`).
;
; * `alias Foo.Bar`, `require Foo`, `use Foo` and `use Foo, opts` bring the module in whole.
; * `import Foo`, with or without only:/except:, is a wildcard: its functions are callable unqualified.
; * `alias Foo.Bar, as: B` binds the name the statement chooses.
; * `alias Foo.{A, B.C}` is one import per member (Foo.A, Foo.B.C): two @import.module captures join with ".".

((call
   target: (identifier) @_k
   (arguments . (alias) @import.module .)) @import.stmt
 (#any-of? @_k "alias" "require" "use"))

((call
   target: (identifier) @_k
   (arguments . (alias) @import.module (keywords) .)) @import.stmt
 (#eq? @_k "use"))

((call
   target: (identifier) @_k
   (arguments . (alias) @import.module @import.wildcard .)) @import.stmt
 (#eq? @_k "import"))

((call
   target: (identifier) @_k
   (arguments . (alias) @import.module @import.wildcard (keywords) .)) @import.stmt
 (#eq? @_k "import"))

((call
   target: (identifier) @_k
   (arguments
     .
     (alias) @import.module
     (keywords (pair key: (keyword) @_as value: (alias) @import.alias)) .)) @import.stmt
 (#eq? @_k "alias")
 (#match? @_as "^as:"))

((call
   target: (identifier) @_k
   (arguments
     (dot
       left: (alias) @import.module
       right: (tuple (alias) @import.module)))) @import.stmt
 (#any-of? @_k "alias" "require"))
