package catalog

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFAQMetadataKeepsRuntimeChoicesAndPrivateDeliveryExplicit(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	e, ok := c.Get("reference.faq-adapter")
	if !ok {
		t.Fatal("missing FAQ tooling")
	}
	dependencies := strings.Join(e.Dependencies, " ")
	for _, phrase := range []string{"Reference alone", "@tailwindplus/elements 1.0.22", "Details needs no JavaScript", "faq.templ and faq_model.go", "faq.css"} {
		if !strings.Contains(dependencies, phrase) {
			t.Error("missing runtime/build boundary", phrase)
		}
	}
	for _, phrase := range []string{"NewFAQReference", "NewFAQDetails", "constructor errors before response output", "unique item keys", "distinct ID prefixes", "six immutable files"} {
		if !strings.Contains(e.Usage, phrase) {
			t.Error("missing typed contract", phrase)
		}
	}
	for _, phrase := range []string{"private output retention", "plain content", "state policy", "consumer task"} {
		if !strings.Contains(strings.Join(e.AppOwns, " "), phrase) {
			t.Error("missing app ownership", phrase)
		}
	}
	encoded, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"<el-disclosure", "<details", "<summary", "class=", "sourceExpected", "sourceDefaults"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Error("metadata includes source/default body", forbidden)
		}
	}
}
