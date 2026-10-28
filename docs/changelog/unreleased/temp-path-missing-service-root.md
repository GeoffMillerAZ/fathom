### Fixed

- `fathom intent audit` no longer refuses a nested service as cross-repo when the repository is opened through a symlinked path and the service's directory does not exist on this host. A missing path now resolves the symlinks of its deepest existing ancestor before roots are compared.
