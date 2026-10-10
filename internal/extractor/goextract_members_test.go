package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// K17 (wave lf2, LB-F342): Go's member pass. A struct's fields are symbols
// named Owner.name (core.KindGoField, canonical field), an embedded field
// named by its type; an interface's methods are Iface.Method with the method
// kind (core.KindGoMethod). Universal-ctags 6.2.1 tags them member,
// anonMember and methodSpec; scip-go defines each as a term of its type.
const membersFixture = `package conf

import (
	"io"
	"sync"
	"time"
)

// Config is a struct with named fields, one line declaring two.
type Config struct {
	Timeout     time.Duration
	Retries, Max int
	inner       string
	_           int
	Hook        func(string) error
	Nested      struct {
		Depth int
	}
}

// Server embeds by pointer, by qualified name and by generic instance.
type Server struct {
	*Config
	sync.Mutex
	io.Reader
	Base[int]
	name string
}

// Box is a generic struct.
type Box[T any] struct {
	Val  T
	next *Box[T]
}

type Base[T any] struct{ ID T }

// Store is an interface with two methods and an embedded interface.
type Store interface {
	Get(key string) (string, error)
	Put(key, value string) error
	io.Closer
}

// Number is a constraint: a type set, no method.
type Number interface {
	~int | ~float64
}

// Alias is an alias of a struct type: not a declared struct of its own.
type Alias = struct{ X int }

func (c *Config) Validate() error { return nil }
`

func TestGoExtractor_Members(t *testing.T) {
	nodes, err := extractor.NewGoExtractor().Extract("svc", "conf/conf.go", []byte(membersFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	byName := map[string]core.SymbolNode{}
	for _, n := range nodes {
		if _, dup := byName[n.Name]; dup {
			t.Fatalf("duplicate symbol %q", n.Name)
		}
		byName[n.Name] = n
	}
	type want struct {
		kind      core.SourceKind
		line, end int
		sig       string
	}
	wants := map[string]want{
		"Config.Timeout": {core.KindGoField, 11, 11, "Timeout time.Duration"},
		"Config.Retries": {core.KindGoField, 12, 12, "Retries int"},
		"Config.Max":     {core.KindGoField, 12, 12, "Max int"},
		"Config.inner":   {core.KindGoField, 13, 13, "inner string"},
		"Config.Hook":    {core.KindGoField, 15, 15, "Hook func(string) error"},
		"Config.Nested":  {core.KindGoField, 16, 18, "Nested struct{Depth int}"},
		"Server.Config":  {core.KindGoField, 23, 23, "*Config"},
		"Server.Mutex":   {core.KindGoField, 24, 24, "sync.Mutex"},
		"Server.Reader":  {core.KindGoField, 25, 25, "io.Reader"},
		"Server.Base":    {core.KindGoField, 26, 26, "Base[int]"},
		"Server.name":    {core.KindGoField, 27, 27, "name string"},
		"Box.Val":        {core.KindGoField, 32, 32, "Val T"},
		"Box.next":       {core.KindGoField, 33, 33, "next *Box[T]"},
		"Base.ID":        {core.KindGoField, 36, 36, "ID T"},
		"Store.Get":      {core.KindGoMethod, 40, 40, "func Get(key string) (string, error)"},
		"Store.Put":      {core.KindGoMethod, 41, 41, "func Put(key, value string) error"},
	}
	for name, w := range wants {
		n, ok := byName[name]
		if !ok {
			t.Errorf("no symbol %q", name)
			continue
		}
		owner := name[:len(name)-len(name[indexDot(name):])]
		if n.Kind != w.kind || n.Line != w.line || n.EndLine != w.end || n.Signature != w.sig || n.Package != owner ||
			n.FilePath != "conf/conf.go" || n.Service != "svc" || n.Tier != core.TierVerified || len(n.Calls) != 0 {
			t.Errorf("%s = {kind %s, line %d-%d, sig %q, package %q, calls %v}; want {kind %s, line %d-%d, sig %q, package %q, no calls}",
				name, n.Kind, n.Line, n.EndLine, n.Signature, n.Package, n.Calls, w.kind, w.line, w.end, w.sig, owner)
		}
	}
	// Not members: a blank field, a nested anonymous struct's own field, an
	// embedded interface (its methods are io.Closer's), a constraint's type
	// set, and an alias's struct literal.
	for _, name := range []string{"Config._", "Config.Depth", "Nested.Depth", "Config.Nested.Depth", "Store.Closer", "Store.Close", "Number.int", "Alias.X"} {
		if _, ok := byName[name]; ok {
			t.Errorf("%q is not a member symbol", name)
		}
	}
	// The declarations themselves are unchanged, and in source order.
	for name, kind := range map[string]core.SourceKind{
		"Config": core.KindGoType, "Server": core.KindGoType, "Box": core.KindGoType, "Base": core.KindGoType,
		"Store": core.KindGoInterface, "Number": core.KindGoInterface, "Alias": core.KindGoType, "Config.Validate": core.KindGoMethod,
	} {
		if n := byName[name]; n.Kind != kind {
			t.Errorf("%s kind %q, want %q", name, n.Kind, kind)
		}
	}
	for i := 1; i < len(nodes); i++ {
		a, b := nodes[i-1], nodes[i]
		if a.Line > b.Line || (a.Line == b.Line && a.Name > b.Name) {
			t.Fatalf("nodes out of (line, name) order: %s@%d before %s@%d", a.Name, a.Line, b.Name, b.Line)
		}
	}
}

func indexDot(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			return i
		}
	}
	return len(s)
}
