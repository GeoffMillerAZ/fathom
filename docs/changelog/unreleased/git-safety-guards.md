### Changed

- The pre-tool edit hook now checks commands within shell chains and nested commands, broadening the commands existing repository guards match while preserving their whole-command glob matches. Readout checks for required arguments share the segment matcher.
- Advisory repository guards no longer hide enforced command guards, and repository or user notes appear before additional git-safety advice.
