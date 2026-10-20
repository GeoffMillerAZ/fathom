; source: written for this dialect. A Nix path literal names a file: `import ./x.nix`, `callPackage ./x.nix { }`,
; an element of `imports = [ ./a.nix ];` and every other `./` or `../` literal that is plain text (a literal with
; an interpolation, `./${name}.nix`, names no file statically and is left out). The literal is taken when it can
; name a Nix file: it ends in `.nix`, or in a directory (no dot in its last segment), which Nix reads as the
; directory's default.nix; `./logo.png` and `./config.toml` are data, not imports. The manifest's relative-path
; strategy resolves it against the importing file's directory (a directory to its default.nix, then its
; flake.nix). A path binds names only where Nix binds them:
;   u = import ./u.nix;            u is the file (u.f 1 calls f of ./u.nix)
;   inherit (import ./u.nix) f g;  f and g are names of ./u.nix
; a bare path (callPackage ./x.nix, an imports element, `import ./x.nix` used inline) is the file as a whole and
; binds no name: it is a derivation or a module, not a set of functions in scope. A flake input whose `url` is a
; plain relative path (`inputs.local.url = "./sub"`, or `local.url` inside `inputs = { }`) names its directory the
; same way. The `path:` form is not read: a capture cannot drop the scheme, so `url = "path:./sub"` names nothing.

; the path literal as a whole
((path_expression . (path_fragment) @import.module .)
 (#match? @import.module "^[.][.]?/([^/]*/)*([^/.]*|[.][.]?|[^/]*[.]nix)$"))

; u = import ./u.nix;  u = import ./u.nix { };
((binding
   attrpath: (attrpath . attr: (identifier) @import.alias .)
   expression: [
     (apply_expression
       function: (variable_expression name: (identifier) @_i)
       argument: (path_expression . (path_fragment) @import.module .))
     (apply_expression
       function: (apply_expression
         function: (variable_expression name: (identifier) @_i)
         argument: (path_expression . (path_fragment) @import.module .)))
   ])
 (#eq? @_i "import")
 (#match? @import.module "^[.][.]?/([^/]*/)*([^/.]*|[.][.]?|[^/]*[.]nix)$"))

; inherit (import ./u.nix) f g;  inherit ((import ./u.nix { })) f;
((inherit_from
   expression: [
     (apply_expression
       function: (variable_expression name: (identifier) @_i)
       argument: (path_expression . (path_fragment) @import.module .))
     (apply_expression
       function: (apply_expression
         function: (variable_expression name: (identifier) @_i)
         argument: (path_expression . (path_fragment) @import.module .)))
     (parenthesized_expression
       [
         (apply_expression
           function: (variable_expression name: (identifier) @_i)
           argument: (path_expression . (path_fragment) @import.module .))
         (apply_expression
           function: (apply_expression
             function: (variable_expression name: (identifier) @_i)
             argument: (path_expression . (path_fragment) @import.module .)))
       ])
   ]
   attrs: (inherited_attrs attr: (identifier) @import.name))
 (#eq? @_i "import")
 (#match? @import.module "^[.][.]?/([^/]*/)*([^/.]*|[.][.]?|[^/]*[.]nix)$"))

; inputs.local.url = "./sub";
((binding
   attrpath: (attrpath attr: (identifier) @_k .)
   expression: (string_expression . (string_fragment) @import.module .))
 (#eq? @_k "url")
 (#match? @import.module "^[.][.]?/([^/]*/)*[^/.]*$"))
