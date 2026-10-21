### Fixed

- `serve-http` now closes its SQLite store after cooperative managed or unmanaged signal shutdown. An exit that exceeds its deadline records incomplete cleanup before forcing the original exit code. Unmanaged SIGTERM and SIGINT retain codes 143 and 130; managed SIGTERM retains code 0.
