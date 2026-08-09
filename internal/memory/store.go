package memory

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// store.go — where memories live.
//
// Project scope, on the loopback machine, in a file the repository owns:
// `<root>/.fathom/memories.jsonl` (law 13 — user scope may set defaults, only
// project scope governs, and nothing leaves the machine). JSONL rather than a
// store table so phase W lands without touching the store schema that phase V
// is concurrently rewriting; the Store interface here is the seam that makes
// a later move a one-file change.

// FileName is the project-scoped file memories live in, under `.fathom/`.
const FileName = "memories.jsonl"

// Path returns the memories file for a project root.
func Path(root string) string { return filepath.Join(root, ".fathom", FileName) }

// Store persists memories. Implementations must be safe for concurrent use.
type Store interface {
	// List returns memories for a service, sorted by id. service "" returns
	// every memory in the store.
	List(ctx context.Context, service string) ([]Memory, error)
	// Get returns one memory by id. found is false when there is no such
	// memory — including for a RETIRED one, which stays retrievable.
	Get(ctx context.Context, id string) (Memory, bool, error)
	// Put inserts or replaces a memory by id.
	Put(ctx context.Context, m Memory) error
}

// FileStore is a JSONL-backed Store.
//
// Concurrency: safe for concurrent use via sync.Mutex. Writes go through a
// temp file in the same directory followed by a rename, so a crash mid-write
// leaves the previous content intact rather than a truncated file.
type FileStore struct {
	path string
	mu   sync.Mutex
}

// NewFileStore returns a store backed by the JSONL file at path. The file
// need not exist; it is created on the first write.
func NewFileStore(path string) *FileStore { return &FileStore{path: path} }

// FilePath returns the file this store reads and writes.
func (s *FileStore) FilePath() string { return s.path }

// load reads every memory. Caller holds the mutex.
func (s *FileStore) load() ([]Memory, error) {
	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open %s: %w", s.path, err)
	}
	defer func() { _ = f.Close() }()

	var out []Memory
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		var m Memory
		if err := json.Unmarshal([]byte(text), &m); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", s.path, line, err)
		}
		out = append(out, m)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", s.path, err)
	}
	sortMemories(out)
	return out, nil
}

// save writes every memory, sorted by id, atomically. Caller holds the mutex.
func (s *FileStore) save(all []Memory) error {
	sortMemories(all)
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(s.path), err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".memories-*.jsonl")
	if err != nil {
		return fmt.Errorf("create temp beside %s: %w", s.path, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	w := bufio.NewWriter(tmp)
	enc := json.NewEncoder(w)
	for _, m := range all {
		if err := enc.Encode(m); err != nil {
			_ = tmp.Close()
			return fmt.Errorf("encode memory %s: %w", m.ID, err)
		}
	}
	if err := w.Flush(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("flush %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("rename into %s: %w", s.path, err)
	}
	return nil
}

// List implements Store.
func (s *FileStore) List(_ context.Context, service string) ([]Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.load()
	if err != nil {
		return nil, err
	}
	if service == "" {
		return all, nil
	}
	out := make([]Memory, 0, len(all))
	for _, m := range all {
		if m.Service == service {
			out = append(out, m)
		}
	}
	return out, nil
}

// Get implements Store.
func (s *FileStore) Get(_ context.Context, id string) (Memory, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.load()
	if err != nil {
		return Memory{}, false, err
	}
	for _, m := range all {
		if m.ID == id {
			return m, true, nil
		}
	}
	return Memory{}, false, nil
}

// Put implements Store.
func (s *FileStore) Put(_ context.Context, m Memory) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.load()
	if err != nil {
		return err
	}
	replaced := false
	for i := range all {
		if all[i].ID == m.ID {
			all[i] = m
			replaced = true
			break
		}
	}
	if !replaced {
		all = append(all, m)
	}
	return s.save(all)
}

// MemStore is an in-memory Store for tests and for callers that do not want
// to touch the filesystem.
//
// Concurrency: safe for concurrent use via sync.Mutex.
type MemStore struct {
	mu sync.Mutex
	by map[string]Memory
}

// NewMemStore returns an empty in-memory store.
func NewMemStore() *MemStore { return &MemStore{by: map[string]Memory{}} }

// List implements Store.
func (s *MemStore) List(_ context.Context, service string) ([]Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Memory, 0, len(s.by))
	for _, m := range s.by {
		if service != "" && m.Service != service {
			continue
		}
		out = append(out, m.clone())
	}
	sortMemories(out)
	return out, nil
}

// Get implements Store.
func (s *MemStore) Get(_ context.Context, id string) (Memory, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.by[id]
	if !ok {
		return Memory{}, false, nil
	}
	return m.clone(), true, nil
}

// Put implements Store.
func (s *MemStore) Put(_ context.Context, m Memory) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.by[m.ID] = m.clone()
	return nil
}

func sortMemories(ms []Memory) {
	sort.SliceStable(ms, func(i, j int) bool { return ms[i].ID < ms[j].ID })
}
