### Fixed

- A CloudFormation catalog's `Contract` no longer marks a read-only property `optional` (it cannot be set) and states neither required nor optional for a nested create-only entry, which the pruned schema cannot support. `fn::secret` around a Pulumi list or mapping literal reads `literal`, as the unwrapped literal and `fn::secret` around a scalar do, so a hardcoded-secret check sees it. The CloudFormation change check no longer reports a changed property when only the spelling moved (quotes, `!Ref` against `Ref`, JSON rewritten as YAML).
