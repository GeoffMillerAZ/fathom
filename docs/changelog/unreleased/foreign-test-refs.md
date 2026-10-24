### Changed

- Symbol-anchored foreign tasks that name tests must also hold a live certificate from at least one named test, in addition to their existing checks. A claimed row that passed before can now report `certificate-absent`. Kiro and Task Master require test names in backticks or `-run` arguments, and Kiro uses only the task text. Rows with an explicit `certified_by` are unchanged.
