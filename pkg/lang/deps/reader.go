package deps

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"slices"
)

const maxReadBytes = 2 << 20

// ErrInput identifies malformed arguments or options. Its errors never echo
// caller-supplied text.
var ErrInput = errors.New("invalid dependency input")

// Resolver resolves a detected construct against the supplied repository view.
// It returns targets and/or a typed unknown. The reader owns source, line,
// raw operand and provenance. A version must come from a manifest pin or
// lockfile. Callers sharing a Reader must supply concurrency-safe resolvers.
type Resolver interface {
	Resolve(ctx context.Context, req ResolveRequest) ResolveResult
}

// ResolveRequest describes a construct in a repository-rooted view.
// Line is one-based when known, or zero when no source line is available.
type ResolveRequest struct {
	Repo fs.FS
	From string
	Raw  string
	Kind Kind
	Line int
}

// ResolveResult holds the targets and optional missing evidence.
type ResolveResult struct {
	Edges   []Edge
	Unknown *Unknown
}

// Option configures a Reader at construction.
type Option func(*readerOptions) error

// WithResolver replaces resolver-driven resolution for an implemented dialect.
// A replacement voids built-in admission for this Reader. Its results read
// Measured false and require TrustUnmeasured to become complete. An unimplemented
// dialect still lacks the detector needed for completeness.
func WithResolver(dialect string, resolver Resolver) Option {
	return func(r *readerOptions) error {
		if !validDialect(dialect) || resolver == nil || r.resolvers[dialect] != nil {
			return fmt.Errorf("%w: expected one resolver per dialect ID", ErrInput)
		}
		r.resolvers[dialect] = resolver
		return nil
	}
}

// WithDeclaredEdges adds direct edges for the specified source paths. Declarations
// never clear an unknown or make a file complete. To must identify a logical
// target. A version must be a manifest or lockfile pin.
func WithDeclaredEdges(edges ...Edge) Option {
	edges = slices.Clone(edges)
	return func(r *readerOptions) error {
		for _, e := range edges {
			if !validPath(e.From) || !validEdge(e) || e.Resolution == ResolvedUnknown {
				return fmt.Errorf("%w: invalid declared edge", ErrInput)
			}
			r.declared[e.From] = append(r.declared[e.From], e)
		}
		return nil
	}
}

// TrustUnmeasured opts into one implemented, unmeasured dialect's results.
// Unknowns still prevent completeness, and failed admission cannot be waived.
func TrustUnmeasured(dialect string) Option {
	return func(r *readerOptions) error {
		if !validDialect(dialect) {
			return fmt.Errorf("%w: invalid trust dialect ID", ErrInput)
		}
		r.trusted[dialect] = true
		return nil
	}
}

// Reader holds immutable per-dialect options. Its zero value uses defaults.
type Reader struct{ readerOptions }

type readerOptions struct {
	resolvers map[string]Resolver
	declared  map[string][]Edge
	trusted   map[string]bool
}

