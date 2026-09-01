// Command appkit-mcp is the kit's Model Context Protocol server (D11, D15).
// It gives any project's agent two things over one stdio or HTTP endpoint:
//
//   - the author's licensed TailwindPlus corpus, queryable — search_components
//     (hybrid BM25 + structural), get_component (full code by name + axes),
//     list_components / list_sections (browse the catalog); and
//   - the app-kit itself, legible — get_kit_guidance (the hard rules and how
//     to build on the kit), list_kit_decisions / get_kit_decision (the ADRs),
//     plus every ADR exposed as an MCP resource.
//
// The corpus is the SQLite index built by `appkit data build`; this server
// opens it read-only. Run with no flags for stdio (the default an agent host
// launches); pass -http :PORT to serve the Streamable HTTP transport.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/geoffmilleraz/app-kit/internal/twplus/embed"
	"github.com/geoffmilleraz/app-kit/internal/twplus/index"
)

const (
	defaultIndexRel        = "data/tailwindplus/index/components.sqlite"
	defaultPreviewsRel     = "data/tailwindplus/index/previews.sqlite"
	defaultEmbeddingsRel   = "data/tailwindplus/index/embeddings.sqlite"
	defaultMatDashIndexRel = "data/matdash/index/components.sqlite"
	kitModulePath          = "module github.com/geoffmilleraz/app-kit"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "appkit-mcp: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		httpAddr   = flag.String("http", "", "serve Streamable HTTP on this address (e.g. :8765); empty = stdio")
		httpPublic = flag.Bool("http-public", false, "allow -http to bind a non-loopback address (this endpoint serves licensed content — deliberate opt-in, D14)")
		idxFlag    = flag.String("index", "", "path to the components SQLite index (default: env APPKIT_TWPLUS_INDEX, else discovered)")
		pvFlag     = flag.String("previews", "", "path to the previews SQLite file (default: env APPKIT_TWPLUS_PREVIEWS, else alongside the index)")
		embFlag    = flag.String("embeddings", "", "path to the embeddings SQLite file for hybrid search (default: env APPKIT_TWPLUS_EMBEDDINGS, else alongside the index)")
		ollamaFlag = flag.String("ollama", "", "Ollama base URL for query embedding (default: env OLLAMA_HOST, else http://127.0.0.1:11434)")
		rootFlag   = flag.String("kit-root", "", "path to the app-kit checkout for ADRs/guidance (default: env APPKIT_ROOT, else discovered)")
		mdIdxFlag  = flag.String("matdash-index", "", "path to the MatDash components SQLite index (default: env APPKIT_MATDASH_INDEX, else discovered)")
	)
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	kitRoot := resolveKitRoot(*rootFlag)
	idxPath := resolveIndexPath(*idxFlag, kitRoot)

	ix, err := index.Open(idxPath)
	if err != nil {
		return fmt.Errorf("open corpus index %q: %w (build it with `appkit data build`)", idxPath, err)
	}
	defer ix.Close()

	// MatDash index is optional: present when `appkit data matdash-build` has run.
	var mdix *index.Index
	if p := resolveMatDashIndexPath(*mdIdxFlag, kitRoot); p != "" && fileExists(p) {
		if mi, e := index.Open(p); e == nil {
			mdix = mi
			defer mdix.Close()
		} else {
			fmt.Fprintf(os.Stderr, "appkit-mcp: matdash index unavailable: %v\n", e)
		}
	}

	docs := loadKitDocs(kitRoot) // never fatal: guidance tools degrade if absent

	// Previews are optional: get_component_preview reports how to build them if absent.
	pvPath := resolvePreviewsPath(*pvFlag, kitRoot)
	var previews *index.PreviewStore
	if pv, err := index.OpenPreviews(pvPath); err == nil {
		previews = pv
		defer previews.Close()
	} else {
		fmt.Fprintf(os.Stderr, "appkit-mcp: previews unavailable (%v) — get_component_preview will explain how to build them\n", err)
	}

	// Semantic search is optional (ADR-0010): present only when an embeddings
	// file exists. search_components fuses BM25 + vector then; otherwise, and on
	// any query-time embed failure, it is pure BM25.
	var sem *semantic
	embPath := resolveEmbeddingsPath(*embFlag, kitRoot)
	if store, err := index.OpenEmbeddings(embPath); err == nil {
		sem = &semantic{store: store, embedder: embed.NewOllama(resolveOllamaURL(*ollamaFlag), store.Model())}
		defer store.Close()
		fmt.Fprintf(os.Stderr, "appkit-mcp: semantic search enabled (model=%s, %d dims) — query embedding via %s\n",
			store.Model(), store.Dims(), resolveOllamaURL(*ollamaFlag))
	} else {
		fmt.Fprintf(os.Stderr, "appkit-mcp: semantic search off (%v) — BM25 only; build with `appkit data embed`\n", err)
	}

	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "appkit",
		Title:   "app-kit · TailwindPlus + MatDash + kit assistant",
		Version: "v0.1.0",
	}, &mcp.ServerOptions{
		Instructions: serverInstructions(ix, mdix, docs),
	})
	registerComponentTools(srv, ix, sem, mdix)
	registerPreviewTool(srv, previews, pvPath)
	registerKitTools(srv, docs)

	fmt.Fprintf(os.Stderr, "appkit-mcp: corpus=%s previews=%v matdash=%v kit-root=%s adrs=%d\n", idxPath, previews != nil, mdix != nil, kitRoot, len(docs.decisions))

	if *httpAddr != "" {
		addr, err := loopbackAddr(*httpAddr, *httpPublic)
		if err != nil {
			return err
		}
		base := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
		// Restore the Origin / DNS-rebinding protection D14 requires (the SDK
		// applies none when constructed with nil options).
		handler := http.NewCrossOriginProtection().Handler(base)
		hs := &http.Server{Addr: addr, Handler: handler}
		errc := make(chan error, 1)
		go func() { errc <- hs.ListenAndServe() }()
		fmt.Fprintf(os.Stderr, "appkit-mcp: listening on %s (streamable http; serves licensed content — keep it loopback)\n", addr)
		select {
		case <-ctx.Done():
			sc, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return hs.Shutdown(sc) // drain in-flight handlers before the deferred ix.Close()
		case err := <-errc:
			if err != nil && err != http.ErrServerClosed {
				return err
			}
			return nil
		}
	}
	return srv.Run(ctx, &mcp.StdioTransport{})
}

