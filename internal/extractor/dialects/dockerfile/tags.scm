; dialects/dockerfile/tags.scm — stage, ARG and ENV key definitions, plus
; naive same-file stage-dependency references (camdencheek/tree-sitter-
; dockerfile v0.2.0, MIT; commit 868e44ce378deb68aac902a9db68ff82d2299dd0).
;
; No upstream tags.scm ships with this grammar (its queries/ directory holds
; only highlights.scm, which has no @definition/@name convention to borrow);
; aider and nvim-treesitter carry no dockerfile query either (design §13
; step 11). Written from scratch against `indexbench lang-probe` output over
; testdata/*.Dockerfile — never against grammar.js/node-types.json.

; FROM ... AS <alias> — the stage is named by its alias.
(from_instruction
  as: (image_alias) @name) @definition.stage

; FROM <image-ref> with no AS — the stage is named by the image reference
; text (matches the incumbent dockerextract.go's fallback naming exactly).
(from_instruction
  !as
  (image_spec) @name) @definition.stage

; A FROM's base image/stage reference: naive same-file call collection (the
; ADR-021 §3 convention every tags dialect follows — unresolved, never
; typed). When the text matches an earlier stage's alias this is a
; stage->base dependency edge; otherwise it names an external image, kept
; for parity with the other tags dialects' "collect every call site" rule.
(from_instruction
  (image_spec
    name: (image_name) @reference.call))

; ARG NAME[=default]
(arg_instruction
  name: (_) @name) @definition.arg

; ENV NAME=value / ENV NAME value (legacy two-token form) — one definition
; per key, even when several keys share one ENV instruction.
(env_instruction
  (env_pair
    name: (_) @name)) @definition.env

; NOTE (design §13 step 12/§9): COPY --from=<stage> is deliberately NOT
; captured as a @reference.call here. tree-sitter-dockerfile's grammar keeps
; every instruction (from_instruction, copy_instruction, run_instruction, …)
; as a FLAT sibling of source_file — a stage's body is a textual run between
; one FROM and the next, never a subtree nested under that FROM. The engine's
; attachTSCalls (internal/extractor/tsextract.go) attaches a call site only
; to a callable def whose byte range CONTAINS it; a copy_instruction several
; instructions after its stage's from_instruction is never contained by it,
; so a @reference.call there would compile and match but attachTSCalls would
; always drop it (best=-1) — captured, counted nowhere, pure noise. Modeling
; "which stage a later instruction belongs to" needs a sequential-section
; scope tree-sitter's query language cannot express (no bounded "until the
; next sibling of this kind"), and reworking attachTSCalls's containment
; rule is a shared tsextract.go change outside one dialect's directory
; (§2.1) — a foundation change, not a worker's. Recorded, not invented
; around: the FROM-references-a-prior-stage edge two rules above still
; gives a real, contained same-file dependency signal for L2.
