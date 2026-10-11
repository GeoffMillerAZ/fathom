package verify

// provenance.go — EVX-2 (docs/plans/portable-receipts.md; vision 43
// sections 6.2 and 6.6, ADR-068 decision 8): what one SARIF run says about
// where its results came from. internal/receipt fills it (the commit, the
// anchors' clean bit, the extractor epoch this package cannot import), and
// RenderSARIFRun writes it as run.properties.fathom, automationDetails.id,
// the driver's version and, only when a repository URI is configured,
// versionControlProvenance.

// Producer is the binary that produced a run: the same name, version and
// build `fathom review` stamps on its response (review.ProducerInfo).
type Producer struct {
	Name    string
	Version string
	Build   string
}

// RunProvenance is one SARIF run's provenance.
type RunProvenance struct {
	// Commit is the full commit id the run's results were read at; "" when
	// the run read no git commit.
	Commit string
	// Clean is true when every anchored result read the commit's own bytes
	// (no anchor is uncommitted); false when one read other bytes, or when
	// there is no commit to compare with.
	Clean          bool
	Producer       Producer
	ExtractorEpoch string
	// RepositoryURI is the operator's configured repository URI
	// ([receipts] repository_uri). Empty writes no versionControlProvenance:
	// fathom never guesses a remote (ADR-068 decision 11).
	RepositoryURI string
}

// properties is run.properties.fathom: snake_case keys, as receipts write.
func (p RunProvenance) properties() map[string]any {
	return map[string]any{
		"commit": p.Commit,
		"clean":  p.Clean,
		"producer": map[string]any{
			"name":    p.Producer.Name,
			"version": p.Producer.Version,
			"build":   p.Producer.Build,
		},
		"extractor_epoch": p.ExtractorEpoch,
	}
}

// automationID is automationDetails.id: the tool as a category, then the
// commit as the run's id ("fathom-verify/<commit>"). Code scanning keys an
// analysis by the category, so two tools never replace each other's alerts;
// with no commit the id is the category alone (a trailing slash).
func automationID(tool, commit string) string {
	return toolCategory(tool) + "/" + commit
}

// toolCategory is tool lowercased with every run of other characters
// folded to one hyphen: "fathom review --receipt" is "fathom-review-receipt".
func toolCategory(tool string) string {
	out := make([]byte, 0, len(tool))
	hyphen := false
	for i := 0; i < len(tool); i++ {
		c := tool[i]
		switch {
		case c >= 'A' && c <= 'Z':
			c += 'a' - 'A'
			fallthrough
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			if hyphen && len(out) > 0 {
				out = append(out, '-')
			}
			hyphen = false
			out = append(out, c)
		default:
			hyphen = true
		}
	}
	return string(out)
}