// loopbackAddr normalizes an -http value to host:port: a missing host defaults
// to loopback, and a non-loopback bind is refused unless allowPublic is set —
// this endpoint serves licensed, non-redistributable content (D14: non-loopback
// binds are refused unless explicitly configured).
func loopbackAddr(addr string, allowPublic bool) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("invalid -http address %q (want [host]:port, e.g. :8765): %w", addr, err)
	}
	if host == "" {
		host = "127.0.0.1"
	}
	if !allowPublic && !isLoopbackHost(host) {
		return "", fmt.Errorf("-http %q binds a non-loopback address; this endpoint serves licensed content (D10/D14). Use a loopback host or pass -http-public to override", addr)
	}
	return net.JoinHostPort(host, port), nil
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// semantic holds the optional hybrid-search dependencies: the vector store and
// the query embedder. Its search method fuses BM25 + vector KNN, degrading to
// BM25 whenever the embedder can't be reached at query time.
type semantic struct {
	store    *index.EmbeddingStore
	embedder embed.Embedder
}

func (s *semantic) search(ctx context.Context, ix *index.Index, query string, f index.Filters, limit int) ([]index.SearchResult, error) {
	vecs, err := s.embedder.Embed(ctx, []string{query})
	if err != nil || len(vecs) == 0 {
		fmt.Fprintf(os.Stderr, "appkit-mcp: query embed failed (%v) — BM25 for this query\n", err)
		return ix.SearchComponents(query, f, limit)
	}
	return ix.SearchHybrid(query, vecs[0], s.store, f, limit)
}

// resolveEmbeddingsPath finds the embeddings artifact: flag, env, then alongside
// the index under the kit root, else a walk up from the working directory.
func resolveEmbeddingsPath(flagVal, kitRoot string) string {
	if flagVal != "" {
		return flagVal
	}
	if env := os.Getenv("APPKIT_TWPLUS_EMBEDDINGS"); env != "" {
		return env
	}
	if p := filepath.Join(kitRoot, defaultEmbeddingsRel); fileExists(p) {
		return p
	}
	if root := findUp(func(dir string) bool { return fileExists(filepath.Join(dir, defaultEmbeddingsRel)) }); root != "" {
		return filepath.Join(root, defaultEmbeddingsRel)
	}
	return filepath.Join(kitRoot, defaultEmbeddingsRel)
}

