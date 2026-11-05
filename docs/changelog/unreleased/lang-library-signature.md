### Changed

- `Symbol.Signature` of a symbol whose `Canonical` is `task` (a `package.json` or `composer.json` script) is the program the script runs, not its text. A string literal bound to a name in a parameter default or a rule is masked whatever the name is, where it was masked only for a credential-like name.
