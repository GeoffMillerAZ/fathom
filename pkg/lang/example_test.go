package lang_test

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// Bytes and a path go in; facts come out. Nothing else is needed.
func ExampleReader_Extract() {
	r, err := lang.New()
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()

	src := "resource \"aws_s3_bucket\" \"logs\" {\n  bucket = var.name\n}\n\nvariable \"name\" {}\n"
	res, err := r.Extract(context.Background(), lang.File{Path: "infra/main.tf", Src: []byte(src)})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("dialect:", res.Dialect)
	for _, s := range res.Symbols {
		fmt.Printf("line %d: %s (%s, %s)\n", s.Line, s.Name, s.Kind, s.Canonical)
	}
	// Output:
	// dialect: terraform
	// line 1: aws_s3_bucket.logs (TerraformResource, resource)
	// line 5: var.name (TerraformVariable, variable_decl)
}

// A file no dialect reads is not an error: Skipped says why.
func ExampleReader_Extract_skipped() {
	r, err := lang.New()
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()

	res, err := r.Extract(context.Background(), lang.File{Path: "image.bin", Src: []byte("\x00\x01\x02")})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("skipped:", res.Skipped, "symbols:", len(res.Symbols))
	// Output: skipped: binary symbols: 0
}

// Dialects lists what the Reader reads, each with the level readings measured
// (DialectInfo.Level, which moves as languages improve).
func ExampleReader_Dialects() {
	r, err := lang.New()
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()

	for _, d := range r.Dialects() {
		if d.ID == "terraform" {
			fmt.Println(d.ID, d.Kind, d.Engine)
		}
	}
	// Output: terraform tooling native
}

// A Go extractor claims files by extension and answers for them. Its dialect
// reads unmeasured until it ships a reading.
func ExampleWithExtractor() {
	r, err := lang.New(lang.WithExtractor(
		lang.Match{ID: "row-spec", Kind: "tooling", Extensions: []string{".row"}, Kinds: map[string]string{"RowDef": "function"}},
		rowLines{},
	))
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()

	res, err := r.Extract(context.Background(), lang.File{Path: "a.row", Src: []byte("def Main\ndef Other\n")})
	if err != nil {
		log.Fatal(err)
	}
	for _, s := range res.Symbols {
		fmt.Println(res.Dialect, res.Level, s.Key, s.Canonical)
	}
	// Output:
	// row-spec unmeasured app.Main function
	// row-spec unmeasured app.Other function
}

// rowLines turns each "def NAME" line into a symbol.
type rowLines struct{}

func (rowLines) Extract(_ context.Context, f lang.File) (lang.Extracted, error) {
	var out lang.Extracted
	for i, line := range strings.Split(string(f.Src), "\n") {
		if w := strings.Fields(line); len(w) == 2 && w[0] == "def" {
			out.Symbols = append(out.Symbols, lang.Symbol{Name: w[1], Package: "app", Kind: "RowDef", Line: i + 1, EndLine: i + 1, Tier: "verified"})
		}
	}
	return out, nil
}
