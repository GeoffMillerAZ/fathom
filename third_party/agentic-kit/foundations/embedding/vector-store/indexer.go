package vectorstore

import (
	"context"
	"fmt"

	embeddingclient "github.com/geoffmilleraz/agentic-kit/foundations/embedding/embedding-client"
)

// Document is an input to Indexer.IndexTexts: an identified piece of text
// with optional metadata that is carried verbatim into the stored Entry.
type Document struct {
	// ID becomes the Entry.ID; later IndexTexts calls with the same ID
	// replace the stored vector.
	ID string

	// Text is the content handed to the Embedder.
	Text string

	// Meta is copied as-is onto the stored Entry. The Indexer never
	// interprets it.
	Meta map[string]string
}

// Indexer composes an embeddingclient.Embedder with a VectorStore: IndexTexts
// embeds documents and upserts the resulting entries, and SearchText embeds a
// query string and retrieves the nearest stored entries. Indexer is safe for
// concurrent use whenever its Embedder and VectorStore are.
type Indexer struct {
	embedder embeddingclient.Embedder
	store    VectorStore
}

// NewIndexer returns an Indexer that uses embedder to produce vectors and
// store to persist them.
func NewIndexer(embedder embeddingclient.Embedder, store VectorStore) *Indexer {
	return &Indexer{embedder: embedder, store: store}
}

// IndexTexts embeds each document's Text in a single batch call and upserts
// the resulting entries into the VectorStore. Entry ID and Meta are taken
// from the document unchanged. Indexing zero documents is a no-op that never
// touches the embedder.
func (idx *Indexer) IndexTexts(ctx context.Context, docs ...Document) error {
	if len(docs) == 0 {
		return nil
	}

	texts := make([]string, len(docs))
	for i, d := range docs {
		texts[i] = d.Text
	}

	vecs, err := idx.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return err
	}
	if len(vecs) != len(docs) {
		return fmt.Errorf("vectorstore: embedder returned %d vectors for %d texts", len(vecs), len(docs))
	}

	entries := make([]Entry, len(docs))
	for i, d := range docs {
		entries[i] = Entry{ID: d.ID, Vector: vecs[i], Meta: d.Meta}
	}
	return idx.store.Upsert(entries...)
}

// SearchText embeds query and returns the topK nearest entries from the
// VectorStore by cosine similarity.
func (idx *Indexer) SearchText(ctx context.Context, query string, topK int) ([]Scored, error) {
	vec, err := idx.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	return idx.store.Search(vec, topK)
}
