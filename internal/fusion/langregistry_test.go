package fusion

// langregistry_test.go — white-box (package fusion, not fusion_test) so it
// can read the unexported Pipeline.exts map directly. Asserts the bijection
// ADR-021 §1 / LT-1 requires: every extension/basename fusion.New registers
// an extractor for has a language classification in
// connector.RegisteredExtensions, and vice versa — so the next extractor
// literally cannot be added without also declaring its language (and a
// language entry with no extractor behind it is caught too).

import (
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
)

func TestLanguageRegistryParity(t *testing.T) {
	p := New(connector.NewFauxConnector(), nil, Config{})

	pipelineExts := make(map[string]bool, len(p.exts))
	for k := range p.exts {
		pipelineExts[k] = true
	}

	connExts := connector.RegisteredExtensions()
	connSet := make(map[string]bool, len(connExts))
	for _, k := range connExts {
		connSet[k] = true
	}

	var missingLang, missingExtractor []string
	for k := range pipelineExts {
		if !connSet[k] {
			missingLang = append(missingLang, k)
		}
	}
	for k := range connSet {
		if !pipelineExts[k] {
			missingExtractor = append(missingExtractor, k)
		}
	}
	sort.Strings(missingLang)
	sort.Strings(missingExtractor)

	if len(missingLang) > 0 {
		t.Errorf("extractor(s) registered in fusion.New with no language classification in connector.extClassMap: %v", missingLang)
	}
	if len(missingExtractor) > 0 {
		t.Errorf("connector classifies extension(s) with no matching extractor registered in fusion.New: %v", missingExtractor)
	}
}
