package receipt

// config.go — the repository URI a SARIF run names in
// versionControlProvenance comes from configuration only: `[receipts]
// repository_uri` in the repository's .fathom/config.toml (or the user
// layer). fathom never derives one from a git remote (ADR-011, ADR-068
// decision 11).

import (
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// ConfiguredRepositoryURI is `[receipts] repository_uri` for the
// repository at root: the project layer over the user layer, "" when
// unset. A config that does not load names none (the SARIF still renders;
// `fathom config` is where a broken config is reported).
func ConfiguredRepositoryURI(root string) string {
	loaded, err := config.Load(config.DefaultUserPath(), projectdir.Config.Path(root))
	if err != nil {
		return ""
	}
	return loaded.Config.Receipts.RepositoryURI
}
