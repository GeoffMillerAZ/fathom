package fusion

import (
	"sync"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// The per-file exclusion reasons the dialect gate adds to file_stats and
// service_summary's excluded counts (design §5), beside "oversize" and
// "unreadable".
const (
	// ExcludedSkippedGenerated: a dialect-owned file that is minified or
	// carries a generator marker (LB-F10) — fingerprinted, never extracted.
	ExcludedSkippedGenerated = "skipped_generated"
	// ExcludedAmbiguous: an extension more than one dialect claims whose
	// sniffs all missed (LB-F9) — left unclassified, never guessed.
	ExcludedAmbiguous = "ambiguous_unclassified"
	// CountClassifiedBySniff is not an exclusion: the number of files a
	// sniff classified, disclosed beside the exclusions (§5: "the count of
	// files classified by sniff … is reported").
	CountClassifiedBySniff = "classified_by_sniff"
)

var (
	dialectRulesOnce sync.Once
	dialectRulesAll  []connector.DialectRule
)

// dialectRules is every embedded manifest's match block as the connector's
// classification consumes it. A manifest load error leaves no rules — the
// gate then passes every file, exactly the pre-manifest behavior —
// and TestDialectManifests fails the build's tests on the same error.
func dialectRules() []connector.DialectRule {
	dialectRulesOnce.Do(func() {
		ds, err := extractor.LoadDialects()
		if err != nil {
			return
		}
		for _, d := range ds {
			r := connector.DialectRule{
				ID:        d.ID,
				Basenames: d.Match.Basenames,
				Globs:     d.Match.Globs,
				Shebangs:  d.Match.Shebangs,
			}
			r.Extensions = append(r.Extensions, d.Match.Extensions...)
			for _, v := range d.Variants {
				r.Extensions = append(r.Extensions, v.Extensions...)
			}
			if s := d.Match.Sniff; s != nil {
				r.Sniff = &connector.DialectSniffRule{Extensions: s.Extensions, Pattern: s.Pattern, Priority: s.Priority, ScanBytes: s.ScanBytes}
			}
			if c := d.Match.Container; c != nil {
				r.Containers = c.Extensions // ADR-021 (1), amended 2026-10-24: binaryExcluded's exemption
			}
			dialectRulesAll = append(dialectRulesAll, r)
		}
	})
	return dialectRulesAll
}

// dialectVerdict is the gate's decision on one read file.
type dialectVerdict struct {
	// excluded is "", ExcludedSkippedGenerated or ExcludedAmbiguous.
	excluded string
	// sniffed: a sniff classified the file.
	sniffed bool
}

// gateDialectFile classifies relPath against the manifests (LB-F9) and
// applies the generated/minified gate (LB-F10) to a dialect-owned file.
// The gate covers every manifest dialect (ADR-048 §7: "apply to every
// dialect"); a native extractor's file (a protoc-gen-go stub) is not a
// dialect's and is still extracted and tagged Generated (ADR-021 §2 TI-5).
func gateDialectFile(relPath string, content []byte) dialectVerdict {
	return gateDialectFileWith(dialectRules(), relPath, content)
}

func gateDialectFileWith(rules []connector.DialectRule, relPath string, content []byte) dialectVerdict {
	v := connector.ClassifyDialect(rules, relPath, content)
	switch {
	case v.By == connector.ClassifiedAmbiguous:
		return dialectVerdict{excluded: ExcludedAmbiguous}
	case v.Dialect != "" && connector.IsGeneratedOrMinified(relPath, content):
		return dialectVerdict{excluded: ExcludedSkippedGenerated, sniffed: v.By == connector.ClassifiedBySniff}
	}
	return dialectVerdict{sniffed: v.By == connector.ClassifiedBySniff}
}
