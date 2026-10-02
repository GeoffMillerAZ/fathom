; source: written for this dialect (tree-sitter-grammars/tree-sitter-bicep v1.1.0 ships no tags.scm; neither
; aider nor nvim-treesitter carries Bicep tags). Node shapes read with `indexbench lang-probe`, never grammar.js.
;
; Definitions: every top-level declaration (param, var, resource, module, output, type, func, metadata) and a
; resource declared inside its parent's body (a nested child: `resource subnet 'subnets' = {...}`), each named by
; its symbolic name. A body is the value after `=`, so a signature reads `resource vnet '<type>@<api>' =`.
;
; References (@reference.call, L2): a symbolic name used as a value inside a declaration — an object property's
; value, a member access's object (`vnet.id`, `storage.outputs.x`), a `parent::child` access's parent, a subscript's
; object or index, a call's argument, an array item, an interpolation, an operand, a loop's iterable, a direct
; declaration value (`var a = b`), and a type annotation's name (`param p myType`). Not a property key, a member's
; name after `.`, a called function's name (Bicep's builtins), a decorator, a loop or lambda variable's binding.
; A module's params keys are the target file's parameters: each is a qualified reference `<path>.<key>`
; (`./storage.bicep.location`, calls.qualify), which the file resolver binds through the module's import of its path
; (imports.scm) to that file's parameter.

(parameter_declaration (identifier) @name (type) . (_) @body) @definition.param
(parameter_declaration (identifier) @name (type) .) @definition.param
(variable_declaration (identifier) @name . (_) @body) @definition.var
(resource_declaration (identifier) @name (string) . [(object) (if_statement) (for_statement)] @body) @definition.resource
(module_declaration (identifier) @name (string) . [(object) (if_statement) (for_statement)] @body) @definition.module
(output_declaration (identifier) @name (type) . (_) @body) @definition.output
(type_declaration (identifier) @name . (_) @body) @definition.type
(user_defined_function name: (identifier) @name (parameters) @body) @definition.func
(metadata_declaration (identifier) @name . (_) @body) @definition.metadata

; value positions
(object_property . (_) . (identifier) @name @reference.call)
((member_expression object: (identifier) @name @reference.call)
 (#not-any-of? @name "az" "sys"))
(resource_expression object: (identifier) @name @reference.call)
(resource_expression resource: (identifier) @name @reference.call)
(subscript_expression object: (identifier) @name @reference.call)
(subscript_expression index: (identifier) @name @reference.call)
(arguments (identifier) @name @reference.call)
; a call of the file's own function (`prefixed('x')`); Bicep's builtin functions (the CLI's sys and az namespaces,
; v0.47.16) and decorators are not the file's
((call_expression function: (identifier) @name) @reference.call
 (#not-any-of? @name "allowed" "any" "array" "base64" "base64ToJson" "base64ToString" "batchSize" "bool" "buildUri" "cidrHost" "cidrSubnet" "concat" "contains" "dataUri" "dataUriToString" "dateTimeAdd" "dateTimeFromEpoch" "dateTimeToEpoch" "deployer" "deployment" "description" "discriminator" "distinct" "empty" "endsWith" "environment" "export" "extensionResourceId" "externalInput" "fail" "filter" "first" "flatten" "format" "getSecret" "groupBy" "guid" "indexOf" "int" "intersection" "items" "join" "json" "last" "lastIndexOf" "length" "like" "loadDirectoryFileInfo" "loadFileAsBase64" "loadJsonContent" "loadTextContent" "loadYamlContent" "managementGroup" "managementGroupResourceId" "map" "mapValues" "max" "maxLength" "maxValue" "metadata" "min" "minLength" "minValue" "nameof" "newGuid" "objectKeys" "onlyIfNotExists" "padLeft" "parseCidr" "parseUri" "pickZones" "providers" "range" "readCliArg" "readEnvironmentVariable" "reduce" "reference" "replace" "resourceGroup" "resourceId" "retryOn" "roleDefinitions" "sealed" "secure" "shallowMerge" "skip" "sort" "split" "startsWith" "string" "subscription" "subscriptionResourceId" "substring" "take" "tenant" "tenantResourceId" "toLogicalZone" "toLogicalZones" "toLower" "toObject" "toPhysicalZone" "toPhysicalZones" "toUpper" "trim" "tryGet" "union" "uniqueString" "uri" "uriComponent" "uriComponentToString" "utcNow" "waitUntil")
 (#not-match? @name "^list"))
(array (identifier) @name @reference.call)
(interpolation (identifier) @name @reference.call)
(binary_expression left: (identifier) @name @reference.call)
(binary_expression right: (identifier) @name @reference.call)
(unary_expression (identifier) @name @reference.call)
(ternary_expression condition: (identifier) @name @reference.call)
(ternary_expression consequence: (identifier) @name @reference.call)
(ternary_expression alternative: (identifier) @name @reference.call)
(parenthesized_expression (identifier) @name @reference.call)
(for_statement initializer: (identifier) . (identifier) @name @reference.call)
(for_statement body: (identifier) @name @reference.call)
(lambda_expression (_) . (identifier) @name @reference.call)
(for_statement (for_loop_parameters) . (identifier) @name @reference.call)
(variable_declaration (identifier) . (identifier) @name @reference.call)
(output_declaration (identifier) (type) . (identifier) @name @reference.call)
(parameter_declaration (identifier) (type) . (identifier) @name @reference.call)
(type (identifier) @name @reference.call)

; a module's params keys, qualified by the module's path: `./storage.bicep.location`
((module_declaration (identifier) (string (string_content) @call.receiver)
   (object (object_property (identifier) @_params (object (object_property . (identifier) @name))))) @reference.call
 (#eq? @_params "params"))
((module_declaration (identifier) (string (string_content) @call.receiver)
   (if_statement (object (object_property (identifier) @_params (object (object_property . (identifier) @name)))))) @reference.call
 (#eq? @_params "params"))
((module_declaration (identifier) (string (string_content) @call.receiver)
   (for_statement body: (object (object_property (identifier) @_params (object (object_property . (identifier) @name)))))) @reference.call
 (#eq? @_params "params"))
((module_declaration (identifier) (string (string_content) @call.receiver)
   (for_statement body: (if_statement (object (object_property (identifier) @_params (object (object_property . (identifier) @name))))))) @reference.call
 (#eq? @_params "params"))
