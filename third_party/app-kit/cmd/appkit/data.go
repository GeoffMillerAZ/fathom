package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/app-kit/internal/matdash"
	"github.com/geoffmilleraz/app-kit/internal/twplus/embed"
	"github.com/geoffmilleraz/app-kit/internal/twplus/index"
	"github.com/geoffmilleraz/app-kit/internal/twplus/normalize"
	"github.com/geoffmilleraz/app-kit/internal/twplus/parse"
	"github.com/geoffmilleraz/app-kit/internal/twplus/schema"
)

const (
	defaultDumpPath       = "data/tailwindplus/tailwindplus-components.json"
	defaultIndexPath      = "data/tailwindplus/index/components.sqlite"
	defaultPreviewsPath   = "data/tailwindplus/index/previews.sqlite"
	defaultEmbeddingsPath = "data/tailwindplus/index/embeddings.sqlite"
	defaultManifest       = "data/tailwindplus/manifest.json"
	defaultEmbedModel     = "qwen3-embedding-0.6b"
)

// dataCmd dispatches the `appkit data` subcommand family: build the corpus
// index from the licensed dump, then search/get/stats against it.
func dataCmd(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: appkit data <build|previews|embed|search|get|stats|verify|eval> [flags]")
	}
	switch args[0] {
	case "build":
		return dataBuild(ctx, args[1:])
	case "matdash-build":
		return dataMatdashBuild(ctx, args[1:])
	case "previews":
		return dataPreviews(ctx, args[1:])
	case "embed":
		return dataEmbed(ctx, args[1:])
	case "search":
		return dataSearch(args[1:])
	case "get":
		return dataGet(args[1:])
	case "stats":
		return dataStats(args[1:])
	case "verify":
		return dataVerify(ctx, args[1:])
	case "eval":
		return dataEval(args[1:])
	default:
		return fmt.Errorf("unknown data subcommand %q (want build|matdash-build|previews|embed|search|get|stats|verify|eval)", args[0])
	}
}

// manifest is the artifact contract written next to the index (no timestamp:
// git records when artifacts change; determinism leaks otherwise).
type manifest struct {
	SchemaVersion int    `json:"schema_version"`
	Source        source `json:"source"`
	Counts        counts `json:"counts"`
	Index         string `json:"index"`
}

type source struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type counts struct {
	Components int   `json:"components"`
	Variants   int   `json:"variants"`
	Blobs      int   `json:"blobs"`
	RawBytes   int64 `json:"raw_code_bytes"`
	ZstdBytes  int64 `json:"zstd_code_bytes"`
}

func dataBuild(_ context.Context, args []string) error {
	fs := flag.NewFlagSet("appkit data build", flag.ContinueOnError)
	in := fs.String("in", defaultDumpPath, "path to the raw TailwindPlus JSON dump")
	out := fs.String("out", defaultIndexPath, "output path for the SQLite index")
	manifestPath := fs.String("manifest", defaultManifest, "output path for the artifact manifest ('' to skip)")
	validate := fs.Bool("validate", true, "validate every component against the CUE schema")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var validator *schema.Validator
	if *validate {
		v, err := schema.NewValidator()
		if err != nil {
			return err
		}
		validator = v
	}

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		return fmt.Errorf("data build: create index dir: %w", err)
	}
	builder, err := index.NewBuilder(*out)
	if err != nil {
		return err
	}
	defer builder.Close() // no-op after a successful Finalize; rolls back on any error path

	f, err := os.Open(*in)
	if err != nil {
		return fmt.Errorf("data build: open dump: %w", err)
	}
	defer f.Close()

	start := time.Now()
	var nComp int
	walkErr := parse.Walk(f, func(c parse.Component) error {
		comp, rows, err := normalize.Build(c)
		if err != nil {
			return err
		}
		if validator != nil {
			if err := validator.ValidateComponent(comp); err != nil {
				return fmt.Errorf("validate %s: %w", comp.FullName, err)
			}
		}
		if err := builder.AddComponent(comp, rows); err != nil {
			return err
		}
		nComp++
		if nComp%100 == 0 {
			fmt.Fprintf(os.Stderr, "  …%d components\n", nComp)
		}
		return nil
	})
	if walkErr != nil {
		return walkErr
	}

	stats, err := builder.Finalize()
	if err != nil {
		return err
	}

	src, err := fileProvenance(*in)
	if err != nil {
		return err
	}
	m := manifest{
		SchemaVersion: 1,
		Source:        src,
		Counts: counts{
			Components: stats.Components, Variants: stats.Variants, Blobs: stats.Blobs,
			RawBytes: stats.RawBytes, ZstdBytes: stats.ZstdBytes,
		},
		Index: *out,
	}
	if *manifestPath != "" {
		if err := writeManifest(*manifestPath, m); err != nil {
			return err
		}
	}

	idxSize := fileSize(*out)
	fmt.Printf("appkit data build: %d components, %d variants, %d blobs in %s\n",
		stats.Components, stats.Variants, stats.Blobs, time.Since(start).Round(time.Millisecond))
	fmt.Printf("  code %0.1fMB → %0.1fMB zstd (%0.1f%%) · index %0.1fMB → %s\n",
		mb(stats.RawBytes), mb(stats.ZstdBytes), 100*float64(stats.ZstdBytes)/float64(max(stats.RawBytes, 1)),
		mb(idxSize), *out)
	return nil
}

