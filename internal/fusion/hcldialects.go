package fusion

// hcldialects.go — the native HCL tool dialects' registration (LB-Q-terragrunt,
// LB-Q-hcl-tools; LB-F141). registerProxyDialects' counterpart: each manifest
// basename and extension (terragrunt.hcl, root.hcl, docker-bake.hcl, .nomad)
// is a p.exts key, so ingest and ReingestFile find it on their fast path; every
// other file a manifest claims, by glob (*.pkr.hcl, *.nomad.hcl,
// docker-bake*.hcl) or by content sniff (a .hcl whose top-level blocks are
// Terragrunt's, or whose top-level block is job), is dispatched by
// sniffedExtractor, which asks connector.ClassifyDialect exactly as the ingest
// gate does, so a .hcl no sniff claims (a Vault or Consul configuration, a
// .terraform.lock.hcl) is never extracted and is counted ambiguous.

import (
	"strings"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

func registerHCLDialects(p *Pipeline, ds []extractor.Dialect) {
	for _, d := range ds {
		e, ok := extractor.HCLToolDialectExtractor(d.ID)
		if d.Engine != "native" || !ok {
			continue
		}
		for _, k := range append(append([]string(nil), d.Match.Basenames...), d.Match.Extensions...) {
			if _, claimed := p.exts[k]; !claimed {
				p.exts[k] = e
			}
		}
		p.sniffExts[d.ID] = e
		if sn := d.Match.Sniff; sn != nil {
			for _, x := range sn.Extensions {
				p.sniffExt[strings.ToLower(x)] = true
			}
		}
	}
}
