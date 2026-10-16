package main

// verify_gone_evidence.go — CV-M6 tooling fix (ADR-044 §3.1, CV-27/CV-28):
// cliEvidence's verify.GoneEvidence, wiring `fathom verify --store` to prove
// (or honestly refuse to prove) a `gone:` row.
//
// The implementation is internal/verify/goneev, the SAME one the daemon's
// verify_plan builds (O1068: the daemon path had none and read every gone:
// row Unknown while --store proved it), so the two paths cannot drift. This
// file only says what the CLI hands it: the store's ingest record and
// persisted typed-pass status, the client as the corpus, the call graph,
// and the working tree the plan was read from (verifyRepoRoot) — which must
// be clean and at the ingested commit (the ordinary, hermetic sequence:
// commit, then ingest, then verify).

import (
	"github.com/geoffmilleraz/fathom/internal/verify"
	"github.com/geoffmilleraz/fathom/internal/verify/goneev"
)

// goneEvidence builds the shared gone: evidence once per process.
func (e *cliEvidence) goneEvidence() *goneev.Evidence {
	if e.gone != nil {
		return e.gone
	}
	cfg := goneev.Config{
		Service:    e.service,
		Plan:       goneev.ReadPlanTree(e.ctx, e.root),
		FindSymbol: e.FindSymbol,
		Graph:      e.graph,
		ByKey:      e.byKey,
	}
	if e.c != nil {
		cfg.CorpusRoot, cfg.IndexedCommit, cfg.MetaFound, cfg.MetaErr = e.c.ServiceMeta(e.ctx, e.service)
		cfg.Corpus = e.c
		cfg.Store = e.c.Store()
	}
	e.gone = goneev.New(e.ctx, cfg)
	return e.gone
}

// IndexAtHead implements verify.GoneEvidence (goneev.Evidence.IndexAtHead).
func (e *cliEvidence) IndexAtHead() (bool, string) { return e.goneEvidence().IndexAtHead() }

// FailedTypedPackages implements verify.GoneEvidence.
func (e *cliEvidence) FailedTypedPackages() []string {
	return e.goneEvidence().FailedTypedPackages()
}

// GoFiles implements verify.GoneEvidence.
func (e *cliEvidence) GoFiles() []verify.GoFile { return e.goneEvidence().GoFiles() }

// ModuleFile implements verify.GoneEvidence.
func (e *cliEvidence) ModuleFile() ([]byte, bool) { return e.goneEvidence().ModuleFile() }

// CorpusFiles implements verify.GoneEvidence.
func (e *cliEvidence) CorpusFiles(paths []string) []verify.CorpusFile {
	return e.goneEvidence().CorpusFiles(paths)
}

// TypedCallers implements verify.GoneEvidence.
func (e *cliEvidence) TypedCallers(symbol, pkg string) (nonTest, test []verify.GoneReferent) {
	return e.goneEvidence().TypedCallers(symbol, pkg)
}

// GoneReadFailures implements verify.GoneReadEvidence: a corpus file the
// CLI could not read is named, never scored as one without the target.
func (e *cliEvidence) GoneReadFailures() []string { return e.goneEvidence().GoneReadFailures() }
