// Package vector adapts fathom's SymbolNode domain type onto agentic-kit's
// generalized vector-store foundation
// (foundations/embedding/vector-store), for fathom's optional semantic
// search layer. When a SymbolIndexer is wired into the query engine, Search
// calls embed the query text and retrieve the nearest symbols by cosine
// similarity.
//
// This package intentionally contains no vector-store implementation of its
// own. MemoryVectorStore, Cosine, the VectorStore interface, and the
// generic text Indexer all live in
// github.com/geoffmilleraz/agentic-kit/foundations/embedding/vector-store —
// that package was lifted FROM this one and generalized (ctx-less
// Upsert/Search, an opaque Meta map instead of Symbol/File fields). Do not
// reintroduce a duplicate store implementation here; the only thing fathom
// owns is the SymbolNode <-> vectorstore.Document/Scored mapping below.
//
// Concurrency: SymbolIndexer is safe for concurrent use whenever the
// Embedder and VectorStore handed to NewSymbolIndexer are (the kit's
// MemoryVectorStore is).
package vector

import (
	"context"

	embeddingclient "github.com/geoffmilleraz/agentic-kit/foundations/embedding/embedding-client"
	vectorstore "github.com/geoffmilleraz/agentic-kit/foundations/embedding/vector-store"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// metaSymbolKey and metaFileKey name the vectorstore.Entry.Meta keys this
// package uses to round-trip symbol coordinates through the kit's opaque
// metadata map.
const (
	metaSymbolKey = "symbol"
	metaFileKey   = "file"
)

// Hit is a search result: a matched symbol/file pair and its
// cosine-similarity score against the query.
type Hit struct {
	Symbol string
	File   string
	Score  float64
}

// SymbolIndexer embeds core.SymbolNode symbols and stores them in a kit
// vectorstore.VectorStore, and provides text-based semantic search over the
// stored vectors — translating between fathom's SymbolNode/Hit and the
// kit's Document/Scored types so callers never handle vectorstore.Entry
// directly.
type SymbolIndexer struct {
	inner *vectorstore.Indexer
}

// NewSymbolIndexer returns a SymbolIndexer that uses embedder to produce
// vectors and store to persist them.
func NewSymbolIndexer(embedder embeddingclient.Embedder, store vectorstore.VectorStore) *SymbolIndexer {
	return &SymbolIndexer{inner: vectorstore.NewIndexer(embedder, store)}
}

// Index embeds each SymbolNode (using its Name + Signature as the text) and
// upserts the resulting entries into the VectorStore. The entry ID is the
// symbol key: "<Package>.<Name>". Indexing zero nodes is a no-op.
func (idx *SymbolIndexer) Index(ctx context.Context, nodes []core.SymbolNode) error {
	if len(nodes) == 0 {
		return nil
	}

	docs := make([]vectorstore.Document, len(nodes))
	for i, n := range nodes {
		docs[i] = vectorstore.Document{
			ID:   symbolID(n),
			Text: symbolText(n),
			Meta: map[string]string{metaSymbolKey: n.Name, metaFileKey: n.FilePath},
		}
	}
	return idx.inner.IndexTexts(ctx, docs...)
}

// SearchText embeds query and returns the topN nearest entries from the
// VectorStore by cosine similarity, mapped onto fathom Hits.
func (idx *SymbolIndexer) SearchText(ctx context.Context, text string, topN int) ([]Hit, error) {
	scored, err := idx.inner.SearchText(ctx, text, topN)
	if err != nil {
		return nil, err
	}

	hits := make([]Hit, len(scored))
	for i, s := range scored {
		hits[i] = Hit{Symbol: s.Meta[metaSymbolKey], File: s.Meta[metaFileKey], Score: s.Score}
	}
	return hits, nil
}

// symbolText returns the embedding input text for a SymbolNode.
func symbolText(n core.SymbolNode) string {
	if n.Signature == "" {
		return n.Name
	}
	return n.Name + " " + n.Signature
}

// symbolID returns the stable key used as the vector store entry ID.
func symbolID(n core.SymbolNode) string {
	if n.Package == "" {
		return n.Name
	}
	return n.Package + "." + n.Name
}
