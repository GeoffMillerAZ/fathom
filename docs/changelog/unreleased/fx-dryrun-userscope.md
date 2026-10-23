### Fixed
- `fathom init --dry-run` with `--no-user-writes` (or `--harness none`) no longer lists user files such as `~/.fathom/config.toml` as `would-create`. Each shows as `skipped-user-scope`, in text and in `--json` `writes[]`, and the "would write or change N files" count leaves them out.