const (
	defaultMatdashSrcPath   = "data/matdash/packages/main/src"
	defaultMatdashIndexPath = "data/matdash/index/components.sqlite"
)

// dataMatdashBuild indexes the MatDash React template source tree into a
// SQLite file, reusing the same index schema as the TailwindPlus corpus.
// Each .tsx file becomes one component with one variant (framework=react,
// language=tsx, tailwind_version=4, mode=none).
// No CUE validation is performed (MatDash has no schema).
func dataMatdashBuild(_ context.Context, args []string) error {
	fs := flag.NewFlagSet("appkit data matdash-build", flag.ContinueOnError)
	src := fs.String("src", defaultMatdashSrcPath, "path to the MatDash src directory")
	out := fs.String("out", defaultMatdashIndexPath, "output path for the SQLite index")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		return fmt.Errorf("data matdash-build: create index dir: %w", err)
	}

	builder, err := index.NewBuilder(*out)
	if err != nil {
		return err
	}
	defer builder.Close()

	start := time.Now()
	var nComp int
	walked, skipped, walkErr := matdash.Walk(*src, func(r matdash.Record) error {
		if err := builder.AddComponent(r.Component, r.Rows); err != nil {
			return err
		}
		nComp++
		if nComp%100 == 0 {
			fmt.Fprintf(os.Stderr, "  …%d components\n", nComp)
		}
		return nil
	})
	if walkErr != nil {
		return walkErr
	}

	stats, err := builder.Finalize()
	if err != nil {
		return err
	}

	idxSize := fileSize(*out)
	fmt.Printf("appkit data matdash-build: %d components, %d variants, %d blobs in %s\n",
		stats.Components, stats.Variants, stats.Blobs, time.Since(start).Round(time.Millisecond))
	fmt.Printf("  walked=%d skipped=%d · code %0.1fMB → %0.1fMB zstd (%0.1f%%) · index %0.1fMB → %s\n",
		walked, skipped,
		mb(stats.RawBytes), mb(stats.ZstdBytes), 100*float64(stats.ZstdBytes)/float64(max(stats.RawBytes, 1)),
		mb(idxSize), *out)
	return nil
}

func dataPreviews(_ context.Context, args []string) error {
	fs := flag.NewFlagSet("appkit data previews", flag.ContinueOnError)
	in := fs.String("in", defaultDumpPath, "path to the raw TailwindPlus JSON dump")
	out := fs.String("out", defaultPreviewsPath, "output path for the previews SQLite file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		return fmt.Errorf("data previews: create dir: %w", err)
	}
	b, err := index.NewPreviewBuilder(*out)
	if err != nil {
		return err
	}
	defer b.Close()

	f, err := os.Open(*in)
	if err != nil {
		return fmt.Errorf("data previews: open dump: %w", err)
	}
	defer f.Close()

	start := time.Now()
	var n int
	walkErr := parse.Walk(f, func(c parse.Component) error {
		comp, _, err := normalize.Build(c)
		if err != nil {
			return err
		}
		for i, v := range comp.Variants {
			if i >= len(c.Raw.Snippets) {
				break
			}
			if err := b.Add(comp.FullName, v.Framework, v.TailwindVersion, v.Mode, c.Raw.Snippets[i].Preview); err != nil {
				return err
			}
		}
		n++
		if n%100 == 0 {
			fmt.Fprintf(os.Stderr, "  …%d components\n", n)
		}
		return nil
	})
	if walkErr != nil {
		return walkErr
	}
	stats, err := b.Finalize()
	if err != nil {
		return err
	}
	fmt.Printf("appkit data previews: %d previews, %d blobs in %s\n",
		stats.Previews, stats.Blobs, time.Since(start).Round(time.Millisecond))
	fmt.Printf("  html %0.1fMB → %0.1fMB zstd · index %0.1fMB → %s\n",
		mb(stats.RawBytes), mb(stats.ZstdBytes), mb(fileSize(*out)), *out)
	return nil
}

