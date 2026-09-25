// fathom's own Review Kit project config (ADR-0096 D5). CV-44 (vision 34 /
// ADR-044 decision 7; ADR-043's 2026-09-24 amendment): arms the
// refactoring-conservation and testing-certifications packs at warn on this
// repository. No org-specific catalog beyond the local, empty starter
// Standard below (ADR-011 / I19) — every requirement this repo evaluates
// comes from the two installed packs.
package reviewkit

config: #ProjectConfig & {
	subject: {
		service: "fathom"
	}
	catalogs: [
		{name: "local", dir: ".reviewkit/catalog"},
	]
	packs: [
		"refactoring-conservation",
		"testing-certifications",
	]
	out: "docs/review"
}
