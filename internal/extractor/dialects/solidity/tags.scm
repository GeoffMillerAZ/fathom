; source: JoranHonig/tree-sitter-solidity queries/tags.scm @ bf268722bc6406429776f71f012e6d1a88801931 (MIT)
; (tag v1.2.9), rewritten: the upstream file tags a contract, struct, enum and event as `class` and
; every library as an `interface` and reads every call by its expression text. This file names each
; Solidity declaration by its own kind, anchors a function by its body, and reads a call by the
; callee's last identifier.

; Contract, interface and library.
(contract_declaration
  name: (identifier) @name) @definition.contract

(interface_declaration
  name: (identifier) @name) @definition.interface

(library_declaration
  name: (identifier) @name) @definition.library

(struct_declaration
  name: (identifier) @name) @definition.struct

(enum_declaration
  name: (identifier) @name) @definition.enum

(user_defined_type_definition
  name: (identifier) @name) @definition.type

(event_definition
  name: (identifier) @name) @definition.event

(error_declaration
  name: (identifier) @name) @definition.error

(constant_variable_declaration
  name: (identifier) @name) @definition.constant

(state_variable_declaration
  name: (identifier) @name) @definition.variable

; Functions: a free function, a member of a contract, library or interface, a modifier.
(source_file
  (function_definition
    name: (identifier) @name
    body: (function_body) @body) @definition.function)

(source_file
  (function_definition
    name: (identifier) @name
    !body) @definition.function)

(contract_body
  (function_definition
    name: (identifier) @name
    body: (function_body) @body) @definition.method)

(contract_body
  (function_definition
    name: (identifier) @name
    !body) @definition.method)

(modifier_definition
  name: (identifier) @name
  body: (function_body) @body) @definition.modifier

(modifier_definition
  name: (identifier) @name
  !body) @definition.modifier

; A constructor, a receive function and a fallback function have no name: the keyword is it.
(contract_body
  (constructor_definition
    "constructor" @name
    body: (function_body) @body) @definition.constructor)

(contract_body
  (fallback_receive_definition
    [ "receive" "fallback" ] @name
    body: (function_body) @body) @definition.constructor)

(contract_body
  (fallback_receive_definition
    [ "receive" "fallback" ] @name
    !body) @definition.constructor)

; Calls.
; A global function is no call of a declaration in a file, and a callee that starts with a capital
; is a type conversion or a struct constructor (a custom error is raised by revert or require's
; own form, read below).
(call_expression
  function: (expression (identifier) @name)
  (#not-any-of? @name
    "require" "assert" "revert" "keccak256" "sha256" "ripemd160" "ecrecover" "addmod" "mulmod"
    "selfdestruct" "blockhash" "gasleft" "blobhash" "payable")
  (#not-match? @name "^[A-Z]")) @reference.call

; abi.encode and the like are builtins of the abi object, and these members of an address, an
; array, a bytes or string and a user-defined value type are builtins too.
(call_expression
  function: (expression
    (member_expression
      object: (_) @_obj
      property: (identifier) @name))
  (#not-eq? @_obj "abi")
  (#not-any-of? @name "push" "pop" "concat" "call" "staticcall" "delegatecall" "send" "wrap" "unwrap")) @reference.call

; require(cond, CustomError()) raises an error declared in the file's closure.
(call_expression
  function: (expression (identifier) @_req)
  (call_argument
    (expression
      (call_expression
        function: (expression (identifier) @name)))) @reference.call
  (#eq? @_req "require")
  (#match? @name "^[A-Z]"))

(emit_statement
  name: (expression (identifier) @name)) @reference.call

(emit_statement
  name: (expression
    (member_expression
      property: (identifier) @name))) @reference.call

(revert_statement
  error: (expression (identifier) @name)) @reference.call

(revert_statement
  error: (expression
    (member_expression
      property: (identifier) @name))) @reference.call

; A modifier invocation; in a constructor a capitalised one is a base constructor call instead.
(function_definition
  (modifier_invocation
    (identifier) @name) @reference.call)

(constructor_definition
  (modifier_invocation
    (identifier) @name) @reference.call
  (#not-match? @name "^[A-Z]"))
