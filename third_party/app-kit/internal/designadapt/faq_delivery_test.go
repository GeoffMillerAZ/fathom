package designadapt

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFAQDeliveryRejectsInvalidArtifactBeforeIO(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "must-not-exist")
	if WriteFAQDelivery(dir, FAQArtifact{}, nil) == nil || CheckFAQDelivery(dir, FAQArtifact{}, nil) == nil {
		t.Fatal("invalid artifact passed source replay")
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatal("invalid artifact caused filesystem output")
	}
}

func TestFAQDeliveryFixedPrivateInventory(t *testing.T) {
	artifact := FAQArtifact{Templ: "synthetic templ", Model: "synthetic model", CSS: "synthetic css", Defaults: []byte(`{}`)}
	files, err := faqDeliveryFiles(artifact)
	if err != nil || validatePrivateDeliveryFiles(files) != nil {
		t.Fatal("invalid fixed inventory", err)
	}
	names := make([]string, len(files))
	for i, file := range files {
		names[i] = file.name
	}
	if !reflect.DeepEqual(names, []string{"faq.templ", "faq_model.go", "faq.css", "defaults.json", "USAGE.md", "artifact.json"}) {
		t.Fatal("FAQ delivery inventory changed", names)
	}
	for _, text := range []string{"NewFAQReference", "NewFAQDetails", "1.0.22", "no Elements module", "distinct ID prefixes", "private", "not a certified target", "six immutable files"} {
		if !bytes.Contains(files[4].body, []byte(text)) {
			t.Fatal("missing delivery boundary", text)
		}
	}
}
