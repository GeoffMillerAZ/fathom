; source: nvim-treesitter/nvim-treesitter runtime/queries/fixture/locals.scm @ 0123456789abcdef0123456789abcdef01234567 (Apache-2.0)
; changes: drafted by indexbench lang-draft (locals2tags): @local.definition -> @name, outer node -> @definition.<kind>; scopes, references and parameters dropped; a draft to tune, never an L1 oracle (ADR-048 amendment)
; inherits: c (not followed; run lang-draft on that locals.scm and merge by hand)

; from locals L57: @local.definition.import
(import_statement
  name: (identifier) @import.name)
