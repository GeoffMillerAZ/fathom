; source: written for this dialect (tree-sitter-grammars/tree-sitter-starlark v1.3.0 ships no tags query). Node
; shapes read with `indexbench lang-probe`, never from grammar.js. A Tiltfile is a script: its definitions are the
; functions it defines and the entities its Tilt API calls declare, each named by a literal string. An entity
; captures its name literal alone, not the call around it, so the call stays owned by the file's module symbol
; (a statement's calls belong to the file, as they do to a def's body): the reference below pairs it with the
; module, not with an entity of its own call.

; def helper(x): ...
(function_definition
  name: (identifier) @name
  body: (block) @body) @definition.function

; local_resource("name", ...), k8s_resource("workload", ...), dc_resource("name", ...), first positional
((call
   function: (identifier) @_fn
   arguments: (argument_list . (string (string_content) @name @definition.resource)))
 (#any-of? @_fn "local_resource" "k8s_resource" "dc_resource"))

; ... or with the name keyword
((call
   function: (identifier) @_fn
   arguments: (argument_list
     (keyword_argument
       name: (identifier) @_k
       value: (string (string_content) @name @definition.resource))))
 (#any-of? @_fn "local_resource" "k8s_resource" "dc_resource")
 (#eq? @_k "name"))

; docker_build("image", context), custom_build("image", command, ...): the image reference is the entity
((call
   function: (identifier) @_fn
   arguments: (argument_list . (string (string_content) @name @definition.image)))
 (#any-of? @_fn "docker_build" "custom_build" "docker_build_with_restart" "custom_build_with_restart"))

; k8s_yaml("deploy.yaml"): the manifest file, when written as one literal path
((call
   function: (identifier) @_fn
   arguments: (argument_list . (string (string_content) @name @definition.manifest)))
 (#eq? @_fn "k8s_yaml"))

; config.define_string("name"), config.define_bool(...), ...: a setting `tilt up -- --name value` reads
((call
   function: (attribute
     object: (identifier) @_o
     attribute: (identifier) @_m)
   arguments: (argument_list . (string (string_content) @name @definition.setting)))
 (#eq? @_o "config")
 (#match? @_m "^define_"))

; Every call, plain or through an object (os.getenv, config.parse): the callee's last name
(call
  function: [
    (identifier) @name
    (attribute attribute: (identifier) @name)
  ]) @reference.call
