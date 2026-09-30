; A locals-style pattern whose predicate the engine does not run.
((function_definition
  name: (identifier) @local.definition.function)
  (#has-ancestor? @local.definition.function class_definition))
