### Fixed
- `fathom doctor` reads Python steps in hook, stdio and pre-push gate wrappers statically (scripts, `python -m`, `uv run` and similar launchers, Python shebangs) instead of failing them: a literal `fathom` spawn reads configured (indirect) or PASS, an unsettled one reads `unknown` naming file, line and cause, and a script that starts nothing is named in the FAIL. Nothing is executed.
