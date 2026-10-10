package fusion

import (
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// Late YAML dialects (wave c1, the c1-hub contract): YAML dialects whose files
// claim a name or a content shape, read only after every earlier classifier
// signal has declined the file. The table is ordered; a file belongs to the
// first dialect that holds all of: the dialect's extractor loads, the path is
// not a workflow, the content is not a Kubernetes object, the manifest's
// basename/glob or the spec's content sniff claims it, and the file decodes to
// at least one root. A dialect whose manifest is not in the build yields a nil
// extractor and claims nothing, so this file is inert until the dialects land.

const (
	// ToolPrometheus tags observations from a Prometheus configuration or rule file.
	ToolPrometheus ToolKind = "prometheus"
	// ToolOTelCollector tags observations from an OpenTelemetry Collector configuration.
	ToolOTelCollector ToolKind = "otel-collector"
	// ToolAlertmanager tags observations from an Alertmanager configuration
	// (LB-Q-alertmanager, wave c3).
	ToolAlertmanager ToolKind = "alertmanager"
	// ToolGrafanaDashboard tags observations from a Grafana dashboard or a
	// Grafana provisioning file (LB-Q-grafana-dashboard, wave c3).
	ToolGrafanaDashboard ToolKind = "grafana-dashboard"
)

// lateYAMLDialects is the ordered table of late-claimed YAML dialects.
// grafana-dashboard and alertmanager come before prometheus (wave c3,
// LB-F365): a Grafana alerting provisioning file has a top-level groups of
// rules too, and a rule written with an `alert:` key anywhere would meet
// prometheus' sniff; neither sniff claims a Prometheus file (an
// `apiVersion: 1` section file, a top-level route and receivers).
var lateYAMLDialects = []struct {
	id   string
	kind ToolKind
}{
	{"grafana-dashboard", ToolGrafanaDashboard},
	{"alertmanager", ToolAlertmanager},
	{"prometheus", ToolPrometheus},
	{"otel-collector", ToolOTelCollector},
}

// isLateYAMLKind reports whether kind is a late-claimed YAML dialect's
// (classify.go's ContentDialect names every one).
func isLateYAMLKind(kind ToolKind) bool {
	for _, d := range lateYAMLDialects {
		if d.kind == kind {
			return true
		}
	}
	return false
}

var (
	lateYAMLMu    sync.Mutex
	lateYAMLCache = map[string]*extractor.DocExtractor{}
)

// lateYAMLLoad builds (once per id) the dialect's document engine; nil when the
// manifest does not load.
func lateYAMLLoad(id string) *extractor.DocExtractor {
	lateYAMLMu.Lock()
	defer lateYAMLMu.Unlock()
	if x, done := lateYAMLCache[id]; done {
		return x
	}
	var out *extractor.DocExtractor
	if x, err := extractor.NewDocDialectExtractor(id); err == nil {
		out = x
	}
	lateYAMLCache[id] = out
	return out
}

// lateYAMLLookup resolves a dialect id to its extractor. A variable so a test
// can inject a testdata dialect.
var lateYAMLLookup = lateYAMLLoad

// lateYAMLExtractor is the extractor for a late kind, nil when kind is not a
// late kind or its dialect is absent.
func lateYAMLExtractor(kind ToolKind) extractor.Extractor {
	for _, d := range lateYAMLDialects {
		if d.kind == kind {
			if x := lateYAMLLookup(d.id); x != nil {
				return x
			}
			return nil
		}
	}
	return nil
}

// lateYAMLDetects returns the late kind that claims the YAML file, or "".
func lateYAMLDetects(relPath string, content []byte) ToolKind {
	slash := strings.TrimPrefix(path.Clean(strings.ReplaceAll(relPath, `\`, "/")), "./")
	if strings.Contains(slash, ".github/workflows/") {
		return ""
	}
	if apiVersionTopRe.Match(content) && kindTopRe.Match(content) {
		return "" // a Kubernetes object
	}
	for _, d := range lateYAMLDialects {
		x := lateYAMLLookup(d.id)
		if x == nil {
			continue
		}
		byPath := connector.ClassifyDialect(dialectRules(), relPath, content).Dialect == d.id
		if !byPath && !x.DetectsContent(content) {
			continue
		}
		res, err := x.ExtractDoc("", filepath.ToSlash(relPath), content)
		if err != nil || len(res.Roots) == 0 {
			continue
		}
		return d.kind
	}
	return ""
}
