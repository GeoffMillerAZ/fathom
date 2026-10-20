; source: written for this dialect (the grammar's own queries/tags.scm names only functions and calls). GENERATED:
; `python3 gen_tags.py > tags.scm` in this directory, do not edit by hand.
;
; The definitions of a Nix file are its root expression's own names: the root is the file's expression peeled
; through at most three wrappers (a function, a let, a with), and its let bindings, the bindings of its attribute
; set (depth one, rec or not) and the formals of its functions are the definitions. A deeper key (config.x.y, a
; nested set, a function's inner let) is not. A dotted key a.b.c = v is the definition `a` (the key at depth one,
; as `nix-instantiate --parse` merges them), written once per binding. Kinds: function (the key's value is a
; function), option (the key is `options`, or its value is mkOption / mkEnableOption / mkPackageOption), package
; (its value is callPackage / mkDerivation / a build* helper applied), else attribute (an attribute set's key)
; or variable (a let binding). A function's formals and its single argument are `param`s. An application's
; head is a call: the identifier, or the selected name as written (lib.mkIf).

; function in root
(source_code expression: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function)))

; function in fn
(source_code expression: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))

; function in let
(source_code expression: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))

; function in with
(source_code expression: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))

; function in fn/fn
(source_code expression: (function_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function)))))

; function in fn/let
(source_code expression: (function_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function)))))

; function in fn/with
(source_code expression: (function_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function)))))

; function in let/fn
(source_code expression: (let_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function)))))

; function in let/let
(source_code expression: (let_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function)))))

; function in let/with
(source_code expression: (let_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function)))))

; function in with/fn
(source_code expression: (with_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function)))))

; function in with/let
(source_code expression: (with_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function)))))

; function in with/with
(source_code expression: (with_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function)))))

