package iacfacts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDumpReads writes what the readers return for the preregistered subjects of the resource readings (LL-M3, LL-M4 and
// LL-M5, calibration/oracles/lang-library) as JSON, for grade_reads.py to join with the oracles. It skips unless
// FATHOM_READS_OUT is set:
//
//	FATHOM_READS_SUBJECTS=<subjects/tool.json> FATHOM_READS_CLONES=<dir> FATHOM_READS_OUT=<file> go test ./internal/iacfacts -run TestDumpReads
func TestDumpReads(t *testing.T) {
	out := os.Getenv("FATHOM_READS_OUT")
	if out == "" {
		t.Skip("FATHOM_READS_OUT is not set")
	}
	var sf struct {
		Tool     string `json:"tool"`
		Selected []struct {
			Repo string `json:"repo"`
			Path string `json:"path"`
		} `json:"selected"`
	}
	b, err := os.ReadFile(os.Getenv("FATHOM_READS_SUBJECTS"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &sf); err != nil {
		t.Fatal(err)
	}
	clones := os.Getenv("FATHOM_READS_CLONES")
	type dumpAttr struct {
		Path  string   `json:"path"`
		Class Class    `json:"class"`
		Refs  []string `json:"refs,omitempty"`
		File  string   `json:"file"`
		Line  int      `json:"line"`
	}
	type dumpRes struct {
		Key   string     `json:"key"`
		Mode  string     `json:"mode"`
		Type  string     `json:"type"`
		Name  string     `json:"name"`
		Attrs []dumpAttr `json:"attrs"`
	}
	type dumpRoot struct {
		OK           bool      `json:"ok"`
		Error        string    `json:"error,omitempty"`
		Resources    []dumpRes `json:"resources,omitempty"`
		SyntaxErrors int       `json:"syntaxErrors"`
		Dropped      []string  `json:"dropped,omitempty"`
	}
	roots := map[string]dumpRoot{}
	for _, s := range sf.Selected {
		key := s.Repo + ":" + s.Path
		repo := filepath.Join(clones, sf.Tool+"-"+strings.ReplaceAll(s.Repo, "/", "-"))
		var set *ResourceSet
		switch sf.Tool {
		case "terraform":
			set, err = ReadTerraformModule(os.DirFS(repo), filepath.ToSlash(s.Path))
		default:
			src, rerr := os.ReadFile(filepath.Join(repo, s.Path))
			if rerr != nil {
				t.Fatal(rerr)
			}
			if sf.Tool == "cloudformation" {
				set, err = ReadCloudFormationFile(s.Path, src)
			} else {
				set, err = ReadPulumiYAMLFile(s.Path, src)
			}
		}
		if err != nil {
			roots[key] = dumpRoot{Error: err.Error()}
			continue
		}
		d := dumpRoot{OK: true, SyntaxErrors: set.SyntaxErrors, Dropped: set.Dropped}
		for _, r := range set.Resources {
			dr := dumpRes{Key: r.Mode + "." + r.Type + "." + r.Name, Mode: r.Mode, Type: r.Type, Name: r.Name}
			if sf.Tool != "terraform" {
				dr.Key = r.Name
			}
			for _, a := range r.Attrs {
				dr.Attrs = append(dr.Attrs, dumpAttr{Path: a.Path, Class: a.Class, Refs: a.Refs, File: a.Span.File, Line: a.Span.StartLine})
			}
			d.Resources = append(d.Resources, dr)
		}
		roots[key] = d
	}
	data, err := json.MarshalIndent(map[string]any{"reader": "internal/iacfacts " + sf.Tool, "roots": roots}, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
