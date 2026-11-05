package fusion

import (
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// ─── cross-refs, cheap tier (ADR-021 §2 / TI-7) ────────────────────────────

// imageBaseName extracts the trailing repository-name segment of a
// container image reference, stripping any registry/path prefix and
// tag/digest suffix — deliberately naive, no registry resolution:
// "ghcr.io/org/myapp:1.2.3" → "myapp"; "myapp@sha256:…" → "myapp";
// "myapp" → "myapp".
func imageBaseName(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if i := strings.LastIndex(ref, "/"); i >= 0 {
		ref = ref[i+1:]
	}
	if i := strings.IndexAny(ref, ":@"); i >= 0 {
		ref = ref[:i]
	}
	return ref
}

// crossRefImageToDockerfile appends a KindImageDockerfileRef reference node
// for every K8sContainer/ComposeService node in nodes whose image name
// matches a Dockerfile found anywhere in the same ingest (TI-7's cheap
// tier: a same-repo name match, no registry resolution, no build-context
// evaluation — Tier is always TierPreScan). Resolution order (critic F3):
//
//  1. *.dockerfile STEM match — the file names the image it builds.
//  2. bare Dockerfile parent-DIRECTORY-name match (dir-per-image layout).
//  3. Lone-root fallback: when the repo contains exactly ONE
//     Dockerfile-class file and it is the bare root Dockerfile, any image
//     ref resolves to it — the single-service-repo layout, where the root
//     Dockerfile is the only build there is. This fallback trades a little
//     precision for the most common layout of all (a public image like
//     postgres referenced in the same repo also matches); TierPreScan and
//     the "dockerfile:" signature keep the heuristic honest, and repos
//     with 2+ Dockerfiles never guess.
//
// Safe to call unconditionally after any extraction: it is a no-op when idx
// has no recorded Dockerfiles or nodes carries no image-bearing kind.
func crossRefImageToDockerfile(service, relPath string, nodes []core.SymbolNode, idx *siblingIndex) []core.SymbolNode {
	if idx.DockerfileCount() == 0 {
		return nil
	}
	var refs []core.SymbolNode
	seen := make(map[string]bool)
	for _, n := range nodes {
		var image string
		switch n.Kind {
		case core.KindK8sContainer:
			image = n.Name
		case core.KindComposeService:
			if !strings.HasPrefix(n.Signature, "build:") {
				image = n.Signature
			}
		default:
			continue
		}
		name := imageBaseName(image)
		if name == "" || seen[name] {
			continue
		}
		dockerfile, ok := idx.DockerfileFor(name)
		if !ok {
			continue
		}
		seen[name] = true
		refs = append(refs, core.SymbolNode{
			Service:   service,
			Name:      name,
			Kind:      core.KindImageDockerfileRef,
			FilePath:  relPath,
			Line:      n.Line,
			Signature: "dockerfile: " + dockerfile,
			Tier:      core.TierPreScan,
		})
	}
	return refs
}
