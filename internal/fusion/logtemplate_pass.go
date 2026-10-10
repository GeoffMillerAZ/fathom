package fusion

// logtemplate_pass.go — TL-4 (F67; ADR-064; docs/design/telemetry-links.md
// "Log templates (TL-4)"): every logger call in a Python, Java,
// JavaScript/TypeScript or PHP file becomes a LogTemplate observation of
// that file (internal/logtemplate), so localize can take a pasted log line
// to the call that wrote it.
//
// Storage is the file's own observations, under the table freeze (no new
// table, column, cache or background job): a changed file's PurgeSource
// takes its templates with its symbols, a deleted file's orphan purge
// takes both, and ReingestFile writes them beside its symbols (parity). So
// a stored template always belongs to the content its file's fingerprint
// names, and the pass only ever adds templates to a file that holds none.
//
// Which files: those this ingest changed, read again after the extract
// stream wrote their symbols (the enclosing symbol is read from them).
// Every candidate is swept instead when no pass has completed for the
// service (a store from before TL-4, a pass that halted) or when the
// previous ingest never committed (it may have written a file's symbols
// and died before its templates). Its time is part of the "extract" pass.

import (
	"context"
	"fmt"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/logtemplate"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// logTemplateStateKey records a completed template pass (typedgate.go's
// pass-state rows, which no typed pass reconciles away).
const logTemplateStateKey = typedPassStatePrefix + "log-template"

// logTemplateFile is one candidate file's facts, read this pass.
type logTemplateFile struct {
	path      string
	hash      string
	generated bool
	facts     []logtemplate.Fact
}

// enrichLogTemplates is the Ingest hook for TL-4. Best-effort per file; a
// halt or a store failure is returned before the final commit, so the
// ingest stays in progress and the next one sweeps.
func (p *Pipeline) enrichLogTemplates(ctx context.Context, svc core.ServiceRef, run *ingestRun, refs []core.SourceRef, changed []string) error {
	sweep := !p.passStateOK(ctx, svc.ID, logTemplateStateKey) || (run != nil && run.hadPrev && run.prev.State == store.IngestInProgress)
	p.beginPassState(ctx, svc.ID, logTemplateStateKey)
	changedSet := make(map[string]bool, len(changed))
	for _, c := range changed {
		changedSet[c] = true
	}
	fis, canRead := p.st.(store.FileImportsStore)
	var files []logTemplateFile
	for _, r := range refs {
		if !logtemplate.Reads(r.Path) || !changedSet[r.Path] && !(sweep && canRead) {
			continue
		}
		content, err := p.conn.Fetch(ctx, r)
		if err != nil {
			if h := haltError(ctx, err); h != nil {
				return h
			}
			continue
		}
		f, ok, err := p.logTemplateFile(ctx, svc.ID, r.Path, content)
		if err != nil {
			return err
		}
		if ok {
			files = append(files, f)
		}
	}
	if len(files) > 0 {
		var nodes map[string][]core.SymbolNode
		var holds map[string]bool
		if canRead {
			var err error
			if nodes, holds, err = logTemplateNodes(ctx, fis, svc.ID, files); err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		var obs []core.Observation
		for _, f := range files {
			if holds[f.path] {
				continue // its templates were written with its current content
			}
			logtemplate.Enclose(f.facts, nodes[f.path])
			for _, fact := range f.facts {
				obs = append(obs, logtemplate.ToObservation(svc.ID, fact, f.hash, now, f.generated))
			}
		}
		if len(obs) > 0 {
			if err := p.st.PutObservations(ctx, obs); err != nil {
				return fmt.Errorf("fusion: log-template pass %q: %w", svc.ID, err)
			}
		}
	}
	p.commitPassState(ctx, svc.ID, logTemplateStateKey)
	return nil
}

// logTemplateFile reads one candidate's facts, when the store holds this
// very content (a file that changed again since the stream read it waits
// for the next ingest) and an ingest would extract it at all.
func (p *Pipeline) logTemplateFile(ctx context.Context, service, path string, content []byte) (logTemplateFile, bool, error) {
	if len(content) > MaxExtractSize || connector.IsBinary(content) || gateDialectFile(path, content).excluded != "" {
		return logTemplateFile{}, false, nil
	}
	hash := FingerprintHash(content)
	stored, found, err := p.st.Fingerprint(ctx, service, path)
	if err != nil {
		return logTemplateFile{}, false, fmt.Errorf("fusion: log-template pass fingerprint %q: %w", path, err)
	}
	if !found || stored != hash {
		return logTemplateFile{}, false, nil
	}
	facts := logtemplate.Read(path, content)
	if len(facts) == 0 {
		return logTemplateFile{}, false, nil
	}
	return logTemplateFile{path: path, hash: hash, generated: connector.IsGenerated(path, content), facts: facts}, true, nil
}

// logTemplateNodes reads the files' stored symbols (the enclosing-symbol
// candidates) and which files already hold templates.
func logTemplateNodes(ctx context.Context, fis store.FileImportsStore, service string, files []logTemplateFile) (map[string][]core.SymbolNode, map[string]bool, error) {
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.path
	}
	stored, err := fis.SourceObservations(ctx, service, paths)
	if err != nil {
		return nil, nil, fmt.Errorf("fusion: log-template pass symbols %q: %w", service, err)
	}
	nodes := map[string][]core.SymbolNode{}
	holds := map[string]bool{}
	for _, o := range stored {
		if o.SourceKind == logtemplate.Kind {
			holds[o.SourceRef] = true
			continue
		}
		nodes[o.SourceRef] = append(nodes[o.SourceRef], o.ToSymbolNode())
	}
	return nodes, holds, nil
}

// logTemplateObservations is ReingestFile's half (parity with the pass):
// the file's templates, enclosed by the symbols just extracted from it.
func logTemplateObservations(service, relFile, hash string, content []byte, nodes []core.SymbolNode, at time.Time, generated bool) []core.Observation {
	facts := logtemplate.Read(relFile, content)
	if len(facts) == 0 {
		return nil
	}
	logtemplate.Enclose(facts, nodes)
	out := make([]core.Observation, 0, len(facts))
	for _, f := range facts {
		out = append(out, logtemplate.ToObservation(service, f, hash, at, generated))
	}
	return out
}
