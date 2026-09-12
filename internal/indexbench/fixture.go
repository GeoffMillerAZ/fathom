package indexbench

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// FauxtownGoServices are the fixture fleet's Go services, in the order the
// report lists them. polyglot-service is excluded on purpose: it carries a
// go.mod but not one .go file, so go/types has nothing to type-check and an
// oracle over it would score an empty truth against an empty answer.
// worktree-service IS included even though it declares no functions — its
// zero denominators are the fixture that proves the report says "measured
// nothing" out loud instead of printing a flattering 1.0 with no context.
var FauxtownGoServices = []string{"ledger-service", "payment-service", "worktree-service"}

// FauxtownRoot is the absolute path of the fixture fleet, resolved from this
// source file so it is stable no matter which directory the test runs in.
func FauxtownRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		wd, _ := os.Getwd()
		return filepath.Join(wd, "..", "connector", "testdata", "fauxtown")
	}
	return filepath.Join(filepath.Dir(file), "..", "connector", "testdata", "fauxtown")
}

// NewFauxtownIndex ingests every Go service of the fixture fleet into a
// fresh in-memory store and returns the client over it — the same
// FauxConnector + MemoryStore construction internal/fusion and pkg/client
// tests use, so the index under test is built exactly the way the rest of
// the suite builds it.
func NewFauxtownIndex(ctx context.Context) (*client.Client, *store.MemoryStore, error) {
	st := store.NewMemoryStore()
	c := client.New(connector.NewFauxConnector(), st)
	for _, svc := range FauxtownGoServices {
		if _, err := c.Ingest(ctx, core.ServiceRef{ID: svc, Path: svc}); err != nil {
			return nil, nil, fmt.Errorf("indexbench: ingest %s: %w", svc, err)
		}
		// The pipeline records the corpus root ABSOLUTE, resolved against
		// the ingesting process's working directory — correct for a real
		// local repo, wrong for a fixture service whose path is a name the
		// FauxConnector resolves against its own root. Empty is the honest
		// value for a service with no local corpus root (the same rule
		// internal/fusion's TestIngest_ServiceMetaPathEmptyStaysEmpty
		// pins), and it is what makes Sources() and ObligationsIndexStale()
		// answer the same way from any working directory — which a bench
		// whose numbers are committed as goldens cannot do without.
		meta, found, err := st.GetServiceMeta(ctx, svc)
		if err != nil || !found {
			return nil, nil, fmt.Errorf("indexbench: service meta %s: found=%v err=%w", svc, found, err)
		}
		meta.Path = ""
		if err := st.SetServiceMeta(ctx, meta); err != nil {
			return nil, nil, fmt.Errorf("indexbench: reset meta %s: %w", svc, err)
		}
	}
	return c, st, nil
}

// StageFauxtownModule copies one fixture service's Go sources into dir and
// writes a go.mod beside them, returning dir.
//
// The fixture services are deliberately not modules — they are fragments a
// connector walks, and adding a go.mod to each would change what every other
// fauxtown test discovers. go/types, though, loads a MODULE. Staging a copy
// under the caller's temp dir keeps both facts true and keeps the oracle
// hermetic: the staged module imports nothing but the standard library, so
// the load never reaches the network or the module cache.
func StageFauxtownModule(svc, dir string) (string, error) {
	src := filepath.Join(FauxtownRoot(), svc)
	entries, err := os.ReadDir(src)
	if err != nil {
		return "", fmt.Errorf("indexbench: stage %s: %w", svc, err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("indexbench: stage %s: %w", svc, err)
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		data, rErr := os.ReadFile(filepath.Join(src, e.Name()))
		if rErr != nil {
			return "", fmt.Errorf("indexbench: stage %s/%s: %w", svc, e.Name(), rErr)
		}
		if wErr := os.WriteFile(filepath.Join(dir, e.Name()), data, 0o644); wErr != nil {
			return "", fmt.Errorf("indexbench: stage %s/%s: %w", svc, e.Name(), wErr)
		}
		n++
	}
	if n == 0 {
		return "", fmt.Errorf("indexbench: stage %s: no .go files", svc)
	}
	// The go directive is deliberately old and fixed: the staged module must
	// type-check identically on any toolchain a contributor happens to have,
	// and nothing in the fixture uses a modern language feature.
	gomod := "module fauxtown.test/" + svc + "\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		return "", fmt.Errorf("indexbench: stage %s: %w", svc, err)
	}
	return dir, nil
}

// FauxtownSubjects stages every fixture Go service under baseDir and returns
// the subjects to score. baseDir is the caller's temp dir — nothing is
// written inside the repository tree.
func FauxtownSubjects(baseDir string) ([]Subject, error) {
	subs := make([]Subject, 0, len(FauxtownGoServices))
	for _, svc := range FauxtownGoServices {
		root, err := StageFauxtownModule(svc, filepath.Join(baseDir, svc))
		if err != nil {
			return nil, err
		}
		// The FauxConnector reports paths relative to the FLEET root, so a
		// truth path of "payment.go" is "payment-service/payment.go" in the
		// index.
		subs = append(subs, Subject{Service: svc, Root: root, Prefix: svc + "/"})
	}
	return subs, nil
}
