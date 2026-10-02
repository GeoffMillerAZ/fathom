; source: written in this repository against `indexbench lang-probe` output over testdata/*.gradle (no upstream query)
;
; dialects/gradle/imports.scm: the project paths a build names, each a whole-module import
; of the path as written, which the namespace-dir strategy (separator ':', the repository
; root, suffixes /build.gradle and /build.gradle.kts) maps to that project's build script by
; Gradle's default layout: ':a:b' and 'a:b' to a/b/build.gradle. A project whose settings
; give it another projectDir or buildFileName resolves to nothing, and a relative
; project('x') is resolved from the root, not from the declaring project.
;  - a settings script's include entries (top level, or a top-level if body);
;  - a dependencies block's project(':x') and project(path: ':x', ...), bare or inside
;    platform/enforcedPlatform/testFixtures (the same shapes tags.scm reads as project
;    dependencies).

(source_file
  [(juxt_function_call
     function: (identifier) @_inc
     args: (argument_list (string) @import.module))
   (function_call
     function: (identifier) @_inc
     args: (argument_list (string) @import.module))]
  (#eq? @_inc "include"))

; the same, when error recovery wraps the file in an ERROR node
(ERROR
  [(juxt_function_call
     function: (identifier) @_inc
     args: (argument_list (string) @import.module))
   (function_call
     function: (identifier) @_inc
     args: (argument_list (string) @import.module))]
  (#eq? @_inc "include"))

(source_file
  (if_statement
    body: (closure
      [(juxt_function_call
         function: (identifier) @_inc
         args: (argument_list (string) @import.module))
       (function_call
         function: (identifier) @_inc
         args: (argument_list (string) @import.module))]))
  (#eq? @_inc "include"))

; the same, when error recovery wraps the file in an ERROR node
(ERROR
  (if_statement
    body: (closure
      [(juxt_function_call
         function: (identifier) @_inc
         args: (argument_list (string) @import.module))
       (function_call
         function: (identifier) @_inc
         args: (argument_list (string) @import.module))]))
  (#eq? @_inc "include"))

([(juxt_function_call
    function: (identifier) @_deps
    args: (argument_list
      (closure
        (function_call
          function: (identifier)
          args: (argument_list
            (function_call
              function: (identifier) @_project
              args: (argument_list . (string) @import.module .)))))))
  ((identifier) @_deps
    .
    (closure
      (function_call
        function: (identifier)
        args: (argument_list
          (function_call
            function: (identifier) @_project
            args: (argument_list . (string) @import.module .))))))]
  (#eq? @_deps "dependencies")
  (#eq? @_project "project"))

([(juxt_function_call
    function: (identifier) @_deps
    args: (argument_list
      (closure
        (function_call
          function: (identifier)
          args: (argument_list
            (function_call
              function: (identifier) @_wrap
              args: (argument_list
                (function_call
                  function: (identifier) @_project
                  args: (argument_list . (string) @import.module .)))))))))
  ((identifier) @_deps
    .
    (closure
      (function_call
        function: (identifier)
        args: (argument_list
          (function_call
            function: (identifier) @_wrap
            args: (argument_list
              (function_call
                function: (identifier) @_project
                args: (argument_list . (string) @import.module .))))))))]
  (#eq? @_deps "dependencies")
  (#any-of? @_wrap "platform" "enforcedPlatform" "testFixtures")
  (#eq? @_project "project"))

([(juxt_function_call
    function: (identifier) @_deps
    args: (argument_list
      (closure
        (juxt_function_call
          function: (identifier)
          args: (argument_list . (identifier) @_project .))
        .
        (parenthesized_expression . (string) @import.module .))))
  ((identifier) @_deps
    .
    (closure
      (juxt_function_call
        function: (identifier)
        args: (argument_list . (identifier) @_project .))
      .
      (parenthesized_expression . (string) @import.module .)))]
  (#eq? @_deps "dependencies")
  (#eq? @_project "project"))

([(juxt_function_call
    function: (identifier) @_deps
    args: (argument_list
      (closure
        (juxt_function_call
          function: (identifier)
          args: (argument_list . (identifier) @_wrap .))
        .
        (parenthesized_expression
          .
          (function_call
            function: (identifier) @_project
            args: (argument_list . (string) @import.module .))
          .))))
  ((identifier) @_deps
    .
    (closure
      (juxt_function_call
        function: (identifier)
        args: (argument_list . (identifier) @_wrap .))
      .
      (parenthesized_expression
        .
        (function_call
          function: (identifier) @_project
          args: (argument_list . (string) @import.module .))
        .)))]
  (#eq? @_deps "dependencies")
  (#any-of? @_wrap "platform" "enforcedPlatform" "testFixtures")
  (#eq? @_project "project"))

; the statement after a comment, and project(path: ':x', ...)
([(juxt_function_call
    function: (identifier) @_deps
    args: (argument_list
      (closure
        (identifier)
        .
        (function_call
          function: (identifier) @_project
          args: (argument_list . (string) @import.module .)))))
  ((identifier) @_deps
    .
    (closure
      (identifier)
      .
      (function_call
        function: (identifier) @_project
        args: (argument_list . (string) @import.module .))))]
  (#eq? @_deps "dependencies")
  (#eq? @_project "project"))

([(juxt_function_call
    function: (identifier) @_deps
    args: (argument_list
      (closure
        (identifier)
        .
        (function_call
          function: (identifier) @_project
          args: (argument_list
            (map_item
              key: (identifier) @_path
              value: (string) @import.module))))))
  ((identifier) @_deps
    .
    (closure
      (identifier)
      .
      (function_call
        function: (identifier) @_project
        args: (argument_list
          (map_item
            key: (identifier) @_path
            value: (string) @import.module)))))]
  (#eq? @_deps "dependencies")
  (#eq? @_project "project")
  (#eq? @_path "path"))

([(juxt_function_call
    function: (identifier) @_deps
    args: (argument_list
      (closure
        (function_call
          function: (identifier)
          args: (argument_list
            (function_call
              function: (identifier) @_project
              args: (argument_list
                (map_item
                  key: (identifier) @_path
                  value: (string) @import.module))))))))
  ((identifier) @_deps
    .
    (closure
      (function_call
        function: (identifier)
        args: (argument_list
          (function_call
            function: (identifier) @_project
            args: (argument_list
              (map_item
                key: (identifier) @_path
                value: (string) @import.module)))))))]
  (#eq? @_deps "dependencies")
  (#eq? @_project "project")
  (#eq? @_path "path"))

([(juxt_function_call
    function: (identifier) @_deps
    args: (argument_list
      (closure
        (function_call
          function: (identifier)
          args: (argument_list
            (function_call
              function: (identifier) @_wrap
              args: (argument_list
                (function_call
                  function: (identifier) @_project
                  args: (argument_list
                    (map_item
                      key: (identifier) @_path
                      value: (string) @import.module))))))))))
  ((identifier) @_deps
    .
    (closure
      (function_call
        function: (identifier)
        args: (argument_list
          (function_call
            function: (identifier) @_wrap
            args: (argument_list
              (function_call
                function: (identifier) @_project
                args: (argument_list
                  (map_item
                    key: (identifier) @_path
                    value: (string) @import.module)))))))))]
  (#eq? @_deps "dependencies")
  (#any-of? @_wrap "platform" "enforcedPlatform" "testFixtures")
  (#eq? @_project "project")
  (#eq? @_path "path"))