// dataEmbed is the offline semantic-search build step: embed every variant's
// text via a local Ollama daemon and write a vec1 embeddings file alongside the
// index. It is strictly optional (ADR-0010) — search runs BM25 without it, and
// runs hybrid (BM25 + vector, RRF-fused) once this file exists and a matching
// embedder is reachable at query time.
func dataEmbed(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("appkit data embed", flag.ContinueOnError)
	idxPath := fs.String("index", defaultIndexPath, "path to the SQLite corpus index")
	out := fs.String("out", defaultEmbeddingsPath, "output path for the embeddings file")
	ollamaURL := fs.String("ollama", embed.DefaultOllamaURL, "Ollama daemon base URL")
	model := fs.String("model", defaultEmbedModel, "embedding model served by Ollama")
	batch := fs.Int("batch", 64, "texts per embed request")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ix, err := index.Open(*idxPath)
	if err != nil {
		return err
	}
	defer ix.Close()
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		return fmt.Errorf("data embed: create dir: %w", err)
	}

	e := embed.NewOllama(*ollamaURL, *model)
	fmt.Fprintf(os.Stderr, "appkit data embed: model=%s ollama=%s → %s\n", *model, *ollamaURL, *out)
	start := time.Now()
	n, err := ix.BuildEmbeddings(ctx, e, *out, *batch)
	if err != nil {
		return err
	}
	fmt.Printf("appkit data embed: %d vectors (%d dims) in %s → %s\n",
		n, e.Dims(), time.Since(start).Round(time.Millisecond), *out)
	fmt.Printf("  index size %0.1fMB · hybrid search active when this file + a matching embedder are present\n", mb(fileSize(*out)))
	return nil
}

func dataSearch(args []string) error {
	fs := flag.NewFlagSet("appkit data search", flag.ContinueOnError)
	idxPath := fs.String("index", defaultIndexPath, "path to the SQLite index")
	section := fs.String("section", "", "filter: section")
	framework := fs.String("framework", "", "filter: framework (html|react|vue)")
	tw := fs.Int("tw", 0, "filter: tailwind version (3|4)")
	mode := fs.String("mode", "", "filter: mode (light|dark|system|none)")
	limit := fs.Int("limit", 10, "max results")
	semantic := fs.Bool("semantic", false, "hybrid BM25 + vector search (needs `appkit data embed` output and a reachable Ollama)")
	embPath := fs.String("embeddings", defaultEmbeddingsPath, "embeddings file for -semantic")
	ollamaURL := fs.String("ollama", embed.DefaultOllamaURL, "Ollama base URL for -semantic")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ix, err := index.Open(*idxPath)
	if err != nil {
		return err
	}
	defer ix.Close()
	query := strings.Join(fs.Args(), " ")
	filters := index.Filters{Section: *section, Framework: *framework, TailwindVersion: *tw, Mode: *mode}

	var res []index.SearchResult
	if *semantic {
		res, err = searchSemantic(context.Background(), ix, *embPath, *ollamaURL, query, filters, *limit)
	} else {
		res, err = ix.SearchComponents(query, filters, *limit)
	}
	if err != nil {
		return err
	}
	for _, r := range res {
		fmt.Printf("%7.3f  %-55s  %s/tw%d/%s\n", r.Score, r.FullName, r.Framework, r.TailwindVersion, r.Mode)
	}
	fmt.Printf("(%d results)\n", len(res))
	return nil
}

