package fusion

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func notebookCorpus() map[string]string {
	return map[string]string{
		"analysis/model.ipynb": `{
 "cells": [
  {
   "cell_type": "code",
   "metadata": {},
   "outputs": [],
   "source": [
    "def load(path):\n",
    "    return read(path)\n",
    "\n",
    "df = load(\"a.csv\")"
   ]
  }
 ],
 "metadata": {"language_info": {"name": "python"}},
 "nbformat": 4,
 "nbformat_minor": 5
}
`,
		"analysis/r.ipynb": `{"cells": [{"cell_type": "code", "source": "def nope(): pass"}], "metadata": {"language_info": {"name": "R"}}}`,
	}
}

// TestIngest_Notebook (LB-Q-jupyter): an ingest reads a notebook's code cells with the
// python dialect, stores each definition at the line of the notebook file that holds
// it, and reads a notebook in another language as empty.
func TestIngest_Notebook(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, notebookCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "nb", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "nb", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, o := range obs {
		got[o.SourceRef+"|"+o.Symbol+"|"+string(o.SourceKind)] = o.Line
	}
	want := map[string]int{
		"analysis/model.ipynb|<module>|JupyterModule": 8,
		"analysis/model.ipynb|load|JupyterFunc":       8,
		"analysis/model.ipynb|df|JupyterVar":          11,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("observations\n got: %v\nwant: %v", got, want)
	}
}

// TestReingestFile_Notebook: the one-file refresh reads a notebook as an ingest does.
func TestReingestFile_Notebook(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, notebookCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "nb", Path: dir}); err != nil {
		t.Fatal(err)
	}
	rel := "analysis/model.ipynb"
	edited := `{"cells": [{"cell_type": "code", "source": "def renamed(): pass"}], "metadata": {}}`
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReingestFile(ctx, st, "nb", dir, rel); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "nb", "")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, o := range obs {
		if o.SourceRef == rel && o.SourceKind == "JupyterFunc" {
			got = append(got, o.Symbol)
		}
	}
	if !reflect.DeepEqual(got, []string{"renamed"}) {
		t.Fatalf("functions after ReingestFile = %v, want [renamed]", got)
	}
}
