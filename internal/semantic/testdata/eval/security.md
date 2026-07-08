# Security standards (eval fixture)

## Secrets

Never log credentials, API keys, or tokens anywhere in the codebase.

## Scope trap: example path is not the scope

Connector implementations under internal/connector/ must never follow
symlinks during discovery. (For example, a bad symlink once aborted an
ingest that internal/fusion/ had started — but the obligation sits with
the connectors, not with internal/fusion/.)
