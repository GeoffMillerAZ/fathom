; source: written for this dialect. A module's path names its file (`./storage.bicep`, `../x.json`; a `br:`, `br/`
; or `ts:` registry reference resolves to no file of the repository): the path itself binds the file as a whole, so
; the module's params keys, stored qualified by the path (`./storage.bicep.location`), reach that file's parameters,
; and a use of the module's symbolic name elsewhere binds nothing of the target. A compile-time import
; names its file too: `import {a, b as c} from './types.bicep'` binds each name, `import * as t from ...` the
; file. A .bicepparam's `using './main.bicep'` names its template (no name is bound).
(module_declaration (identifier) (string (string_content) @import.alias) @import.module)

(import_functionality ["{" ","] . (identifier) @import.name . ["," "}"] (string) @import.module)
(import_functionality ["{" ","] . (identifier) @import.name . "as" . (identifier) @import.alias (string) @import.module)
(import_functionality "*" . "as" . (identifier) @import.alias . "from" . (string) @import.module)

(using_statement (string) @import.module)
