; source: written for this dialect. load("//pkg:file.bzl", "sym", alias = "other") binds each listed symbol (a
; keyword argument binds the symbol its string names under the keyword's name). The module label //pkg:file.bzl and
; :file.bzl name a file of this repository; the manifest's file-symbol strategy reads them as Bazel labels. A label
; into another repository (@repo//pkg:f.bzl) resolves to nothing.
((module
   (expression_statement
     (call
       function: (identifier) @_fn
       arguments: (argument_list
         . (string (string_content) @import.module)
         (string (string_content) @import.name))) @import.stmt))
 (#eq? @_fn "load"))

((module
   (expression_statement
     (call
       function: (identifier) @_fn
       arguments: (argument_list
         . (string (string_content) @import.module)
         (keyword_argument
           name: (identifier) @import.alias
           value: (string (string_content) @import.name)))) @import.stmt))
 (#eq? @_fn "load"))
