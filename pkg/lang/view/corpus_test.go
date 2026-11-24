package view_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang"
	"github.com/geoffmilleraz/fathom/pkg/lang/view"
)

// TestCorpusReading is the HLP-1 measurement. It is opt-in: it skips unless
// FATHOM_LP8_CORPUS names a manifest (a JSON list of {dialect, subject,
// commit, path, selection}). It checks each checkout against its pin,
// derives the selected files from the selection text, reads every file with
// the reader, and prints one deterministic JSON aggregate and its SHA-256.
// No path from the manifest reaches the output.
//
//	FATHOM_LP8_CORPUS=/path/to/manifest.json go test -race -count=1 -run TestCorpusReading -v ./pkg/lang/view
func TestCorpusReading(t *testing.T) {
	mp := os.Getenv("FATHOM_LP8_CORPUS")
	if mp == "" {
		t.Skip("FATHOM_LP8_CORPUS not set")
	}
	raw, err := os.ReadFile(mp)
	if err != nil {
		t.Fatal(err)
	}
	var subjects []corpusSubject
	if err := json.Unmarshal(raw, &subjects); err != nil {
		t.Fatal(err)
	}
	sort.Slice(subjects, func(i, j int) bool {
		if subjects[i].Dialect != subjects[j].Dialect {
			return subjects[i].Dialect < subjects[j].Dialect
		}
		return subjects[i].Subject < subjects[j].Subject
	})
	rd, err := lang.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rd.Close()

	agg := corpusAggregate{Dialects: map[string]*dialectAgg{}, ReaderDialects: map[string]*readerAgg{}}
	for _, s := range subjects {
		if head := strings.TrimSpace(gitOut(t, s.Path, "rev-parse", "HEAD")); head != s.Commit {
			t.Fatalf("%s: HEAD %s is not the pin %s", s.Subject, head, s.Commit)
		}
		if st := gitOut(t, s.Path, "status", "--porcelain", "--untracked-files=all"); st != "" {
			t.Fatalf("%s: checkout is not clean", s.Subject)
		}
		files := selectFiles(t, s)
		da := agg.Dialects[s.Dialect]
		if da == nil {
			da = &dialectAgg{}
			agg.Dialects[s.Dialect] = da
		}
		sa := subjectAgg{Subject: s.Subject, Commit: s.Commit, Selected: len(files)}
		for _, rel := range files {
			src, err := os.ReadFile(filepath.Join(s.Path, filepath.FromSlash(rel)))
			if err != nil {
				t.Fatal(err)
			}
			res, err := rd.Extract(context.Background(), lang.File{Path: rel, Src: src})
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Symbols) == 0 {
				continue
			}
			sa.Files++
			if res.Incomplete {
				sa.Incomplete++
			}
			ra := agg.ReaderDialects[res.Dialect]
			if ra == nil {
				ra = &readerAgg{}
				agg.ReaderDialects[res.Dialect] = ra
			}
			ra.Files++
			ix := view.NewIndex(res)
			for _, sym := range res.Symbols {
				sa.Symbols++
				ra.Symbols++
				if sym.Line > 0 && sym.EndLine >= sym.Line {
					sa.Known++
				} else {
					ra.NoEnd++
				}
			}
			lines := bytes.Count(src, []byte("\n"))
			if len(src) > 0 && src[len(src)-1] != '\n' {
				lines++
			}
			for ln := 1; ln <= lines; ln++ {
				_, st := ix.Enclosing(ln)
				switch st {
				case view.StatusFound:
					sa.Found++
				case view.StatusNone:
					sa.None++
				default:
					sa.Unknown++
				}
			}
			sa.Lines += lines
		}
		da.Subjects = append(da.Subjects, sa)
		da.Selected += sa.Selected
		da.Files += sa.Files
		da.Incomplete += sa.Incomplete
		da.Symbols += sa.Symbols
		da.Known += sa.Known
		da.Lines += sa.Lines
		da.Found += sa.Found
		da.None += sa.None
		da.Unknown += sa.Unknown
	}
	for _, d := range agg.Dialects {
		d.KnownShare = share(d.Known, d.Symbols)
		d.FoundShare = share(d.Found, d.Lines)
		d.NoneShare = share(d.None, d.Lines)
		d.UnknownShare = share(d.Unknown, d.Lines)
		for i := range d.Subjects {
			s := &d.Subjects[i]
			s.KnownShare = share(s.Known, s.Symbols)
			s.FoundShare = share(s.Found, s.Lines)
			s.NoneShare = share(s.None, s.Lines)
			s.UnknownShare = share(s.Unknown, s.Lines)
		}
	}
	for _, r := range agg.ReaderDialects {
		r.NoEndShare = share(r.NoEnd, r.Symbols)
	}
	out, err := json.MarshalIndent(agg, "", "  ") // map keys marshal sorted
	if err != nil {
		t.Fatal(err)
	}
	out = append(out, '\n')
	sum := sha256.Sum256(out)
	fmt.Printf("%s", out)
	fmt.Printf("aggregate sha256: %s\n", hex.EncodeToString(sum[:]))
}

