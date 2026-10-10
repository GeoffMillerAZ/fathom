package fusion_test

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fusion"
)

// TestResolveImportSpecs_Scala (LB-F438): Scala's imports resolve under the
// namespace-dir strategy with "." between segments: a named import of a class
// file, a selector list (`{Functor => F, _}`: a rename and a wildcard), a
// wildcard of an object's members (`cats.syntax.all._`), Scala 3's `*`, `given`
// and `as`, and a library import that names no file of the subject.
func TestResolveImportSpecs_Scala(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "scalaimports"))
	if err != nil {
		t.Fatal(err)
	}
	files, symbols := loadFileResolveFixture(t, dir, "scala", fixtureFiles(t, dir, ".scala"))
	ris, report := fusion.ResolveImportSpecs(context.Background(), "scala", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("skipped: %s", report.SkipReason)
	}
	got := map[string]string{}
	for _, ri := range ris {
		k := filepath.Base(ri.File) + ":" + ri.Spec.Module + "|" + ri.Spec.LocalName
		var ts []string
		for _, f := range ri.Targets {
			ts = append(ts, filepath.Base(f))
		}
		sort.Strings(ts)
		got[k] = strings.Join(ts, ",")
	}
	want := map[string]string{
		"Use.scala:cats.data.NonEmptyList|NonEmptyList": "NonEmptyList.scala",
		"Use.scala:cats.Functor|F":                      "Functor.scala",
		"Use.scala:cats|*":                              "Functor.scala",
		"Use.scala:cats.syntax.all|*":                   "all.scala",
		"Use.scala:cats.data.Validated|Validated":       "Validated.scala",
		"Use.scala:cats.data.Valid|V":                   "Validated.scala",
		"Use.scala:scala.collection.mutable|mutable":    "",
		"Scala3.scala:cats.data|*":                      "NonEmptyList.scala,Validated.scala",
		"Scala3.scala:cats.syntax.all|*":                "all.scala",
		"Scala3.scala:cats.data.NonEmptyList|Nel":       "NonEmptyList.scala",
	}
	for k, w := range want {
		g, ok := got[k]
		if !ok {
			t.Errorf("no import %s; got %v", k, got)
			continue
		}
		if g != w {
			t.Errorf("%s targets %q, want %q", k, g, w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d imports %v, want %d", len(got), got, len(want))
	}
}

// TestScalaPackageClauses (LB-F438): a definition's package is its file's
// package clauses joined, chained or dotted (`package cats` then `package
// data` is cats.data), so a namespace-dir import binds by package where a
// file does not mirror its directory.
func TestScalaPackageClauses(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "scalaimports"))
	if err != nil {
		t.Fatal(err)
	}
	_, symbols := loadFileResolveFixture(t, dir, "scala", fixtureFiles(t, dir, ".scala"))
	want := map[string]string{"NonEmptyList": "cats.data", "Validated": "cats.data", "Functor": "cats", "all": "cats.syntax", "Use": "app", "three": "app"}
	for _, s := range symbols {
		if w, ok := want[s.Name]; ok && s.Package != w {
			t.Errorf("%s (%s) package %q, want %q", s.Name, s.FilePath, s.Package, w)
		}
	}
}

// TestResolveFileEdges_ScalaImportsAndPackage (LB-F438): a call binds through
// a named import (`NonEmptyList(1, Nil)`), a renamed one (`V(2)` is Valid),
// a wildcard of an object's members (`showAll`) and the caller's own package
// (Validated's `Valid(a)`, no import: Scala sees its package's names).
func TestResolveFileEdges_ScalaImportsAndPackage(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "scalaimports"))
	if err != nil {
		t.Fatal(err)
	}
	files, symbols := loadFileResolveFixture(t, dir, "scala", fixtureFiles(t, dir, ".scala"))
	edges, report := fusion.ResolveFileEdges(context.Background(), "scala", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("skipped: %s", report.SkipReason)
	}
	got := strings.Join(namedEdges(edges, symbols), "\n") + "\n"
	for _, want := range []string{
		"app/src/main/scala/app/Use.scala|Use.go -> core/src/main/scala/cats/data/NonEmptyList.scala|NonEmptyList",
		"app/src/main/scala/app/Use.scala|Use.go -> core/src/main/scala/cats/data/Validated.scala|Valid",
		"app/src/main/scala/app/Use.scala|Use.go -> core/src/main/scala/cats/syntax/all.scala|all.showAll",
	} {
		if !strings.Contains(got, want+"\n") {
			t.Errorf("missing %s; edges:\n%s", want, got)
		}
	}
}
