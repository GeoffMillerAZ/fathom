package fusion

import "github.com/geoffmilleraz/fathom/internal/connector"

// binaryExcluded is fusion's one binary rule (ADR-021 decision (1), amended 2026-10-24):
// NUL-sniffed content is never extracted, except an OPC zip model container whose
// extension a dialect manifest's match.container claims (a Simulink .slx), which its
// dialect reads in memory under its own caps. Ingest, the one-file refresh, the
// harness and every cross-file pass ask this, never connector.IsBinary directly
// (TestBinaryGateIsTheOnlySniff), so the exemption is one rule.
func binaryExcluded(relPath string, content []byte) bool {
	return binaryExcludedWith(dialectRules(), relPath, content)
}

func binaryExcludedWith(rules []connector.DialectRule, relPath string, content []byte) bool {
	return connector.IsBinary(content) && !connector.ClaimedModelContainer(rules, relPath, content)
}
