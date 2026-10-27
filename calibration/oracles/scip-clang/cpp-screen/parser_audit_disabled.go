//go:build !fathom_grammar_audit

package main

import (
	"context"
	"errors"
	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

type auditCandidate struct {
	ModuleSHA256, GrammarVersion, RuntimeVersion string
	GrammarArchiveSHA256, RuntimeArchiveSHA256   string
}

func loadAuditCandidate(config) (*auditCandidate, error) {
	return nil, errors.New("external C++ parser requires fathom_grammar_audit build tag")
}

func openAuditParser(context.Context, *auditCandidate) (*tsengine.Parser, error) {
	return nil, errors.New("external C++ parser requires fathom_grammar_audit build tag")
}
