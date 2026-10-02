; source: written for this dialect (uyha/tree-sitter-cmake v0.7.5 ships queries/ with highlights, folds and
; injections only; neither aider nor nvim-treesitter carries CMake tags). Node shapes read with
; `indexbench lang-probe`, never from grammar.js. A CMake file is a sequence of commands, so every definition is
; one command invocation named by its first argument; the command's own name is matched case-insensitively
; (CMake treats ADD_EXECUTABLE and add_executable alike).

; project(Name ...)
((normal_command
   (identifier) @_cmd
   (argument_list . (argument [(unquoted_argument) @name (quoted_argument (quoted_element) @name)]))) @definition.project
 (#not-match? @name "\\$")
 (#match? @_cmd "^(?i:project)$"))

; add_executable / add_library (STATIC, SHARED, OBJECT, INTERFACE, IMPORTED, ALIAS) / add_custom_target
((normal_command
   (identifier) @_cmd
   (argument_list . (argument [(unquoted_argument) @name (quoted_argument (quoted_element) @name)]))) @definition.target
 (#not-match? @name "\\$")
 (#match? @_cmd "^(?i:add_executable|add_library|add_custom_target)$"))

; function(name ...) and macro(name ...): the definition runs to endfunction() / endmacro()
(function_def
  (function_command
    (argument_list . (argument [(unquoted_argument) @name (quoted_argument (quoted_element) @name)])))
  (body) @body) @definition.function

(macro_def
  (macro_command
    (argument_list . (argument [(unquoted_argument) @name (quoted_argument (quoted_element) @name)])))
  (body) @body) @definition.function

; option(NAME "doc" [value])
((normal_command
   (identifier) @_cmd
   (argument_list . (argument [(unquoted_argument) @name (quoted_argument (quoted_element) @name)]))) @definition.option
 (#not-match? @name "\\$")
 (#match? @_cmd "^(?i:option)$"))

; set(NAME value... CACHE TYPE "doc"): only a cache entry is a configuration surface
((normal_command
   (identifier) @_cmd
   (argument_list . (argument [(unquoted_argument) @name (quoted_argument (quoted_element) @name)])
                  (argument (unquoted_argument) @_cache))) @definition.variable
 (#not-match? @name "\\$")
 (#match? @_cmd "^(?i:set)$")
 (#eq? @_cache "CACHE"))

; find_package(Pkg ...)
((normal_command
   (identifier) @_cmd
   (argument_list . (argument [(unquoted_argument) @name (quoted_argument (quoted_element) @name)]))) @definition.dependency
 (#not-match? @name "\\$")
 (#match? @_cmd "^(?i:find_package)$"))

; target_link_libraries(tgt [PRIVATE|PUBLIC|INTERFACE] lib ...) and add_dependencies(tgt dep ...): every
; argument after the first that is a plain target spelling is a reference
((normal_command
   (identifier) @_cmd
   (argument_list . (argument) (argument (unquoted_argument) @name) @reference.call))
 (#match? @_cmd "^(?i:target_link_libraries|add_dependencies)$")
 (#not-any-of? @name "PRIVATE" "PUBLIC" "INTERFACE" "LINK_PRIVATE" "LINK_PUBLIC" "LINK_INTERFACE_LIBRARIES" "debug" "optimized" "general")
 (#match? @name "^[A-Za-z_][A-Za-z0-9_.:+-]*$"))

; The link and dependency commands are sibling commands of the target's own, not inside it, so a reference above
; has no definition enclosing it. These attach every reference of a target_link_libraries or add_dependencies
; command to the add_executable/add_library/add_custom_target definition of the same name that precedes it as a
; sibling in the same file or block, with at most four commands (target_include_directories,
; set_target_properties, ...) between them (the engine's @calls.to/@calls.from attachment, LB-F65): across the
; 481 literally named targets of the calibration subjects and their neighbours, 61 are followed by a link command
; of their own in the same block, 57 of them within two commands and none beyond four. One pattern per gap,
; because a pattern that pairs every target with every later link command in the file is quadratic and the
; engine's scan budget fails the file at a few hundred targets; each pattern costs about 10% of the parse. The
; equal-name predicate runs first, as it rejects nearly every candidate pair before a regex is evaluated. A link
; written in another directory's file, more than four commands further down, or inside an if() block the
; target's command is not a direct child of attaches to nothing (cmake.json's disposition counts them).
((_
   (normal_command
     (identifier) @_def
     (argument_list . (argument (unquoted_argument) @_tgt))) @calls.to
   . 
   (normal_command
     (identifier) @_link
     (argument_list . (argument (unquoted_argument) @_tgt2))) @calls.from)
 (#eq? @_tgt @_tgt2)
 (#match? @_def "^(?i:add_executable|add_library|add_custom_target)$")
 (#match? @_link "^(?i:target_link_libraries|add_dependencies)$"))
((_
   (normal_command
     (identifier) @_def
     (argument_list . (argument (unquoted_argument) @_tgt))) @calls.to
   . (_) .
   (normal_command
     (identifier) @_link
     (argument_list . (argument (unquoted_argument) @_tgt2))) @calls.from)
 (#eq? @_tgt @_tgt2)
 (#match? @_def "^(?i:add_executable|add_library|add_custom_target)$")
 (#match? @_link "^(?i:target_link_libraries|add_dependencies)$"))
((_
   (normal_command
     (identifier) @_def
     (argument_list . (argument (unquoted_argument) @_tgt))) @calls.to
   . (_) . (_) .
   (normal_command
     (identifier) @_link
     (argument_list . (argument (unquoted_argument) @_tgt2))) @calls.from)
 (#eq? @_tgt @_tgt2)
 (#match? @_def "^(?i:add_executable|add_library|add_custom_target)$")
 (#match? @_link "^(?i:target_link_libraries|add_dependencies)$"))
((_
   (normal_command
     (identifier) @_def
     (argument_list . (argument (unquoted_argument) @_tgt))) @calls.to
   . (_) . (_) . (_) .
   (normal_command
     (identifier) @_link
     (argument_list . (argument (unquoted_argument) @_tgt2))) @calls.from)
 (#eq? @_tgt @_tgt2)
 (#match? @_def "^(?i:add_executable|add_library|add_custom_target)$")
 (#match? @_link "^(?i:target_link_libraries|add_dependencies)$"))
((_
   (normal_command
     (identifier) @_def
     (argument_list . (argument (unquoted_argument) @_tgt))) @calls.to
   . (_) . (_) . (_) . (_) .
   (normal_command
     (identifier) @_link
     (argument_list . (argument (unquoted_argument) @_tgt2))) @calls.from)
 (#eq? @_tgt @_tgt2)
 (#match? @_def "^(?i:add_executable|add_library|add_custom_target)$")
 (#match? @_link "^(?i:target_link_libraries|add_dependencies)$"))
