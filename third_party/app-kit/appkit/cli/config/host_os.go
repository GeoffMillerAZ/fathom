package config

import (
	"io/fs"
	"os"
)

// osHost is the production Host: the real filesystem rooted at "/" (via
// os.DirFS), the real environment, the real working directory. Nothing in
// this package's own tests uses it — Load's tests inject an
// fstest.MapFS-backed Host instead, which is the whole point of the seam:
// real I/O has exactly one adapter, and it is not the one under test.
type osHost struct{ fs.FS }

// NewOSHost returns the real-filesystem Host — what a non-test caller of
// Load passes as Options.Host.
func NewOSHost() Host { return osHost{FS: os.DirFS("/")} }

func (osHost) Getenv(key string) string { return os.Getenv(key) }
func (osHost) Getwd() (string, error)   { return os.Getwd() }
