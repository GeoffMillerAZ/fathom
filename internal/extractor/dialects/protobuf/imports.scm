; source: written for this dialect. `import "a/b.proto";`, `import public "…";` and `import weak "…";` name
; one file by its path under a proto root; the path is the module (unquoted by the engine).
(import path: (string) @import.module) @import.stmt
