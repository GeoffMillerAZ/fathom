### Fixed

- Terraform and OpenTofu references to local module outputs resolve across sibling files, including indexed module calls. Valid override files merge with their primary declarations. Ambiguous ordinary calls or outputs remain unresolved, and existing local-source bounds still apply.
