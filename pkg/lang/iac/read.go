package iac

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/iacfacts"
	"github.com/geoffmilleraz/fathom/internal/readlimit"
)

// DefaultReadDeadline is how long ReadFile and ReadModule wait for one read: 30 seconds, the same bound as
// lang.DefaultReadDeadline. WithDeadline may lower it, never raise it. The value is a literal; TestPublicConstantValues
// holds it to pkg/lang's.
const DefaultReadDeadline = 30 * time.Second

// MaxFileBytes is the largest file a read accepts, the product's own limit of 2 MiB. A larger file is refused whole
// with ErrLimit. A directory read accepts 64 MiB across its files and 4,096 files. The value is a literal;
// TestPublicConstantValues holds it.
const MaxFileBytes = 2 * 1024 * 1024

// ErrLimit is wrapped by every refusal at a limit: a file or directory over its size, more attributes than a read
// holds, a value past its bound. The message names the limit and a count, never source text.
var ErrLimit = iacfacts.ErrLimit

// ErrDeadline is returned by a read that passed its deadline. The reader is abandoned, not stopped, and none of its
// facts are returned.
var ErrDeadline = errors.New("read-deadline: the read passed its deadline and was abandoned, so none of its facts are returned")

// ErrBusy is returned, with nothing read, when GOMAXPROCS reads that passed their deadline are still running. It
// clears as they finish.
var ErrBusy = errors.New("read-busy: reads that passed their deadline are still running, so this read was not started")

// NoReaderError is the error of a file that no reader here reads.
type NoReaderError struct {
	// Path is the path the caller gave.
	Path string
}

func (e *NoReaderError) Error() string {
	return fmt.Sprintf("iac: no reader for %s: it is not Terraform or OpenTofu (.tf, .tofu, .tf.json), CloudFormation (a YAML or JSON template) or Pulumi.yaml", e.Path)
}

// Option configures a read.
type Option func(*config) error

type config struct {
	values   bool
	deadline time.Duration
}

// WithValues keeps Attr.Value, which is empty without it. It is the only way a value is returned. The reader still
// withholds what its source marks secret, and refuses an input whose values pass its bounds.
func WithValues() Option {
	return func(c *config) error { c.values = true; return nil }
}

// WithDeadline lowers the time a read may take. A deadline above DefaultReadDeadline, or one that is not positive,
// is an error: no option raises the bound.
func WithDeadline(d time.Duration) Option {
	return func(c *config) error {
		switch {
		case d <= 0:
			return fmt.Errorf("iac: deadline %v must be positive", d)
		case d > DefaultReadDeadline:
			return fmt.Errorf("iac: deadline %v is above the limit %v; limits may only be lowered", d, DefaultReadDeadline)
		}
		c.deadline = d
		return nil
	}
}

func newConfig(opts []Option) (config, error) {
	c := config{deadline: DefaultReadDeadline}
	for _, o := range opts {
		if o == nil {
			continue
		}
		if err := o(&c); err != nil {
			return c, err
		}
	}
	return c, nil
}

func (c config) factOptions() []iacfacts.Option {
	if c.values {
		return []iacfacts.Option{iacfacts.WithValues()}
	}
	return nil
}

// abandoned counts the reads that passed their deadline and are still running, for every caller of the package.
var abandoned readlimit.Group

// guarded runs fn under the read guarantee: it starts nothing while GOMAXPROCS abandoned reads are still running,
// returns by the deadline and ends with the context. A panic in the reader is the read's failure.
func guarded(ctx context.Context, c config, fn func() (*iacfacts.ResourceSet, error)) (*ResourceSet, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if abandoned.Saturated(runtime.GOMAXPROCS(0)) {
		return nil, ErrBusy
	}
	set, out, err := readlimit.Run(ctx, &abandoned, c.deadline, fn)
	switch out {
	case readlimit.DeadlineExceeded:
		return nil, ErrDeadline
	case readlimit.Canceled:
		return nil, err
	}
	var pe *readlimit.PanicError
	if errors.As(err, &pe) {
		return nil, errors.New("iac: the reader failed on this input") // the panic's text may hold source text
	}
	if err != nil {
		return nil, err
	}
	return convert(set), nil
}

