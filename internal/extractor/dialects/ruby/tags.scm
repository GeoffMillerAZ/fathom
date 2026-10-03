; dialects/ruby/tags.scm — symbol + call queries for the wazero tree-sitter engine
; (internal.extractor.NewTSExtractor("ruby"), ADR-021 §4; LB-Q-ruby).

(module
  name: (_) @name) @definition.module

(class
  name: (_) @name) @definition.class

(method
  name: (_) @name
  body: (_)? @body) @definition.method

; def self.x and def Const.x. A singleton method on a local (`def obj.x`, `def block.each`) is
; left out: it has no owner name to qualify it by, and a bare `each` would read as a
; top-level method to the file resolver.
(singleton_method
  object: [(self) (constant) (scope_resolution)]
  name: (_) @name
  body: (_)? @body) @definition.method

; alias new old / alias_method :new, :old — the new name is the definition.
(alias
  name: (_) @name) @definition.alias

((call
  method: (identifier) @_m
  arguments: (argument_list . (simple_symbol) @name)) @definition.alias
 (#eq? @_m "alias_method"))

; attr_accessor :a, :b — one property per symbol argument. The definition is the symbol
; itself, not the call: N names sharing one node would nest in one another and lose their
; owner's qualification.
((call
  method: (identifier) @_m
  arguments: (argument_list (simple_symbol) @name @definition.property))
 (#any-of? @_m "attr_accessor" "attr_reader" "attr_writer"))

; define_method(:name) { ... } and define_method("name") { ... }
((call
  method: (identifier) @_m
  arguments: (argument_list . [(simple_symbol) (string)] @name)
  block: (_)? @body) @definition.method
 (#eq? @_m "define_method"))

; CONST = value
((assignment
  left: (constant) @name
  right: (_) @_rhs) @definition.constant
 (#not-match? @_rhs "^(Class|Module)\\.new"))

; Name = Class.new(...) / Module.new, and the same bound to a local (`app = Class.new`): the
; methods its block defines qualify under it.
((assignment
  left: [(constant) (identifier)] @name
  right: (call
    receiver: (constant) @_r
    method: (identifier) @_n)) @definition.class
 (#eq? @_r "Class")
 (#eq? @_n "new"))

((assignment
  left: [(constant) (identifier)] @name
  right: (call
    receiver: (constant) @_r
    method: (identifier) @_n)) @definition.module
 (#eq? @_r "Module")
 (#eq? @_n "new"))

; Calls: the method name of every call node, and every constant reference.
((call
  method: (_) @reference.call)
 (#not-any-of? @reference.call "alias_method" "attr_accessor" "attr_reader" "attr_writer" "define_method"))

(constant) @reference.call

; A bare identifier (`helper`: no receiver, no parentheses, no arguments) is a call unless it reads a
; local variable. The engine's locals rule (manifest queries.locals, locals.scm; LB-F254) keeps it
; only where no local of that name is in scope (the pattern's `(#set! is-not "local")`), as upstream's
; `[(identifier) (constant)] @name @reference.call (#is-not? local)` does; it leaves out the names
; upstream leaves out and the definers the call pattern above leaves out. A name another pattern
; takes (a method's own name, a call's method name, an @ignore below) is no bare call of its own.
((identifier) @reference.call
 (#not-any-of? @reference.call "lambda" "load" "require" "require_relative" "__FILE__" "__LINE__"
   "alias_method" "attr_accessor" "attr_reader" "attr_writer" "define_method")
 (#set! is-not "local"))

; Neither a call nor a local read: a setter's name (`def x=`), and a singleton method's name and
; object on a local (`def obj.x`, which this query does not define: the object is the local it is
; defined on, read in the scope around the def, not inside it).
(setter (identifier) @ignore)
(singleton_method name: (identifier) @ignore)
(singleton_method object: (identifier) @ignore)
