package main

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func docsOf(t *testing.T, src string) []*yaml.Node {
	t.Helper()
	docs, err := parseDocs([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return docs
}

func encoded(t *testing.T, docs []*yaml.Node) string {
	t.Helper()
	b, err := encodeDocs(docs)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestAblateEdgeRemovesOneItem(t *testing.T) {
	src := "a:\n  needs: [b, {job: c, artifacts: false}, d]\n  dependencies:\n    - b\n  extends: .t\n.t:\n  script: [x]\n"
	docs := docsOf(t, src)
	if !ablateEdge(docs, "a", "needs", "c") {
		t.Fatal("needs c (a job: map) was not removed")
	}
	out := encoded(t, docs)
	if strings.Contains(out, "job: c") || !strings.Contains(out, "- b") && !strings.Contains(out, "[b, d]") {
		t.Errorf("only c must go:\n%s", out)
	}
	docs = docsOf(t, src)
	if !ablateEdge(docs, "a", "extends", ".t") {
		t.Fatal("a scalar extends was not removed")
	}
	if out := encoded(t, docs); strings.Contains(out, "extends") {
		t.Errorf("a scalar edge loses its key:\n%s", out)
	}
	docs = docsOf(t, src)
	if !ablateEdge(docs, "a", "dependencies", "b") {
		t.Fatal("dependencies b was not removed")
	}
	if out := encoded(t, docs); !strings.Contains(out, "dependencies: []") {
		t.Errorf("the emptied list stays, empty:\n%s", out)
	}
	if ablateEdge(docsOf(t, src), "a", "needs", "zz") || ablateEdge(docsOf(t, src), "nope", "needs", "b") {
		t.Error("nothing to remove must report false")
	}
}

func TestAblateEdgeReadsTheLastDuplicate(t *testing.T) {
	// Psych keeps the last of a key written twice: that is the one the tool reads.
	docs := docsOf(t, "a:\n  needs: [x]\na:\n  needs: [y]\n")
	if ablateEdge(docs, "a", "needs", "x") {
		t.Error("the first a is not the job the tool reads")
	}
	if !ablateEdge(docs, "a", "needs", "y") {
		t.Error("the last a's need was not removed")
	}
}

func TestAblateDecl(t *testing.T) {
	docs := docsOf(t, "spec:\n  inputs: {}\n---\na:\n  script: [x]\n.t:\n  image: y\n")
	if !ablateDecl(docs, ".t") {
		t.Fatal(".t was not removed")
	}
	out := encoded(t, docs)
	if strings.Contains(out, ".t:") || !strings.Contains(out, "spec:") || !strings.Contains(out, "a:") {
		t.Errorf("only .t must go, both documents kept:\n%s", out)
	}
}

func TestEncodeKeepsAnchorsAndMergeKeys(t *testing.T) {
	out := encoded(t, docsOf(t, ".base: &base\n  image: x\njob:\n  <<: *base\n  script: [y]\n"))
	if !strings.Contains(out, "&base") || !strings.Contains(out, "<<: *base") {
		t.Errorf("anchors and merge keys must survive a re-encode:\n%s", out)
	}
}
