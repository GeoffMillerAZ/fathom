; source: murtaza64/tree-sitter-groovy queries/highlights.scm @ 2a6ddd558b6aa39c5b77d8db9fe9baf817486b2c (MIT), read for node vocabulary only; no rule below is copied from it
; changes: written from scratch (the grammar ships no tags.scm; aider and nvim-treesitter carry no Jenkinsfile query), against `indexbench lang-probe` output over testdata/*
;
; dialects/jenkinsfile/tags.scm: what a Jenkins pipeline declares (LB-Q-jenkinsfile,
; design §13): a Jenkinsfile (basename Jenkinsfile, extension .jenkinsfile) and a shared
; library's global variable (vars/<name>.groovy, claimed by its path, never by .groovy).
; Names are the file's own spelling, compared whole (name_match identity), quotes trimmed.
;
; Definitions (the declarative pipeline's sections, each a block written with its closure):
;  - pipeline (JenkinsPipeline): `pipeline { }`, named `pipeline`; the grammar reads a
;    top-level one as its own `pipeline` node and one inside a function as a call.
;  - stage (JenkinsStage): `stage('name') { }` (and `stage("name") { }`, a GString kept
;    as written), named by its string: under `stages { }`, nested `stages { }`, `parallel { }`
;    and, in a scripted pipeline, anywhere. Declarative stage names are unique in a pipeline,
;    so a stage is named by its string alone (a container that does not chain).
;  - agent, stages, steps, post, environment, parameters, options (JenkinsAgent,
;    JenkinsStages, JenkinsSteps, JenkinsPost, JenkinsEnvironment, JenkinsParameters,
;    JenkinsOptions): each section written with a closure (`agent` also as `agent any`,
;    `agent none` or `agent 'label'`), named by its keyword and qualified by the stage it
;    belongs to (`Build.steps`); a pipeline-level section is the keyword alone.
;  - function (JenkinsFunction): a top-level method, `def name(...) { }` or a typed one, by
;    its name: a Jenkinsfile's helper, a shared-library variable's `call` and its other
;    methods.
;  - library (JenkinsLibrary): each library a file loads, as written (`@Library('a@ref') _`,
;    `@Library(['a', 'b']) _`, `library 'a'`, `library('a')`, `library identifier: 'a@ref'`).
;
; References (L2, the naive same-file collection): every call of a bare name (`sh '...'`,
; `echo(...)`, `buildPlugin(...)`, `helper()`), at any depth, belongs to the smallest
; definition that encloses it: a step to its `steps`, a post condition's steps to `post`, an
; option to `options`, a `when` condition to its stage, a helper's calls to the helper. A call
; through a receiver (`docker.image('x')`, `currentBuild.result`) is not one, and neither is
; a section keyword (pipeline, agent, stages, stage, steps, post, environment, parameters,
; options), which is a definition when it carries its closure and nothing otherwise.

; ── pipeline ─────────────────────────────────────────────────────────────

(pipeline
  "pipeline" @name
  (closure) @body) @definition.pipeline

; the same, when a statement after the pipeline makes the grammar read the
; keyword and its block as an ERROR (the grammar's pipeline rule ends the file)
(ERROR
  "pipeline" @name
  .
  (closure) @body) @definition.pipeline

([(juxt_function_call
    function: (identifier) @name
    args: (argument_list . (closure) @body .))
  (function_call
    function: (identifier) @name
    args: (argument_list . (closure) @body .))] @definition.pipeline
  (#eq? @name "pipeline"))

; ── stage ────────────────────────────────────────────────────────────────

((function_call
   function: (identifier) @_stage
   args: (argument_list . (string) @name . (closure) @body .)) @definition.stage
  (#eq? @_stage "stage"))

; ── sections ─────────────────────────────────────────────────────────────

([(juxt_function_call
    function: (identifier) @name
    args: (argument_list . (closure) @body .))
  (function_call
    function: (identifier) @name
    args: (argument_list . (closure) @body .))] @definition.stages
  (#eq? @name "stages"))

([(juxt_function_call
    function: (identifier) @name
    args: (argument_list . (closure) @body .))
  (function_call
    function: (identifier) @name
    args: (argument_list . (closure) @body .))] @definition.steps
  (#eq? @name "steps"))

([(juxt_function_call
    function: (identifier) @name
    args: (argument_list . (closure) @body .))
  (function_call
    function: (identifier) @name
    args: (argument_list . (closure) @body .))] @definition.post
  (#eq? @name "post"))

([(juxt_function_call
    function: (identifier) @name
    args: (argument_list . (closure) @body .))
  (function_call
    function: (identifier) @name
    args: (argument_list . (closure) @body .))] @definition.environment
  (#eq? @name "environment"))

([(juxt_function_call
    function: (identifier) @name
    args: (argument_list . (closure) @body .))
  (function_call
    function: (identifier) @name
    args: (argument_list . (closure) @body .))] @definition.parameters
  (#eq? @name "parameters"))

([(juxt_function_call
    function: (identifier) @name
    args: (argument_list . (closure) @body .))
  (function_call
    function: (identifier) @name
    args: (argument_list . (closure) @body .))] @definition.options
  (#eq? @name "options"))

([(juxt_function_call
    function: (identifier) @name
    args: (argument_list . (closure) @body .))
  (function_call
    function: (identifier) @name
    args: (argument_list . (closure) @body .))] @definition.agent
  (#eq? @name "agent"))

((juxt_function_call
   function: (identifier) @name
   args: (argument_list . [(identifier) (string)] .)) @definition.agent
  (#eq? @name "agent"))

; ── after a comment ──────────────────────────────────────────────────────
; The grammar reads a command call that follows a comment (on its own line, or
; trailing the statement before) as its parts side by side: `options { }` as the
; identifier and the closure, `sh 'x'` as the identifier and the string, and
; `agent any  // c` followed by `steps { }` as the identifier `agent`, a command
; call `any steps` and the closure. These patterns read those shapes as the
; definitions and calls they are; the closure is the definition's @end.

([(source_file
    (identifier) @name @definition.pipeline
    .
    (closure) @body @end)
  (closure
    (identifier) @name @definition.pipeline
    .
    (closure) @body @end)]
  (#eq? @name "pipeline"))

([(source_file
    (identifier) @name @definition.agent
    .
    (closure) @body @end)
  (closure
    (identifier) @name @definition.agent
    .
    (closure) @body @end)]
  (#eq? @name "agent"))

([(source_file
    (identifier) @name @definition.stages
    .
    (closure) @body @end)
  (closure
    (identifier) @name @definition.stages
    .
    (closure) @body @end)]
  (#eq? @name "stages"))

([(source_file
    (identifier) @name @definition.steps
    .
    (closure) @body @end)
  (closure
    (identifier) @name @definition.steps
    .
    (closure) @body @end)]
  (#eq? @name "steps"))

([(source_file
    (identifier) @name @definition.post
    .
    (closure) @body @end)
  (closure
    (identifier) @name @definition.post
    .
    (closure) @body @end)]
  (#eq? @name "post"))

([(source_file
    (identifier) @name @definition.environment
    .
    (closure) @body @end)
  (closure
    (identifier) @name @definition.environment
    .
    (closure) @body @end)]
  (#eq? @name "environment"))

([(source_file
    (identifier) @name @definition.parameters
    .
    (closure) @body @end)
  (closure
    (identifier) @name @definition.parameters
    .
    (closure) @body @end)]
  (#eq? @name "parameters"))

([(source_file
    (identifier) @name @definition.options
    .
    (closure) @body @end)
  (closure
    (identifier) @name @definition.options
    .
    (closure) @body @end)]
  (#eq? @name "options"))

(closure
  (identifier) @name @definition.agent
  .
  [(identifier) (string)] @end
  (#eq? @name "agent"))

(closure
  (identifier) @name @definition.agent
  .
  (juxt_function_call
    function: (identifier) @end)
  (#eq? @name "agent")
  (#any-of? @end "any" "none"))

(closure
  (juxt_function_call
    function: (identifier) @_arg
    args: (argument_list . (identifier) @name @definition.agent .))
  .
  (closure) @body @end
  (#any-of? @_arg "any" "none")
  (#eq? @name "agent"))

(closure
  (juxt_function_call
    function: (identifier) @_arg
    args: (argument_list . (identifier) @name @definition.stages .))
  .
  (closure) @body @end
  (#any-of? @_arg "any" "none")
  (#eq? @name "stages"))

(closure
  (juxt_function_call
    function: (identifier) @_arg
    args: (argument_list . (identifier) @name @definition.steps .))
  .
  (closure) @body @end
  (#any-of? @_arg "any" "none")
  (#eq? @name "steps"))

(closure
  (juxt_function_call
    function: (identifier) @_arg
    args: (argument_list . (identifier) @name @definition.post .))
  .
  (closure) @body @end
  (#any-of? @_arg "any" "none")
  (#eq? @name "post"))

(closure
  (juxt_function_call
    function: (identifier) @_arg
    args: (argument_list . (identifier) @name @definition.environment .))
  .
  (closure) @body @end
  (#any-of? @_arg "any" "none")
  (#eq? @name "environment"))

(closure
  (juxt_function_call
    function: (identifier) @_arg
    args: (argument_list . (identifier) @name @definition.parameters .))
  .
  (closure) @body @end
  (#any-of? @_arg "any" "none")
  (#eq? @name "parameters"))

(closure
  (juxt_function_call
    function: (identifier) @_arg
    args: (argument_list . (identifier) @name @definition.options .))
  .
  (closure) @body @end
  (#any-of? @_arg "any" "none")
  (#eq? @name "options"))

([(source_file
    (identifier) @_lib
    .
    (string) @name @definition.library)
  (closure
    (identifier) @_lib
    .
    (string) @name @definition.library)]
  (#eq? @_lib "library"))

; ── function ─────────────────────────────────────────────────────────────

(source_file
  (function_definition
    function: (identifier) @name
    body: (closure) @body) @definition.function)

; ── library ──────────────────────────────────────────────────────────────

((annotation
   (identifier) @_lib
   (argument_list
     [(string) @name
      (list (string) @name)])) @definition.library
  (#eq? @_lib "Library"))

([(juxt_function_call
    function: (identifier) @_lib
    args: (argument_list . (string) @name))
  (function_call
    function: (identifier) @_lib
    args: (argument_list . (string) @name))] @definition.library
  (#eq? @_lib "library"))

([(juxt_function_call
    function: (identifier) @_lib
    args: (argument_list
      (map_item
        key: (identifier) @_key
        value: (string) @name)))
  (function_call
    function: (identifier) @_lib
    args: (argument_list
      (map_item
        key: (identifier) @_key
        value: (string) @name)))] @definition.library
  (#eq? @_lib "library")
  (#eq? @_key "identifier"))

; ── references ───────────────────────────────────────────────────────────

([(juxt_function_call
    function: (identifier) @name)
  (function_call
    function: (identifier) @name)] @reference.call
  (#not-any-of? @name "pipeline" "agent" "stages" "stage" "steps" "post" "environment" "parameters" "options" "any" "none"))

; a command call after a comment, read as its name beside its argument
([(source_file
    (identifier) @name @reference.call
    .
    [(string) (closure)])
  (closure
    (identifier) @name @reference.call
    .
    [(string) (closure)])]
  (#not-any-of? @name "pipeline" "agent" "stages" "stage" "steps" "post" "environment" "parameters" "options" "any" "none"))

; the statement after `agent any  // c`, read as the argument of a command call `any x`
(closure
  (juxt_function_call
    function: (identifier) @_arg
    args: (argument_list . (identifier) @name .)) @reference.call
  .
  [(string) (closure)]
  (#any-of? @_arg "any" "none")
  (#not-any-of? @name "pipeline" "agent" "stages" "stage" "steps" "post" "environment" "parameters" "options" "any" "none"))