// resolveOllamaURL picks the embedding daemon: flag, OLLAMA_HOST, else default.
func resolveOllamaURL(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	if env := os.Getenv("OLLAMA_HOST"); env != "" {
		return env
	}
	return embed.DefaultOllamaURL
}

// resolveKitRoot finds the app-kit checkout: flag, then env, then a walk up
// from the working directory looking for the module's go.mod.
func resolveKitRoot(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	if env := os.Getenv("APPKIT_ROOT"); env != "" {
		return env
	}
	if root := findUp(func(dir string) bool {
		b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		return err == nil && strings.Contains(string(b), kitModulePath)
	}); root != "" {
		return root
	}
	return "."
}

// resolveIndexPath finds the SQLite corpus: flag, env, kit-root default, else
// a walk up from the working directory.
func resolveIndexPath(flagVal, kitRoot string) string {
	if flagVal != "" {
		return flagVal
	}
	if env := os.Getenv("APPKIT_TWPLUS_INDEX"); env != "" {
		return env
	}
	if p := filepath.Join(kitRoot, defaultIndexRel); fileExists(p) {
		return p
	}
	if root := findUp(func(dir string) bool { return fileExists(filepath.Join(dir, defaultIndexRel)) }); root != "" {
		return filepath.Join(root, defaultIndexRel)
	}
	return defaultIndexRel
}

// resolveMatDashIndexPath finds the MatDash SQLite corpus: flag, env,
// kit-root default, else a walk up from the working directory. Returns "" if
// none found (the index is optional).
func resolveMatDashIndexPath(flagVal, kitRoot string) string {
	if flagVal != "" {
		return flagVal
	}
	if env := os.Getenv("APPKIT_MATDASH_INDEX"); env != "" {
		return env
	}
	if p := filepath.Join(kitRoot, defaultMatDashIndexRel); fileExists(p) {
		return p
	}
	if root := findUp(func(dir string) bool { return fileExists(filepath.Join(dir, defaultMatDashIndexRel)) }); root != "" {
		return filepath.Join(root, defaultMatDashIndexRel)
	}
	return ""
}

// resolvePreviewsPath finds the previews artifact: flag, env, then alongside
// the index under the kit root, else a walk up from the working directory.
func resolvePreviewsPath(flagVal, kitRoot string) string {
	if flagVal != "" {
		return flagVal
	}
	if env := os.Getenv("APPKIT_TWPLUS_PREVIEWS"); env != "" {
		return env
	}
	if p := filepath.Join(kitRoot, defaultPreviewsRel); fileExists(p) {
		return p
	}
	if root := findUp(func(dir string) bool { return fileExists(filepath.Join(dir, defaultPreviewsRel)) }); root != "" {
		return filepath.Join(root, defaultPreviewsRel)
	}
	return filepath.Join(kitRoot, defaultPreviewsRel)
}

// findUp walks from the working directory toward the filesystem root, returning
// the first directory for which match reports true (or "" if none).
func findUp(match func(dir string) bool) string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if match(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func serverInstructions(ix *index.Index, mdix *index.Index, docs *kitDocs) string {
	var b strings.Builder
	b.WriteString("app-kit assistant. Two domains:\n")
	b.WriteString("1. TailwindPlus components — call search_components(query, optional section/framework/tailwind_version/mode/source), then get_component(full_name, framework, tailwind_version, mode, source) for the exact code. Browse with list_sections / list_components. Frameworks: html, react, vue. Tailwind versions: 3, 4. Modes: light, dark, system (none for Ecommerce). Pass source=\"matdash\" to search the MatDash dashboard-template corpus instead (React+Tailwind v4, mode=none).\n")
	b.WriteString("2. Using app-kit itself — call get_kit_guidance first for the hard rules and architecture, then list_kit_decisions / get_kit_decision for the ADR behind any choice. Honor those decisions (no Node; CUE is source of truth; loopback invariant) when writing code for a kit app.\n")
	if c, v, _, err := ix.Counts(); err == nil {
		fmt.Fprintf(&b, "TailwindPlus corpus: %d components, %d variants. ", c, v)
	}
	if mdix != nil {
		if c, v, _, err := mdix.Counts(); err == nil {
			fmt.Fprintf(&b, "MatDash corpus: %d components, %d variants. ", c, v)
		}
	}
	fmt.Fprintf(&b, "Kit decisions available: %d.\n", len(docs.decisions))
	return b.String()
}