// New constructs a dependency reader. By default it trusts no unmeasured dialect.
func New(opts ...Option) (*Reader, error) {
	r := &Reader{readerOptions{
		resolvers: make(map[string]Resolver),
		declared:  make(map[string][]Edge),
		trusted:   make(map[string]bool),
	}}
	for _, opt := range opts {
		if opt == nil {
			return nil, ErrInput
		}
		if err := opt(&r.readerOptions); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// ReadFile reads source bytes, consulting only repo for resolution. Dialect
// is explicit when provided. Blank dialect infers from the logical path.
func (r *Reader) ReadFile(ctx context.Context, repo fs.FS, in Input, src []byte) (FileDeps, error) {
	if err := validateInput(repo, in); err != nil {
		return FileDeps{}, err
	}
	return r.readFile(ctx, repositoryFS{view: repo}, in, src)
}

func (r *Reader) readFile(ctx context.Context, repo repositoryFS, in Input, src []byte) (FileDeps, error) {
	if err := ctx.Err(); err != nil {
		return FileDeps{}, err
	}
	f := newResult(in.Path, inferDialect(in.Path, in.Dialect))
	if len(src) > maxReadBytes {
		f.addUnknown(Unknown{Code: UnknownReadLimit, DecidableBy: []DecidableBy{DecideObservation}})
		r.finish(&f)
		return f.FileDeps, nil
	}
	switch f.Dialect {
	case "go":
		readGo(ctx, repo, src, r.resolvers[f.Dialect], &f)
	case "python":
		readPythonDependencies(ctx, repo, src, r.resolvers[f.Dialect], &f)
	}
	if err := ctx.Err(); err != nil {
		return FileDeps{}, err
	}
	r.applyDeclarations(&f)
	r.finish(&f)
	return f.FileDeps, nil
}

// ReadFiles reads a non-empty batch of distinct files in logical path order.
// An unreadable file appears as incomplete and the batch returns an error.
// A canceled read returns the completed prefix and the context error.
func (r *Reader) ReadFiles(ctx context.Context, repo fs.FS, inputs []Input) ([]FileDeps, error) {
	out := make([]FileDeps, 0, len(inputs))
	if len(inputs) == 0 {
		return out, fmt.Errorf("%w: expected at least one file", ErrInput)
	}
	inputs = slices.Clone(inputs)
	slices.SortFunc(inputs, func(a, b Input) int { return cmp.Compare(a.Path, b.Path) })
	for i, input := range inputs {
		if err := validateInput(repo, input); err != nil {
			return out, err
		}
		if i != 0 && input.Path == inputs[i-1].Path {
			return out, fmt.Errorf("%w: duplicate file", ErrInput)
		}
	}

	view := repositoryFS{view: repo}
	var readErr error
	for _, input := range inputs {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		src, code := readSource(view, input.Path)
		if err := ctx.Err(); err != nil {
			return out, err
		}
		if code != "" {
			f := newResult(input.Path, inferDialect(input.Path, input.Dialect))
			f.addUnknown(Unknown{Code: code, DecidableBy: []DecidableBy{DecideObservation}})
			r.finish(&f)
			out = append(out, f.FileDeps)
			readErr = errors.New("one or more dependency files could not be read")
			continue
		}
		f, err := r.readFile(ctx, view, input, src)
		if err != nil {
			return out, err
		}
		out = append(out, f)
	}
	return out, readErr
}

func (r *Reader) finish(f *result) {
	r.finishWithAdmission(f, admissions[f.Dialect])
}

func (r *Reader) finishWithAdmission(f *result, state admission) {
	if state == measured && r.resolvers[f.Dialect] != nil {
		state = unmeasured
	}
	f.finish(state, implemented(f.Dialect), r.trusted[f.Dialect])
}

func (r *Reader) applyDeclarations(f *result) {
	for _, e := range r.declared[f.Path] {
		e.Provenance = ProvenanceDeclared
		f.addEdge(e)
	}
}

func validateInput(repo fs.FS, in Input) error {
	if repo == nil || !validPath(in.Path) || (in.Dialect != "" && !validDialect(in.Dialect)) {
		return fmt.Errorf("%w: expected a repository view, relative slash path and valid dialect ID", ErrInput)
	}
	return nil
}

func inferDialect(file, dialect string) string {
	if dialect == "" {
		switch path.Ext(file) {
		case ".go":
			return "go"
		case ".py":
			return "python"
		}
	}
	return dialect
}

func implemented(dialect string) bool { return dialect == "go" || dialect == "python" }

// Read regular files only. Neither filesystem errors nor parser errors are
// surfaced verbatim: their messages can contain repository data or host paths.
func readSource(repo repositoryFS, name string) ([]byte, UnknownCode) {
	info, err := repo.Stat(name)
	if err != nil || !info.Mode().IsRegular() {
		return nil, UnknownMissingConfig
	}
	if info.Size() > maxReadBytes {
		return nil, UnknownReadLimit
	}
	f, err := repo.Open(name)
	if err != nil {
		return nil, UnknownMissingConfig
	}
	defer f.Close()
	src, err := io.ReadAll(io.LimitReader(f, maxReadBytes+1))
	if err != nil {
		return nil, UnknownMissingConfig
	}
	if len(src) > maxReadBytes {
		return nil, UnknownReadLimit
	}
	return src, ""
}
