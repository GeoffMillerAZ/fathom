package fusion

// register.go — New. The extractor registration table and the per-file dispatch
// live in internal/langroute (ADR-092, LL-4); a Pipeline embeds the Router that
// owns them and registers the product-only formats and the CUE family on it.

import (
	"fmt"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/extractor/cuelang"
	"github.com/geoffmilleraz/fathom/internal/fathomformat"
	"github.com/geoffmilleraz/fathom/internal/langroute"
	"github.com/geoffmilleraz/fathom/internal/store"
)

var (
	timoniOnce sync.Once
	timoniExt  extractor.Extractor
)

// timoniExtractor is the Timoni dialect's extractor, built once (it holds no
// state) and registered on every Pipeline's router.
func timoniExtractor() extractor.Extractor {
	timoniOnce.Do(func() { timoniExt = cuelang.NewTimoniExtractor() })
	return timoniExt
}

// New constructs a Pipeline with extractors pre-registered for all supported
// file types. Additional extractors can be added or replaced by calling
// RegisterExtractor after construction.
func New(conn connector.Connector, st store.Store, cfg Config) *Pipeline {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	if cfg.InflightBytes <= 0 {
		cfg.InflightBytes = DefaultInflightBytes
	}
	router, err := langroute.NewRouter(
		langroute.WithCUE(cuelang.NewCUEExtractor()),
		langroute.WithTimoni(timoniExtractor()),
		langroute.WithFormats(fathomformat.Dispatch),
	)
	if err != nil {
		panic(fmt.Sprintf("fusion: %v", err))
	}
	return &Pipeline{
		Router:         router,
		conn:           conn,
		st:             st,
		cfg:            cfg,
		queue:          map[string][]pendingFile{},
		siblingIdx:     map[string]*siblingIndex{},
		classifiedZero: map[string]int64{},
		gitHeadFn:      realGitHead,
		gitLogFn:       realGitLog,
	}
}

// resolveExtractor is ingest's per-file dispatch (langroute.Router.ResolveExtractor).
func (p *Pipeline) resolveExtractor(relPath string, content []byte, idx *siblingIndex) (e extractor.Extractor, toolKind ToolKind, ok, unclassified bool) {
	return p.Router.ResolveExtractor(relPath, content, idx)
}
