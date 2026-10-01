; source: murtaza64/tree-sitter-groovy queries/highlights.scm and queries/locals.scm @ 2a6ddd558b6aa39c5b77d8db9fe9baf817486b2c (MIT), read for node vocabulary only; no rule below is copied from them
; changes: written from scratch (the grammar ships no tags.scm and its locals.scm defines parameters only; aider and nvim-treesitter carry no Gradle query), against `indexbench lang-probe` output over testdata/*.gradle
;
; dialects/gradle/tags.scm: what a Gradle build script written in the Groovy DSL declares
; (LB-Q-gradle, design §13). The names are the script's own spelling, compared whole
; (name_match identity), quotes trimmed.
;
; Definitions:
;  - include (GradleInclude): each project path a settings script's `include` names, as
;    written (`include ':app', ':lib'`, `include("core:util")`), at the top level or in a
;    top-level `if` body. An include in a loop or a computed path (`include "x:${it.name}"`
;    inside a closure) is not one; `includeBuild` names a build, not a project.
;  - dependency (GradleDependency): each entry of a `dependencies { }` block (the project's,
;    `buildscript`'s, or one a `subprojects`/`project(':x')` block configures) written as a
;    string notation (`'group:name:version'`, a GString keeps its `${...}`) or as a project
;    (`project(':x')` or `project(path: ':x', ...)`, named by the path), also inside `platform(...)`,
;    `enforcedPlatform(...)` and `testFixtures(...)`. Not one: a version-catalog accessor
;    (`libs.x.y`, a property path the script does not resolve), the map notation
;    (`group: 'g', name: 'n'`), `files(...)`/`fileTree(...)`, and `constraints { }` entries
;    (constraints, not dependencies).
;  - plugin (GradlePlugin): each `id '...'` of the top-level `plugins { }` block (so a
;    `pluginManagement { plugins { } }` version declaration is not one) and each
;    `apply plugin: '...'`. An `apply plugin: SomeClass` names a class, not an id, and an
;    `alias(libs.plugins.x)` a catalog entry: neither is one.
;  - task (GradleTask): `task name { }`, `task name(type: T) { }`, `task('name')`,
;    `task name`, and `tasks.register('name', ...)` / `tasks.create('name', ...)`. A
;    `tasks.named('x') { }` configures a task declared elsewhere and declares nothing.
;
; References (L2, the naive same-file collection): inside a task, each task named by
; `dependsOn` or `finalizedBy` (a string, an identifier, the last segment of `tasks.x`, the
; argument of `tasks.named('x')`, a `dependsOn: [...]` map entry) is a call from that task.
; A project dependency and an include call the project path they name, so the file resolver
; can bind them to that project's build script (imports.scm, namespace-dir over ':').
; Not captured: `x.dependsOn y` outside a task's own body and `tasks.named('x') { dependsOn
; y }` (no definition of x encloses them), `mustRunAfter`/`shouldRunAfter` (ordering, not a
; dependency).
;
; Grammar limits that shape the patterns: tree-sitter-groovy reads a command call whose
; argument is a call, `api project(':x')`, as the call `api project` followed by a
; parenthesized expression `(':x')`, so those forms are matched as two adjacent siblings and
; the sibling is the definition's @end; the statement right after a comment, and a project
; named by a map, read as the configuration's identifier followed by its argument, matched the
; same way; `task name(type: T) { }` reads as the identifier
; `task` followed by the call `name(type: T) { }`, so the definition starts at `task` and
; ends with the call. A comma before a trailing closure (`api project(':x'), { }`) is a
; syntax error the grammar recovers from: the entry before it is still read.

; ── include ──────────────────────────────────────────────────────────────

(source_file
  [(juxt_function_call
     function: (identifier) @_inc
     args: (argument_list (string) @name @definition.include))
   (function_call
     function: (identifier) @_inc
     args: (argument_list (string) @name @definition.include))]
  (#eq? @_inc "include"))

; the same, when error recovery wraps the file in an ERROR node
(ERROR
  [(juxt_function_call
     function: (identifier) @_inc
     args: (argument_list (string) @name @definition.include))
   (function_call
     function: (identifier) @_inc
     args: (argument_list (string) @name @definition.include))]
  (#eq? @_inc "include"))

(source_file
  (if_statement
    body: (closure
      [(juxt_function_call
         function: (identifier) @_inc
         args: (argument_list (string) @name @definition.include))
       (function_call
         function: (identifier) @_inc
         args: (argument_list (string) @name @definition.include))]))
  (#eq? @_inc "include"))

; the same, when error recovery wraps the file in an ERROR node
(ERROR
  (if_statement
    body: (closure
      [(juxt_function_call
         function: (identifier) @_inc
         args: (argument_list (string) @name @definition.include))
       (function_call
         function: (identifier) @_inc
         args: (argument_list (string) @name @definition.include))]))
  (#eq? @_inc "include"))

(source_file
  [(juxt_function_call
     function: (identifier) @_inc
     args: (argument_list (string) @name @reference.call))
   (function_call
     function: (identifier) @_inc
     args: (argument_list (string) @name @reference.call))]
  (#eq? @_inc "include"))

; the same, when error recovery wraps the file in an ERROR node
(ERROR
  [(juxt_function_call
     function: (identifier) @_inc
     args: (argument_list (string) @name @reference.call))
   (function_call
     function: (identifier) @_inc
     args: (argument_list (string) @name @reference.call))]
  (#eq? @_inc "include"))

(source_file
  (if_statement
    body: (closure
      [(juxt_function_call
         function: (identifier) @_inc
         args: (argument_list (string) @name @reference.call))
       (function_call
         function: (identifier) @_inc
         args: (argument_list (string) @name @reference.call))]))
  (#eq? @_inc "include"))

; the same, when error recovery wraps the file in an ERROR node
(ERROR
  (if_statement
    body: (closure
      [(juxt_function_call
         function: (identifier) @_inc
         args: (argument_list (string) @name @reference.call))
       (function_call
         function: (identifier) @_inc
         args: (argument_list (string) @name @reference.call))]))
  (#eq? @_inc "include"))

; ── plugin ───────────────────────────────────────────────────────────────
; The top-level plugins block; after a syntax error elsewhere in the file the grammar can
; read `plugins { }` as the identifier followed by the closure, so both shapes are matched
; (the same holds for `dependencies { }` below).

(source_file
  [(juxt_function_call
     function: (identifier) @_plugins
     args: (argument_list
       (closure
         [(juxt_function_call
            function: (identifier) @_id
            args: (argument_list . (string) @name)) @definition.plugin
          (function_call
            function: (identifier) @_id
            args: (argument_list . (string) @name)) @definition.plugin])))
   ((identifier) @_plugins
     .
     (closure
       [(juxt_function_call
          function: (identifier) @_id
          args: (argument_list . (string) @name)) @definition.plugin
        (function_call
          function: (identifier) @_id
          args: (argument_list . (string) @name)) @definition.plugin]))]
  (#eq? @_plugins "plugins")
  (#eq? @_id "id"))

; the same, when error recovery wraps the file in an ERROR node
(ERROR
  [(juxt_function_call
     function: (identifier) @_plugins
     args: (argument_list
       (closure
         [(juxt_function_call
            function: (identifier) @_id
            args: (argument_list . (string) @name)) @definition.plugin
          (function_call
            function: (identifier) @_id
            args: (argument_list . (string) @name)) @definition.plugin])))
   ((identifier) @_plugins
     .
     (closure
       [(juxt_function_call
          function: (identifier) @_id
          args: (argument_list . (string) @name)) @definition.plugin
        (function_call
          function: (identifier) @_id
          args: (argument_list . (string) @name)) @definition.plugin]))]
  (#eq? @_plugins "plugins")
  (#eq? @_id "id"))

([(juxt_function_call
    function: (identifier) @_apply
    args: (argument_list
      (map_item
        key: (identifier) @_key
        value: (string) @name)))
  (function_call
    function: (identifier) @_apply
    args: (argument_list
      (map_item
        key: (identifier) @_key
        value: (string) @name)))] @definition.plugin
  (#eq? @_apply "apply")
  (#eq? @_key "plugin"))

; ── dependency ───────────────────────────────────────────────────────────
; Every pattern sits directly in a `dependencies { }` closure.

; cfg 'g:a:v'
([(juxt_function_call
    function: (identifier) @_deps
    args: (argument_list
      (closure
        (juxt_function_call
          function: (identifier)
          args: (argument_list (string) @name)) @definition.dependency)))
  ((identifier) @_deps
    .
    (closure
      (juxt_function_call
        function: (identifier)
        args: (argument_list (string) @name)) @definition.dependency))]
  (#eq? @_deps "dependencies")
  (#match? @name ":"))

; cfg('g:a:v') and cfg('g:a:v') { ... }
([(juxt_function_call
    function: (identifier) @_deps
    args: (argument_list
      (closure
        (function_call
          function: (identifier)
          args: (argument_list (string) @name (closure)? @body)) @definition.dependency)))
  ((identifier) @_deps
    .
    (closure
      (function_call
        function: (identifier)
        args: (argument_list (string) @name (closure)? @body)) @definition.dependency))]
  (#eq? @_deps "dependencies")
  (#match? @name ":"))

; cfg(project(':x')), cfg(platform('g:a:v'))
([(juxt_function_call
    function: (identifier) @_deps
    args: (argument_list
      (closure
        (function_call
          function: (identifier)
          args: (argument_list
            (function_call
              function: (identifier) @_wrap
              args: (argument_list . (string) @name .))
            (closure)? @body)) @definition.dependency)))
  ((identifier) @_deps
    .
    (closure
      (function_call
        function: (identifier)
        args: (argument_list
          (function_call
            function: (identifier) @_wrap
            args: (argument_list . (string) @name .))
          (closure)? @body)) @definition.dependency))]
  (#eq? @_deps "dependencies")
  (#any-of? @_wrap "project" "platform" "enforcedPlatform" "testFixtures"))

; cfg(platform(project(':x')))
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
                  args: (argument_list . (string) @name .))))
            (closure)? @body)) @definition.dependency)))
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
                args: (argument_list . (string) @name .))))
          (closure)? @body)) @definition.dependency))]
  (#eq? @_deps "dependencies")
  (#any-of? @_wrap "platform" "enforcedPlatform" "testFixtures")
  (#eq? @_project "project"))

; cfg project(':x') and cfg platform('g:a:v'): the grammar's two adjacent siblings
([(juxt_function_call
    function: (identifier) @_deps
    args: (argument_list
      (closure
        (juxt_function_call
          function: (identifier)
          args: (argument_list . (identifier) @_wrap .)) @definition.dependency
        .
        (parenthesized_expression . (string) @name .) @end)))
  ((identifier) @_deps
    .
    (closure
      (juxt_function_call
        function: (identifier)
        args: (argument_list . (identifier) @_wrap .)) @definition.dependency
      .
      (parenthesized_expression . (string) @name .) @end))]
  (#eq? @_deps "dependencies")
  (#any-of? @_wrap "project" "platform" "enforcedPlatform" "testFixtures"))

; cfg platform(project(':x'))
([(juxt_function_call
    function: (identifier) @_deps
    args: (argument_list
      (closure
        (juxt_function_call
          function: (identifier)
          args: (argument_list . (identifier) @_wrap .)) @definition.dependency
        .
        (parenthesized_expression
          .
          (function_call
            function: (identifier) @_project
            args: (argument_list . (string) @name .))
          .) @end)))
  ((identifier) @_deps
    .
    (closure
      (juxt_function_call
        function: (identifier)
        args: (argument_list . (identifier) @_wrap .)) @definition.dependency
      .
      (parenthesized_expression
        .
        (function_call
          function: (identifier) @_project
          args: (argument_list . (string) @name .))
        .) @end))]
  (#eq? @_deps "dependencies")
  (#any-of? @_wrap "platform" "enforcedPlatform" "testFixtures")
  (#eq? @_project "project"))

; A project dependency calls the path it names (the file resolver's binding).
([(juxt_function_call
    function: (identifier) @_deps
    args: (argument_list
      (closure
        (function_call
          function: (identifier)
          args: (argument_list
            (function_call
              function: (identifier) @_project
              args: (argument_list . (string) @name @reference.call .)))))))
  ((identifier) @_deps
    .
    (closure
      (function_call
        function: (identifier)
        args: (argument_list
          (function_call
            function: (identifier) @_project
            args: (argument_list . (string) @name @reference.call .))))))]
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
                  args: (argument_list . (string) @name @reference.call .)))))))))
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
                args: (argument_list . (string) @name @reference.call .))))))))]
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
        (parenthesized_expression . (string) @name @reference.call .))))
  ((identifier) @_deps
    .
    (closure
      (juxt_function_call
        function: (identifier)
        args: (argument_list . (identifier) @_project .))
      .
      (parenthesized_expression . (string) @name @reference.call .)))]
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
            args: (argument_list . (string) @name @reference.call .))
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
          args: (argument_list . (string) @name @reference.call .))
        .)))]
  (#eq? @_deps "dependencies")
  (#any-of? @_wrap "platform" "enforcedPlatform" "testFixtures")
  (#eq? @_project "project"))

; The statement right after a comment, and a project named by a map (path: ...), read as the
; configuration's identifier followed by its argument: cfg 'g:a:v', cfg project(':x'),
; cfg project(path: ':x', configuration: 'c')
([(juxt_function_call
    function: (identifier) @_deps
    args: (argument_list
      (closure
        (identifier) @definition.dependency
        .
        (string) @name @end)))
  ((identifier) @_deps
    .
    (closure
      (identifier) @definition.dependency
      .
      (string) @name @end))]
  (#eq? @_deps "dependencies")
  (#match? @name ":"))

([(juxt_function_call
    function: (identifier) @_deps
    args: (argument_list
      (closure
        (identifier) @definition.dependency
        .
        (function_call
          function: (identifier) @_wrap
          args: (argument_list . (string) @name .)) @end)))
  ((identifier) @_deps
    .
    (closure
      (identifier) @definition.dependency
      .
      (function_call
        function: (identifier) @_wrap
        args: (argument_list . (string) @name .)) @end))]
  (#eq? @_deps "dependencies")
  (#any-of? @_wrap "project" "platform" "enforcedPlatform" "testFixtures"))

([(juxt_function_call
    function: (identifier) @_deps
    args: (argument_list
      (closure
        (identifier) @definition.dependency
        .
        (function_call
          function: (identifier) @_project
          args: (argument_list
            (map_item
              key: (identifier) @_path
              value: (string) @name))) @end)))
  ((identifier) @_deps
    .
    (closure
      (identifier) @definition.dependency
      .
      (function_call
        function: (identifier) @_project
        args: (argument_list
          (map_item
            key: (identifier) @_path
            value: (string) @name))) @end))]
  (#eq? @_deps "dependencies")
  (#eq? @_project "project")
  (#eq? @_path "path"))

; cfg(project(path: ':x'))
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
                  value: (string) @name)))
            (closure)? @body)) @definition.dependency)))
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
                value: (string) @name)))
          (closure)? @body)) @definition.dependency))]
  (#eq? @_deps "dependencies")
  (#eq? @_project "project")
  (#eq? @_path "path"))

; ... and the path those project dependencies call
([(juxt_function_call
    function: (identifier) @_deps
    args: (argument_list
      (closure
        (identifier)
        .
        (function_call
          function: (identifier) @_project
          args: (argument_list . (string) @name @reference.call .)))))
  ((identifier) @_deps
    .
    (closure
      (identifier)
      .
      (function_call
        function: (identifier) @_project
        args: (argument_list . (string) @name @reference.call .))))]
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
              value: (string) @name @reference.call))))))
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
            value: (string) @name @reference.call)))))]
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
                  value: (string) @name @reference.call))))))))
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
                value: (string) @name @reference.call)))))))]
  (#eq? @_deps "dependencies")
  (#eq? @_project "project")
  (#eq? @_path "path"))

; cfg(platform(project(path: ':x')))
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
                      value: (string) @name))))))) @definition.dependency)))
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
                    value: (string) @name))))))) @definition.dependency))]
  (#eq? @_deps "dependencies")
  (#any-of? @_wrap "platform" "enforcedPlatform" "testFixtures")
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
                      value: (string) @name @reference.call))))))))))
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
                    value: (string) @name @reference.call)))))))))]
  (#eq? @_deps "dependencies")
  (#any-of? @_wrap "platform" "enforcedPlatform" "testFixtures")
  (#eq? @_project "project")
  (#eq? @_path "path"))

; ── task ─────────────────────────────────────────────────────────────────

; task name { ... } and task name
((juxt_function_call
   function: (identifier) @_task
   args: (argument_list . (identifier) @name .)) @definition.task
  .
  (closure)? @end @body
  (#eq? @_task "task"))

; task name(type: T) { ... }: the identifier `task`, then the call
((identifier) @_task @definition.task
  .
  (function_call
    function: (identifier) @name
    args: (argument_list (closure)? @body)) @end
  (#eq? @_task "task"))

; task('name') { ... }
(function_call
  function: (identifier) @_task
  args: (argument_list . (string) @name (closure)? @body)
  (#eq? @_task "task")) @definition.task

; tasks.register('name', T) { ... } and tasks.create(...)
(function_call
  function: (dotted_identifier
    [(identifier) @_tasks
     (dotted_identifier (identifier) @_tasks .)]
    .
    (identifier) @_register .)
  args: (argument_list . (string) @name (closure)? @body)
  (#eq? @_tasks "tasks")
  (#any-of? @_register "register" "create")) @definition.task

; tasks.register('name').configure { ... } after another statement of a block: the grammar
; reads the chain as the command call `tasks.register ('name').configure` and its closure as
; the next sibling
((juxt_function_call
   function: (dotted_identifier (identifier) @_tasks . (identifier) @_register .)
   args: (argument_list
     .
     (dotted_identifier
       .
       (parenthesized_expression . (string) @name .)))) @definition.task
  .
  (closure)? @end @body
  (#eq? @_tasks "tasks")
  (#any-of? @_register "register" "create"))

; dependsOn / finalizedBy inside a task: a string, an identifier, tasks.x
([(juxt_function_call
    function: (identifier) @_dep
    args: (argument_list [(string) (identifier)] @name @reference.call))
  (function_call
    function: (identifier) @_dep
    args: (argument_list [(string) (identifier)] @name @reference.call))]
  (#any-of? @_dep "dependsOn" "finalizedBy"))

([(juxt_function_call
    function: (identifier) @_dep
    args: (argument_list
      (dotted_identifier (identifier) @_tasks . (identifier) @name @reference.call .)))
  (function_call
    function: (identifier) @_dep
    args: (argument_list
      (dotted_identifier (identifier) @_tasks . (identifier) @name @reference.call .)))]
  (#any-of? @_dep "dependsOn" "finalizedBy")
  (#eq? @_tasks "tasks"))

; dependsOn(tasks.named('x'))
((function_call
   function: (identifier) @_dep
   args: (argument_list
     (function_call
       function: (dotted_identifier (identifier) @_tasks . (identifier) @_named .)
       args: (argument_list . (string) @name @reference.call))))
  (#any-of? @_dep "dependsOn" "finalizedBy")
  (#eq? @_tasks "tasks")
  (#eq? @_named "named"))

; dependsOn tasks.named('x'): the grammar's two adjacent siblings
((juxt_function_call
   function: (identifier) @_dep
   args: (argument_list
     . (dotted_identifier (identifier) @_tasks . (identifier) @_named .) .))
  .
  (parenthesized_expression . (string) @name @reference.call .)
  (#any-of? @_dep "dependsOn" "finalizedBy")
  (#eq? @_tasks "tasks")
  (#eq? @_named "named"))

; task x(dependsOn: ['a', 'b']) and dependsOn: 'a'
(map_item
  key: (identifier) @_dep
  value: [(string) @name @reference.call
          (identifier) @name @reference.call
          (list [(string) (identifier)] @name @reference.call)]
  (#any-of? @_dep "dependsOn" "finalizedBy"))