// ReadFile reads one file's resources. The reader is chosen by the path and, for a YAML or JSON template, by the
// content: .tf and .tofu read as Terraform or OpenTofu, .tf.json and .tofu.json as its JSON syntax (which is not
// read: the result holds no resource and one syntax error), Pulumi.yaml as Pulumi YAML, and any other .yaml, .yml,
// .json or .template file that reads as a CloudFormation template as CloudFormation. Any other file returns a
// *NoReaderError.
//
// Path is only a name and a classifier: nothing is read from disk, and it is the File of every Span. src is not
// modified. A file past MaxFileBytes is refused with ErrLimit. The read returns by DefaultReadDeadline (lowered by
// WithDeadline) with ErrDeadline, and with ErrBusy while GOMAXPROCS abandoned reads are running. A context that ends
// first returns its error and abandons the read the same way.
//
// An error never carries a value from the file.
func ReadFile(ctx context.Context, filePath string, src []byte, opts ...Option) (*ResourceSet, error) {
	c, err := newConfig(opts)
	if err != nil {
		return nil, err
	}
	if filePath == "" {
		return nil, errors.New("iac: path is empty")
	}
	p := filepath.ToSlash(filePath)
	read, ok := route(p, src)
	if !ok {
		return nil, &NoReaderError{Path: filePath}
	}
	if len(src) > MaxFileBytes {
		return nil, fmt.Errorf("%w: %s: file is %d bytes, over %d", ErrLimit, p, len(src), MaxFileBytes)
	}
	iopts := c.factOptions()
	return guarded(ctx, c, func() (*iacfacts.ResourceSet, error) { return read(p, src, iopts...) })
}

type reader func(path string, src []byte, opts ...iacfacts.Option) (*iacfacts.ResourceSet, error)

// route picks the reader for a path and its content.
func route(p string, src []byte) (reader, bool) {
	base := path.Base(p)
	lower := strings.ToLower(base)
	switch {
	case strings.HasSuffix(base, ".tf.json"), strings.HasSuffix(base, ".tofu.json"):
		return iacfacts.ReadTerraformJSONFile, true
	case strings.HasSuffix(base, ".tf"), strings.HasSuffix(base, ".tofu"):
		return iacfacts.ReadTerraformFile, true
	case base == "Pulumi.yaml", base == "Pulumi.yml":
		return iacfacts.ReadPulumiYAMLFile, true
	}
	for _, ext := range []string{".yaml", ".yml", ".json", ".template"} {
		if strings.HasSuffix(lower, ext) && iacfacts.LooksLikeCloudFormation(src) {
			return iacfacts.ReadCloudFormationFile, true
		}
	}
	return nil, false
}

// ReadModule reads one Terraform or OpenTofu directory of fsys as a module: every .tf and .tofu file directly in
// dir, with variable defaults and locals resolved across them, so an attribute that reads var.x, or a local chain
// ending in a variable default, is defaulted. It follows OpenTofu's file rules: a .tofu file shadows the .tf file
// of the same name, an override file adds no resource and marks what it names unknown, dot files and editor
// backups are skipped. A module call reads as a relation and is never expanded, and no other directory is listed.
// Spans name the files as dir joined with the file name.
//
// dir is an fs.ValidPath ("." for the root of fsys). The directory is refused with ErrLimit past 4,096 files, 2 MiB
// a file or 64 MiB in all. The deadline, busy and context rules are ReadFile's.
func ReadModule(ctx context.Context, fsys fs.FS, dir string, opts ...Option) (*ResourceSet, error) {
	c, err := newConfig(opts)
	if err != nil {
		return nil, err
	}
	if fsys == nil {
		return nil, errors.New("iac: fsys is nil")
	}
	if !fs.ValidPath(dir) {
		return nil, fmt.Errorf("iac: dir %q is not a valid fs path (slash-separated, no .. or leading slash; \".\" is the root)", dir)
	}
	iopts := c.factOptions()
	return guarded(ctx, c, func() (*iacfacts.ResourceSet, error) { return iacfacts.ReadTerraformModule(fsys, dir, iopts...) })
}