// searchSemantic embeds the query with the same model the embeddings were built
// with and fuses BM25 + vector KNN. Any setup gap (missing embeddings, model
// mismatch, Ollama unreachable) prints a note and falls back to BM25, so the
// command always returns results.
func searchSemantic(ctx context.Context, ix *index.Index, embPath, ollamaURL, query string, f index.Filters, limit int) ([]index.SearchResult, error) {
	store, err := index.OpenEmbeddings(embPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "semantic: no embeddings (%v) — falling back to BM25\n", err)
		return ix.SearchComponents(query, f, limit)
	}
	defer store.Close()
	e := embed.NewOllama(ollamaURL, store.Model())
	vecs, err := e.Embed(ctx, []string{query})
	if err != nil || len(vecs) == 0 {
		fmt.Fprintf(os.Stderr, "semantic: embed query failed (%v) — falling back to BM25\n", err)
		return ix.SearchComponents(query, f, limit)
	}
	return ix.SearchHybrid(query, vecs[0], store, f, limit)
}

func dataGet(args []string) error {
	fs := flag.NewFlagSet("appkit data get", flag.ContinueOnError)
	idxPath := fs.String("index", defaultIndexPath, "path to the SQLite index")
	framework := fs.String("framework", "", "framework (html|react|vue)")
	tw := fs.Int("tw", 0, "tailwind version (3|4)")
	mode := fs.String("mode", "", "mode (light|dark|system|none)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("usage: appkit data get <full_name> [-framework -tw -mode]")
	}
	ix, err := index.Open(*idxPath)
	if err != nil {
		return err
	}
	defer ix.Close()
	r, err := ix.GetComponent(strings.Join(fs.Args(), " "), *framework, *tw, *mode)
	if err != nil {
		return err
	}
	fmt.Printf("// %s  [%s · tw%d · %s]\n", r.FullName, r.Framework, r.TailwindVersion, r.Mode)
	fmt.Printf("// available: %v\n", r.Available)
	fmt.Println(r.Code)
	return nil
}

func dataStats(args []string) error {
	fs := flag.NewFlagSet("appkit data stats", flag.ContinueOnError)
	idxPath := fs.String("index", defaultIndexPath, "path to the SQLite index")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ix, err := index.Open(*idxPath)
	if err != nil {
		return err
	}
	defer ix.Close()
	c, v, b, err := ix.Counts()
	if err != nil {
		return err
	}
	fmt.Printf("components=%d variants=%d blobs=%d  (%s)\n", c, v, b, *idxPath)
	secs, err := ix.Sections()
	if err != nil {
		return err
	}
	for _, s := range secs {
		fmt.Printf("  %-16s %d\n", s.Value, s.Count)
	}
	return nil
}