; function in fn/fn/fn
(source_code expression: (function_expression body: (function_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in fn/fn/let
(source_code expression: (function_expression body: (function_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in fn/fn/with
(source_code expression: (function_expression body: (function_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in fn/let/fn
(source_code expression: (function_expression body: (let_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in fn/let/let
(source_code expression: (function_expression body: (let_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in fn/let/with
(source_code expression: (function_expression body: (let_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in fn/with/fn
(source_code expression: (function_expression body: (with_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in fn/with/let
(source_code expression: (function_expression body: (with_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in fn/with/with
(source_code expression: (function_expression body: (with_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in let/fn/fn
(source_code expression: (let_expression body: (function_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in let/fn/let
(source_code expression: (let_expression body: (function_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in let/fn/with
(source_code expression: (let_expression body: (function_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in let/let/fn
(source_code expression: (let_expression body: (let_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in let/let/let
(source_code expression: (let_expression body: (let_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in let/let/with
(source_code expression: (let_expression body: (let_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in let/with/fn
(source_code expression: (let_expression body: (with_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in let/with/let
(source_code expression: (let_expression body: (with_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in let/with/with
(source_code expression: (let_expression body: (with_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in with/fn/fn
(source_code expression: (with_expression body: (function_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in with/fn/let
(source_code expression: (with_expression body: (function_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in with/fn/with
(source_code expression: (with_expression body: (function_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in with/let/fn
(source_code expression: (with_expression body: (let_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in with/let/let
(source_code expression: (with_expression body: (let_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in with/let/with
(source_code expression: (with_expression body: (let_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in with/with/fn
(source_code expression: (with_expression body: (with_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in with/with/let
(source_code expression: (with_expression body: (with_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; function in with/with/with
(source_code expression: (with_expression body: (with_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (function_expression body: (_) @body)) @definition.function))))))

; option-value in root
(source_code expression: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in fn
(source_code expression: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in let
(source_code expression: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in with
(source_code expression: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in fn/fn
(source_code expression: (function_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in fn/let
(source_code expression: (function_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in fn/with
(source_code expression: (function_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in let/fn
(source_code expression: (let_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in let/let
(source_code expression: (let_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in let/with
(source_code expression: (let_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in with/fn
(source_code expression: (with_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in with/let
(source_code expression: (with_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in with/with
(source_code expression: (with_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in fn/fn/fn
(source_code expression: (function_expression body: (function_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in fn/fn/let
(source_code expression: (function_expression body: (function_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in fn/fn/with
(source_code expression: (function_expression body: (function_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in fn/let/fn
(source_code expression: (function_expression body: (let_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in fn/let/let
(source_code expression: (function_expression body: (let_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in fn/let/with
(source_code expression: (function_expression body: (let_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in fn/with/fn
(source_code expression: (function_expression body: (with_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in fn/with/let
(source_code expression: (function_expression body: (with_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in fn/with/with
(source_code expression: (function_expression body: (with_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in let/fn/fn
(source_code expression: (let_expression body: (function_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in let/fn/let
(source_code expression: (let_expression body: (function_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in let/fn/with
(source_code expression: (let_expression body: (function_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in let/let/fn
(source_code expression: (let_expression body: (let_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in let/let/let
(source_code expression: (let_expression body: (let_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in let/let/with
(source_code expression: (let_expression body: (let_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in let/with/fn
(source_code expression: (let_expression body: (with_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in let/with/let
(source_code expression: (let_expression body: (with_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in let/with/with
(source_code expression: (let_expression body: (with_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in with/fn/fn
(source_code expression: (with_expression body: (function_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in with/fn/let
(source_code expression: (with_expression body: (function_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in with/fn/with
(source_code expression: (with_expression body: (function_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in with/let/fn
(source_code expression: (with_expression body: (let_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in with/let/let
(source_code expression: (with_expression body: (let_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in with/let/with
(source_code expression: (with_expression body: (let_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in with/with/fn
(source_code expression: (with_expression body: (with_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in with/with/let
(source_code expression: (with_expression body: (with_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-value in with/with/with
(source_code expression: (with_expression body: (with_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.option)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(mkOption|mkEnableOption|mkPackageOption)\\s"))

; option-key in root
(source_code expression: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option))
 (#eq? @name "options"))

; option-key in fn
(source_code expression: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))
 (#eq? @name "options"))

; option-key in let
(source_code expression: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))
 (#eq? @name "options"))

; option-key in with
(source_code expression: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))
 (#eq? @name "options"))

; option-key in fn/fn
(source_code expression: (function_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option))))
 (#eq? @name "options"))

; option-key in fn/let
(source_code expression: (function_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option))))
 (#eq? @name "options"))

; option-key in fn/with
(source_code expression: (function_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option))))
 (#eq? @name "options"))

; option-key in let/fn
(source_code expression: (let_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option))))
 (#eq? @name "options"))

; option-key in let/let
(source_code expression: (let_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option))))
 (#eq? @name "options"))

; option-key in let/with
(source_code expression: (let_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option))))
 (#eq? @name "options"))

; option-key in with/fn
(source_code expression: (with_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option))))
 (#eq? @name "options"))

; option-key in with/let
(source_code expression: (with_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option))))
 (#eq? @name "options"))

; option-key in with/with
(source_code expression: (with_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option))))
 (#eq? @name "options"))

; option-key in fn/fn/fn
(source_code expression: (function_expression body: (function_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in fn/fn/let
(source_code expression: (function_expression body: (function_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in fn/fn/with
(source_code expression: (function_expression body: (function_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in fn/let/fn
(source_code expression: (function_expression body: (let_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in fn/let/let
(source_code expression: (function_expression body: (let_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in fn/let/with
(source_code expression: (function_expression body: (let_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in fn/with/fn
(source_code expression: (function_expression body: (with_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in fn/with/let
(source_code expression: (function_expression body: (with_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in fn/with/with
(source_code expression: (function_expression body: (with_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in let/fn/fn
(source_code expression: (let_expression body: (function_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in let/fn/let
(source_code expression: (let_expression body: (function_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in let/fn/with
(source_code expression: (let_expression body: (function_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in let/let/fn
(source_code expression: (let_expression body: (let_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in let/let/let
(source_code expression: (let_expression body: (let_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in let/let/with
(source_code expression: (let_expression body: (let_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in let/with/fn
(source_code expression: (let_expression body: (with_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in let/with/let
(source_code expression: (let_expression body: (with_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in let/with/with
(source_code expression: (let_expression body: (with_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in with/fn/fn
(source_code expression: (with_expression body: (function_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in with/fn/let
(source_code expression: (with_expression body: (function_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in with/fn/with
(source_code expression: (with_expression body: (function_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in with/let/fn
(source_code expression: (with_expression body: (let_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in with/let/let
(source_code expression: (with_expression body: (let_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in with/let/with
(source_code expression: (with_expression body: (let_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in with/with/fn
(source_code expression: (with_expression body: (with_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in with/with/let
(source_code expression: (with_expression body: (with_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; option-key in with/with/with
(source_code expression: (with_expression body: (with_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.option)))))
 (#eq? @name "options"))

; package in root
(source_code expression: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in fn
(source_code expression: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in let
(source_code expression: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in with
(source_code expression: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in fn/fn
(source_code expression: (function_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in fn/let
(source_code expression: (function_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in fn/with
(source_code expression: (function_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in let/fn
(source_code expression: (let_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in let/let
(source_code expression: (let_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in let/with
(source_code expression: (let_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in with/fn
(source_code expression: (with_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in with/let
(source_code expression: (with_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in with/with
(source_code expression: (with_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in fn/fn/fn
(source_code expression: (function_expression body: (function_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in fn/fn/let
(source_code expression: (function_expression body: (function_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in fn/fn/with
(source_code expression: (function_expression body: (function_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in fn/let/fn
(source_code expression: (function_expression body: (let_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in fn/let/let
(source_code expression: (function_expression body: (let_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in fn/let/with
(source_code expression: (function_expression body: (let_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in fn/with/fn
(source_code expression: (function_expression body: (with_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in fn/with/let
(source_code expression: (function_expression body: (with_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in fn/with/with
(source_code expression: (function_expression body: (with_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in let/fn/fn
(source_code expression: (let_expression body: (function_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in let/fn/let
(source_code expression: (let_expression body: (function_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in let/fn/with
(source_code expression: (let_expression body: (function_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in let/let/fn
(source_code expression: (let_expression body: (let_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in let/let/let
(source_code expression: (let_expression body: (let_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in let/let/with
(source_code expression: (let_expression body: (let_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in let/with/fn
(source_code expression: (let_expression body: (with_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in let/with/let
(source_code expression: (let_expression body: (with_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in let/with/with
(source_code expression: (let_expression body: (with_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in with/fn/fn
(source_code expression: (with_expression body: (function_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in with/fn/let
(source_code expression: (with_expression body: (function_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in with/fn/with
(source_code expression: (with_expression body: (function_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in with/let/fn
(source_code expression: (with_expression body: (let_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in with/let/let
(source_code expression: (with_expression body: (let_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in with/let/with
(source_code expression: (with_expression body: (let_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in with/with/fn
(source_code expression: (with_expression body: (with_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in with/with/let
(source_code expression: (with_expression body: (with_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; package in with/with/with
(source_code expression: (with_expression body: (with_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name .) expression: (apply_expression) @_v @body) @definition.package)))))
 (#match? @_v "^([A-Za-z0-9_'-]+[.])*(callPackage|mkDerivation|buildPythonPackage|buildPythonApplication|buildGoModule|buildRustPackage|buildNpmPackage|writeShellApplication|writeShellScriptBin)\\s"))

; variable in let
(source_code expression: (let_expression (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.variable)))

; variable in fn/let
(source_code expression: (function_expression body: (let_expression (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.variable))))

; variable in let/let
(source_code expression: (let_expression body: (let_expression (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.variable))))

; variable in with/let
(source_code expression: (with_expression body: (let_expression (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.variable))))

; variable in fn/fn/let
(source_code expression: (function_expression body: (function_expression body: (let_expression (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.variable)))))

; variable in fn/let/let
(source_code expression: (function_expression body: (let_expression body: (let_expression (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.variable)))))

; variable in fn/with/let
(source_code expression: (function_expression body: (with_expression body: (let_expression (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.variable)))))

; variable in let/fn/let
(source_code expression: (let_expression body: (function_expression body: (let_expression (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.variable)))))

; variable in let/let/let
(source_code expression: (let_expression body: (let_expression body: (let_expression (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.variable)))))

; variable in let/with/let
(source_code expression: (let_expression body: (with_expression body: (let_expression (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.variable)))))

; variable in with/fn/let
(source_code expression: (with_expression body: (function_expression body: (let_expression (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.variable)))))

; variable in with/let/let
(source_code expression: (with_expression body: (let_expression body: (let_expression (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.variable)))))

; variable in with/with/let
(source_code expression: (with_expression body: (with_expression body: (let_expression (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.variable)))))

; attribute in root
(source_code expression: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute)))

; attribute in fn
(source_code expression: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))

; attribute in let
(source_code expression: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))

; attribute in with
(source_code expression: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))

; attribute in fn/fn
(source_code expression: (function_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute)))))

; attribute in fn/let
(source_code expression: (function_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute)))))

; attribute in fn/with
(source_code expression: (function_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute)))))

; attribute in let/fn
(source_code expression: (let_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute)))))

; attribute in let/let
(source_code expression: (let_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute)))))

; attribute in let/with
(source_code expression: (let_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute)))))

; attribute in with/fn
(source_code expression: (with_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute)))))

; attribute in with/let
(source_code expression: (with_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute)))))

; attribute in with/with
(source_code expression: (with_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute)))))

; attribute in fn/fn/fn
(source_code expression: (function_expression body: (function_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in fn/fn/let
(source_code expression: (function_expression body: (function_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in fn/fn/with
(source_code expression: (function_expression body: (function_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in fn/let/fn
(source_code expression: (function_expression body: (let_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in fn/let/let
(source_code expression: (function_expression body: (let_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in fn/let/with
(source_code expression: (function_expression body: (let_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in fn/with/fn
(source_code expression: (function_expression body: (with_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in fn/with/let
(source_code expression: (function_expression body: (with_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in fn/with/with
(source_code expression: (function_expression body: (with_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in let/fn/fn
(source_code expression: (let_expression body: (function_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in let/fn/let
(source_code expression: (let_expression body: (function_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in let/fn/with
(source_code expression: (let_expression body: (function_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in let/let/fn
(source_code expression: (let_expression body: (let_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in let/let/let
(source_code expression: (let_expression body: (let_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in let/let/with
(source_code expression: (let_expression body: (let_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in let/with/fn
(source_code expression: (let_expression body: (with_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in let/with/let
(source_code expression: (let_expression body: (with_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in let/with/with
(source_code expression: (let_expression body: (with_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in with/fn/fn
(source_code expression: (with_expression body: (function_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in with/fn/let
(source_code expression: (with_expression body: (function_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in with/fn/with
(source_code expression: (with_expression body: (function_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in with/let/fn
(source_code expression: (with_expression body: (let_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in with/let/let
(source_code expression: (with_expression body: (let_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in with/let/with
(source_code expression: (with_expression body: (let_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in with/with/fn
(source_code expression: (with_expression body: (with_expression body: (function_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in with/with/let
(source_code expression: (with_expression body: (with_expression body: (let_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; attribute in with/with/with
(source_code expression: (with_expression body: (with_expression body: (with_expression body: (_ (binding_set binding: (binding attrpath: (attrpath . attr: (identifier) @name) expression: (_) @body) @definition.attribute))))))

; param in fn
(source_code expression: (function_expression formals: (formals formal: (formal name: (identifier) @name) @definition.param)))

; param in fn
(source_code expression: (function_expression universal: (identifier) @name @definition.param))

; param in fn/fn
(source_code expression: (function_expression body: (function_expression formals: (formals formal: (formal name: (identifier) @name) @definition.param))))

; param in fn/fn
(source_code expression: (function_expression body: (function_expression universal: (identifier) @name @definition.param)))

; param in let/fn
(source_code expression: (let_expression body: (function_expression formals: (formals formal: (formal name: (identifier) @name) @definition.param))))

; param in let/fn
(source_code expression: (let_expression body: (function_expression universal: (identifier) @name @definition.param)))

; param in with/fn
(source_code expression: (with_expression body: (function_expression formals: (formals formal: (formal name: (identifier) @name) @definition.param))))

; param in with/fn
(source_code expression: (with_expression body: (function_expression universal: (identifier) @name @definition.param)))

; param in fn/fn/fn
(source_code expression: (function_expression body: (function_expression body: (function_expression formals: (formals formal: (formal name: (identifier) @name) @definition.param)))))

; param in fn/fn/fn
(source_code expression: (function_expression body: (function_expression body: (function_expression universal: (identifier) @name @definition.param))))

; param in fn/let/fn
(source_code expression: (function_expression body: (let_expression body: (function_expression formals: (formals formal: (formal name: (identifier) @name) @definition.param)))))

; param in fn/let/fn
(source_code expression: (function_expression body: (let_expression body: (function_expression universal: (identifier) @name @definition.param))))

; param in fn/with/fn
(source_code expression: (function_expression body: (with_expression body: (function_expression formals: (formals formal: (formal name: (identifier) @name) @definition.param)))))

; param in fn/with/fn
(source_code expression: (function_expression body: (with_expression body: (function_expression universal: (identifier) @name @definition.param))))

; param in let/fn/fn
(source_code expression: (let_expression body: (function_expression body: (function_expression formals: (formals formal: (formal name: (identifier) @name) @definition.param)))))

; param in let/fn/fn
(source_code expression: (let_expression body: (function_expression body: (function_expression universal: (identifier) @name @definition.param))))

; param in let/let/fn
(source_code expression: (let_expression body: (let_expression body: (function_expression formals: (formals formal: (formal name: (identifier) @name) @definition.param)))))

; param in let/let/fn
(source_code expression: (let_expression body: (let_expression body: (function_expression universal: (identifier) @name @definition.param))))

; param in let/with/fn
(source_code expression: (let_expression body: (with_expression body: (function_expression formals: (formals formal: (formal name: (identifier) @name) @definition.param)))))

; param in let/with/fn
(source_code expression: (let_expression body: (with_expression body: (function_expression universal: (identifier) @name @definition.param))))

; param in with/fn/fn
(source_code expression: (with_expression body: (function_expression body: (function_expression formals: (formals formal: (formal name: (identifier) @name) @definition.param)))))

; param in with/fn/fn
(source_code expression: (with_expression body: (function_expression body: (function_expression universal: (identifier) @name @definition.param))))

; param in with/let/fn
(source_code expression: (with_expression body: (let_expression body: (function_expression formals: (formals formal: (formal name: (identifier) @name) @definition.param)))))

; param in with/let/fn
(source_code expression: (with_expression body: (let_expression body: (function_expression universal: (identifier) @name @definition.param))))

; param in with/with/fn
(source_code expression: (with_expression body: (with_expression body: (function_expression formals: (formals formal: (formal name: (identifier) @name) @definition.param)))))

; param in with/with/fn
(source_code expression: (with_expression body: (with_expression body: (function_expression universal: (identifier) @name @definition.param))))

; calls: the head of an application, the identifier or the last attribute of the selected name
(apply_expression function: (variable_expression name: (identifier) @name)) @reference.call

(apply_expression function: (select_expression) @name) @reference.call
