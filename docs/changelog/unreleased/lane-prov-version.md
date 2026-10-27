### Changed

- `fathom version` on a binary built without a VCS revision (`go install`) reports `(module vX, sum h1:...)` instead of `(revision unknown)`. `version --json` and `/statusz` build info gain `module_version` and `module_sum`.
