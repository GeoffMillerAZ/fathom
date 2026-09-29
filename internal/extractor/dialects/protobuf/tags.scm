; source: written for this dialect (coder3101/tree-sitter-proto 0.6.0 ships no tags.scm; neither aider nor
; nvim-treesitter carries protobuf tags). Node shapes read with `indexbench lang-probe`, never grammar.js.
; Kinds are the incumbent protoextract.go's four (ProtoMessage, ProtoEnum, ProtoService, ProtoRPC). A message
; and an enum nested in a message are named by their parents (Outer.Inner, the protobuf full name inside the
; file); an rpc is named by its service (Greeter.SayHello). A message owns the type references its fields,
; map values and oneof members make; an rpc owns its request and response types. A scalar type (string,
; int32) is not a reference; an option's name, an extend's target and a reserved name are not either.

(message
  (message_name (identifier) @name)
  (message_body) @body) @definition.message

; A proto2 group declares a nested message in place (protoc makes it one, named by the group).
(group
  (message_name (identifier) @name)
  (message_body) @body) @definition.message

(enum
  (enum_name (identifier) @name)
  (enum_body) @body) @definition.enum

(service
  (service_name (identifier) @name)) @definition.service

; An rpc ends at `;` or at the `{ option ... }` block it may carry; the signature runs to either.
(rpc
  (rpc_name (identifier) @name)
  ["{" ";"] @body) @definition.rpc

; A type reference (a field's, a map value's, a oneof member's, an rpc's request or response, an extension
; field's) is named by its last identifier: `acme.v1.Other` and `.acme.v1.Other` are Other. A trailing `.`
; anchor does not select the last identifier in this grammar's node (every identifier matches), so each
; pattern fixes the path's length with #match? on the whole reference and walks to the last identifier
; with leading and sibling anchors. Paths of one to eight segments; a longer one is not captured.
((message_or_enum_type . (identifier) @name) @reference.call
  (#match? @reference.call "^\\.?\\s*[A-Za-z_][A-Za-z0-9_]*$"))
((message_or_enum_type . (identifier) . (identifier) @name) @reference.call
  (#match? @reference.call "^\\.?(\\s*[A-Za-z_][A-Za-z0-9_]*\\s*\\.){1}\\s*[A-Za-z_][A-Za-z0-9_]*$"))
((message_or_enum_type . (identifier) . (identifier) . (identifier) @name) @reference.call
  (#match? @reference.call "^\\.?(\\s*[A-Za-z_][A-Za-z0-9_]*\\s*\\.){2}\\s*[A-Za-z_][A-Za-z0-9_]*$"))
((message_or_enum_type . (identifier) . (identifier) . (identifier) . (identifier) @name) @reference.call
  (#match? @reference.call "^\\.?(\\s*[A-Za-z_][A-Za-z0-9_]*\\s*\\.){3}\\s*[A-Za-z_][A-Za-z0-9_]*$"))
((message_or_enum_type . (identifier) . (identifier) . (identifier) . (identifier) . (identifier) @name) @reference.call
  (#match? @reference.call "^\\.?(\\s*[A-Za-z_][A-Za-z0-9_]*\\s*\\.){4}\\s*[A-Za-z_][A-Za-z0-9_]*$"))
((message_or_enum_type . (identifier) . (identifier) . (identifier) . (identifier) . (identifier) . (identifier) @name) @reference.call
  (#match? @reference.call "^\\.?(\\s*[A-Za-z_][A-Za-z0-9_]*\\s*\\.){5}\\s*[A-Za-z_][A-Za-z0-9_]*$"))
((message_or_enum_type . (identifier) . (identifier) . (identifier) . (identifier) . (identifier) . (identifier) . (identifier) @name) @reference.call
  (#match? @reference.call "^\\.?(\\s*[A-Za-z_][A-Za-z0-9_]*\\s*\\.){6}\\s*[A-Za-z_][A-Za-z0-9_]*$"))
((message_or_enum_type . (identifier) . (identifier) . (identifier) . (identifier) . (identifier) . (identifier) . (identifier) . (identifier) @name) @reference.call
  (#match? @reference.call "^\\.?(\\s*[A-Za-z_][A-Za-z0-9_]*\\s*\\.){7}\\s*[A-Za-z_][A-Za-z0-9_]*$"))
