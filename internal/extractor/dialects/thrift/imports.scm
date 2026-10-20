; source: written for this dialect. `include "shared.thrift"` names one file, relative to the including file's directory
; first and then to the include path (the repository root here); the path is the module (unquoted by the engine).
; `cpp_include "x.h"` parses as the same node and names a C++ header, so it is no import.
((include_statement (string) @import.module) @import.stmt
  (#match? @import.stmt "^include"))
