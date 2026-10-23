### Fixed
- `fathom doctor` judges the pre-push hook git actually runs. When a project-owned hook is installed and `lefthook.yml` is not, the `lefthook_gates` row reads the hook (following `.tools/bin/task validate`) instead of failing on the unused `lefthook.yml`. If the strict verify step sits in a non-shell script such as Python, the row is `unknown` and names that file rather than failing.
