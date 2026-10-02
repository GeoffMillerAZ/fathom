; source: written for this dialect. add_subdirectory(dir) names the directory whose CMakeLists.txt CMake runs
; next; include(file) names a .cmake file (or a module, `include(Foo)`, read as Foo.cmake beside the includer).
; Both are whole-file imports that bring the target's top-level names into scope (the target and function
; names a later command refers to), so each is also an @import.wildcard. A spelling that holds a variable
; (`${CMAKE_CURRENT_LIST_DIR}/x.cmake`) names no file without CMake's evaluation and is not an import.
((normal_command
   (identifier) @_cmd
   (argument_list . (argument [(unquoted_argument) @import.module (quoted_argument (quoted_element) @import.module)]))) @import.stmt @import.wildcard
 (#match? @_cmd "^(?i:add_subdirectory|include)$")
 (#not-match? @import.module "\\$"))