type corpusSubject struct {
	Dialect   string `json:"dialect"`
	Subject   string `json:"subject"`
	Commit    string `json:"commit"`
	Path      string `json:"path"`
	Selection string `json:"selection"`
}

type subjectAgg struct {
	Subject      string  `json:"subject"`
	Commit       string  `json:"commit"`
	Selected     int     `json:"selected_files"`
	Files        int     `json:"files"`
	Incomplete   int     `json:"incomplete_files"`
	Symbols      int     `json:"symbols"`
	Known        int     `json:"known_span"`
	KnownShare   float64 `json:"known_span_share"`
	Lines        int     `json:"lines"`
	Found        int     `json:"found"`
	None         int     `json:"none"`
	Unknown      int     `json:"unknown"`
	FoundShare   float64 `json:"found_share"`
	NoneShare    float64 `json:"none_share"`
	UnknownShare float64 `json:"unknown_share"`
}

type dialectAgg struct {
	Subjects     []subjectAgg `json:"subjects"`
	Selected     int          `json:"selected_files"`
	Files        int          `json:"files"`
	Incomplete   int          `json:"incomplete_files"`
	Symbols      int          `json:"symbols"`
	Known        int          `json:"known_span"`
	KnownShare   float64      `json:"known_span_share"`
	Lines        int          `json:"lines"`
	Found        int          `json:"found"`
	None         int          `json:"none"`
	Unknown      int          `json:"unknown"`
	FoundShare   float64      `json:"found_share"`
	NoneShare    float64      `json:"none_share"`
	UnknownShare float64      `json:"unknown_share"`
}

type readerAgg struct {
	Files      int     `json:"files"`
	Symbols    int     `json:"symbols"`
	NoEnd      int     `json:"symbols_without_end_line"`
	NoEndShare float64 `json:"symbols_without_end_line_share"`
}

type corpusAggregate struct {
	Dialects       map[string]*dialectAgg `json:"dialects"`
	ReaderDialects map[string]*readerAgg  `json:"reader_dialects"`
}

func share(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(int(float64(n)/float64(d)*1e6+0.5)) / 1e6
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", dir}, args...)...)
	b, err := c.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(b)
}

// selectFiles derives the selected paths from the selection text and, where
// the manifest carries a frozen digest, checks it.
func selectFiles(t *testing.T, s corpusSubject) []string {
	t.Helper()
	var prefixes, suffixes []string
	var want string
	sel := s.Selection
	if i := strings.Index(sel, "selectionSHA256="); i >= 0 {
		want = sel[i+len("selectionSHA256="):]
	}
	list := func(key string) []string {
		i := strings.Index(sel, key)
		if i < 0 {
			return nil
		}
		rest := sel[i+len(key):]
		j := strings.Index(rest, "]")
		var out []string
		for _, p := range strings.Split(rest[:j], ",") {
			p = strings.Trim(strings.TrimSpace(p), "'")
			if p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	prefixes = list("prefixes [")
	suffixes = list("suffixes [")
	var wantCount int
	if i := strings.Index(sel, "files="); i >= 0 {
		fmt.Sscanf(sel[i:], "files=%d", &wantCount)
	}
	lsOut := gitOut(t, s.Path, "ls-files", "-s", "-z")
	var files []string
	for _, rec := range strings.Split(lsOut, "\x00") {
		tab := strings.IndexByte(rec, '\t')
		if tab < 0 {
			continue
		}
		mode, path := strings.Fields(rec[:tab])[0], rec[tab+1:]
		if mode != "100644" && mode != "100755" {
			continue
		}
		switch s.Dialect {
		case "python":
			if !strings.HasSuffix(path, ".py") {
				continue
			}
		case "go":
			if !strings.HasSuffix(path, ".go") {
				continue
			}
		default:
			okP := false
			for _, p := range prefixes {
				if strings.HasPrefix(path, p) {
					okP = true
				}
			}
			okS := false
			for _, x := range suffixes {
				if strings.HasSuffix(path, x) {
					okS = true
				}
			}
			if !okP || !okS {
				continue
			}
		}
		files = append(files, path)
	}
	sort.Strings(files)
	if wantCount != 0 && len(files) != wantCount {
		t.Fatalf("%s: selected %d files, the manifest says %d", s.Subject, len(files), wantCount)
	}
	if want != "" {
		h := sha256.New()
		for _, f := range files {
			h.Write([]byte(f))
			h.Write([]byte{0})
		}
		if got := hex.EncodeToString(h.Sum(nil)); got != want {
			t.Fatalf("%s: selection digest %s is not the frozen %s", s.Subject, got, want)
		}
	}
	return files
}
