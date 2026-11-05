; A sample pack's tags query over the thrift grammar fathom embeds: structs,
; services and their functions.
(struct_definition type: (identifier) @name) @definition.struct
(service_definition . type: (identifier) @name) @definition.service
(function_definition (identifier) @name [";" ","]? @body) @definition.function
