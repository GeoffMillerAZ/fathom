// Package cue adds CUE and Timoni extraction to a lang.Reader. CUE is its own
// package because reading it links cuelang.org/go, which a consumer that does
// not read CUE should not pay for (ADR-092). Without this package a .cue file
// is skipped with lang.SkipNotLinked, and Reader.Dialects leaves the CUE
// dialects out.
//
//	import "github.com/geoffmilleraz/fathom/pkg/lang/cue"
//
//	r, err := lang.New(cue.Option())
package cue

import (
	"github.com/geoffmilleraz/fathom/internal/extractor/cuelang"
	"github.com/geoffmilleraz/fathom/internal/langlink"
	"github.com/geoffmilleraz/fathom/internal/langroute"
	"github.com/geoffmilleraz/fathom/pkg/lang"
)

func init() {
	langlink.Register(langlink.Family{
		Name: langlink.CUE,
		Options: func() []langroute.Option {
			return []langroute.Option{
				langroute.WithCUE(cuelang.NewCUEExtractor()),
				langroute.WithTimoni(cuelang.NewTimoniExtractor()),
			}
		},
	})
}

// Option links CUE and Timoni extraction into the Reader it is passed to.
func Option() lang.Option { return lang.Link(langlink.CUE) }