// dataVerify proves the built index is faithful to the licensed source. It runs
// three checks, any of which can fail the command (non-zero exit):
//
//  1. integrity — every blob decompresses and its bytes hash back to the
//     content_hash it is stored under; FTS rows == variants; no dangling refs.
//  2. manifest — the committed manifest's counts match the live index.
//  3. fidelity — re-walks the dump and confirms every source variant's code
//     hash equals the hash the index recorded, i.e. the stored code is
//     byte-identical to the source (the claim docs/design/tailwindplus-parity.md
//     makes). Skipped when the licensed dump is not present (adopters won't
//     have it); integrity + manifest still run.
func dataVerify(_ context.Context, args []string) error {
	fs := flag.NewFlagSet("appkit data verify", flag.ContinueOnError)
	idxPath := fs.String("index", defaultIndexPath, "path to the SQLite index")
	manifestPath := fs.String("manifest", defaultManifest, "manifest to cross-check counts against ('' to skip)")
	srcPath := fs.String("source", defaultDumpPath, "licensed dump to verify byte-fidelity against ('' to skip)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ix, err := index.Open(*idxPath)
	if err != nil {
		return err
	}
	defer ix.Close()

	rep, err := ix.Verify()
	if err != nil {
		return err
	}
	fmt.Printf("integrity: %d components · %d variants · %d blobs · %d fts rows\n",
		rep.Components, rep.Variants, rep.Blobs, rep.FTSRows)
	fmt.Printf("  blobs checked=%d hash_mismatch=%d len_mismatch=%d decode_errors=%d dangling_variants=%d\n",
		rep.BlobsChecked, rep.HashMismatch, rep.LenMismatch, rep.DecodeErrors, rep.DanglingVariants)
	fail := !rep.OK()

	if *manifestPath != "" {
		if m, merr := readManifest(*manifestPath); merr != nil {
			fmt.Printf("manifest: skipped (%v)\n", merr)
		} else if m.Counts.Components != rep.Components || m.Counts.Variants != rep.Variants || m.Counts.Blobs != rep.Blobs {
			fail = true
			fmt.Printf("manifest: MISMATCH — manifest %d/%d/%d vs index %d/%d/%d (rebuild or re-commit manifest)\n",
				m.Counts.Components, m.Counts.Variants, m.Counts.Blobs, rep.Components, rep.Variants, rep.Blobs)
		} else {
			fmt.Printf("manifest: counts match (%d/%d/%d)\n", rep.Components, rep.Variants, rep.Blobs)
		}
	}

	if *srcPath != "" {
		if _, serr := os.Stat(*srcPath); serr != nil {
			fmt.Printf("fidelity: skipped (licensed source not present at %s)\n", *srcPath)
		} else {
			fres, ferr := verifyFidelity(ix, *srcPath, *manifestPath)
			if ferr != nil {
				return ferr
			}
			fmt.Printf("fidelity: %d/%d source variants byte-identical  missing=%d differ=%d\n",
				fres.matched, fres.sourceVariants, fres.missing, fres.differ)
			if orphan := rep.Variants - fres.matched; orphan != 0 && fres.missing == 0 && fres.differ == 0 {
				fmt.Printf("  note: %d index variants not present in the current source\n", orphan)
			}
			if fres.missing > 0 || fres.differ > 0 {
				fail = true
			}
			for _, p := range fres.problems {
				fmt.Printf("  %s\n", p)
			}
		}
	}

	for _, p := range rep.Problems {
		fmt.Printf("  %s\n", p)
	}
	if fail {
		return fmt.Errorf("verify: index FAILED — see problems above")
	}
	fmt.Println("verify: OK — index is internally consistent and faithful to the licensed source")
	return nil
}

type fidelityResult struct {
	sourceVariants int
	matched        int
	missing        int // source variant absent from the index
	differ         int // present but stored code hash differs from source
	problems       []string
}

// verifyFidelity re-walks the source dump and compares each variant's freshly
// computed content hash to the hash the index recorded. content_hash is
// sha256(code), so equal hashes mean byte-identical code.
func verifyFidelity(ix *index.Index, srcPath, manifestPath string) (fidelityResult, error) {
	var res fidelityResult
	if manifestPath != "" {
		if m, err := readManifest(manifestPath); err == nil && m.Source.SHA256 != "" {
			if prov, err := fileProvenance(srcPath); err == nil && prov.SHA256 != m.Source.SHA256 {
				res.problems = append(res.problems, fmt.Sprintf(
					"source sha256 differs from manifest (on-disk %s… vs manifest %s…) — verifying against the on-disk dump",
					prov.SHA256[:12], m.Source.SHA256[:12],
				))
			}
		}
	}
	hashes, err := ix.VariantHashes()
	if err != nil {
		return res, err
	}
	f, err := os.Open(srcPath)
	if err != nil {
		return res, fmt.Errorf("data verify: open source: %w", err)
	}
	defer f.Close()

	const maxProblems = 20
	walkErr := parse.Walk(f, func(c parse.Component) error {
		_, rows, err := normalize.Build(c)
		if err != nil {
			return err
		}
		for _, r := range rows {
			res.sourceVariants++
			k := index.VariantKey{FullName: r.FullName, Framework: r.Framework, TailwindVersion: r.TailwindVersion, Mode: r.Mode}
			switch got, ok := hashes[k]; {
			case !ok:
				res.missing++
				if len(res.problems) < maxProblems {
					res.problems = append(res.problems, fmt.Sprintf("missing in index: %s [%s/%d/%s]", r.FullName, r.Framework, r.TailwindVersion, r.Mode))
				}
			case got != r.ContentHash:
				res.differ++
				if len(res.problems) < maxProblems {
					res.problems = append(res.problems, fmt.Sprintf("code differs: %s [%s/%d/%s]", r.FullName, r.Framework, r.TailwindVersion, r.Mode))
				}
			default:
				res.matched++
			}
		}
		return nil
	})
	return res, walkErr
}

func readManifest(path string) (manifest, error) {
	var m manifest
	data, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("parse manifest %s: %w", path, err)
	}
	return m, nil
}

func fileProvenance(path string) (source, error) {
	f, err := os.Open(path)
	if err != nil {
		return source{}, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return source{}, err
	}
	return source{Path: path, SHA256: hex.EncodeToString(h.Sum(nil)), Bytes: n}, nil
}

func writeManifest(path string, m manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func fileSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}

func mb(n int64) float64 { return float64(n) / (1 << 20) }
