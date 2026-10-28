//go:build fathom_grammar_audit

package tsengine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

// GrammarAuditSpec is an explicit, private identity for one isolated C++ module.
// It never enters the embedded grammar lock, compiled cache, or parser pool.
type GrammarAuditSpec struct {
	Blob, EntryPoint, SHA256, Licence string
	Bytes                             int
	ABI                               uint32
}

const maxGrammarAuditBytes = 32 << 20

func NewGrammarAuditParser(ctx context.Context, raw []byte, spec GrammarAuditSpec) (*Parser, error) {
	if spec.Blob != "cpp" || spec.EntryPoint != "tree_sitter_cpp" || spec.Licence != "MIT" ||
		spec.Bytes <= 0 || spec.Bytes > maxGrammarAuditBytes || len(raw) != spec.Bytes || spec.ABI == 0 || len(spec.SHA256) != 64 {
		return nil, errors.New("tsengine: invalid C++ audit module identity")
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != spec.SHA256 {
		return nil, errors.New("tsengine: C++ audit module sha256 mismatch")
	}
	r, err := ensureRuntime(ctx)
	if err != nil {
		return nil, err
	}
	cm, err := r.CompileModule(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("tsengine: audit compile: %w", err)
	}
	defer cm.Close(ctx)
	mod, err := r.InstantiateModule(ctx, cm, wazeroModuleConfig())
	if err != nil {
		return nil, fmt.Errorf("tsengine: audit instantiate: %w", err)
	}
	liveModules.Add(1)
	ownedModules.Add(1)
	instantiations.Add(1)
	fail := func(e error) (*Parser, error) {
		_ = mod.Close(ctx)
		liveModules.Add(-1)
		ownedModules.Add(-1)
		return nil, e
	}
	p := &Parser{ctx: ctx, mod: mod, fns: make(map[string]*api.Function, len(requiredExports)+len(probeExports))}
	for _, name := range requiredExports {
		fn := mod.ExportedFunction(name)
		if fn == nil {
			return fail(fmt.Errorf("tsengine: audit module missing required export %q", name))
		}
		p.fns[name] = &fn
	}
	for _, name := range probeExports {
		if fn := mod.ExportedFunction(name); fn != nil {
			p.fns[name] = &fn
		}
	}
	entry := mod.ExportedFunction(spec.EntryPoint)
	if entry == nil {
		return fail(errors.New("tsengine: audit module missing C++ entry point"))
	}
	lang, err := entry.Call(ctx)
	if err != nil || len(lang) != 1 {
		return fail(errors.New("tsengine: audit C++ entry point failed"))
	}
	p.langPtr = uint32(lang[0])
	got, err := p.call("ts_language_abi_version", uint64(p.langPtr))
	if err != nil || len(got) != 1 || uint32(got[0]) != spec.ABI {
		return fail(errors.New("tsengine: audit language ABI mismatch"))
	}
	parser, err := p.call("ts_parser_new")
	if err != nil || len(parser) != 1 || parser[0] == 0 {
		return fail(errors.New("tsengine: audit parser creation failed"))
	}
	p.parserPtr = uint32(parser[0])
	ok, err := p.call("ts_parser_set_language", uint64(p.parserPtr), uint64(p.langPtr))
	if err != nil || len(ok) != 1 || ok[0] == 0 {
		return fail(errors.New("tsengine: audit language binding failed"))
	}
	return p, nil
}
