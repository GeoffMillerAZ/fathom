package certify

// incident_certificates.go — IncidentCertificates (TC-19, design doc §5):
// an incident's own "Outcome:" line may name "certificate <Test>" — an
// alternate closing path independent of the certifications layer's usual
// "declare a marker naming kind=incident" route (TC-15's own convention).
// Gaps wires this in: an incident whose named test has a live
// certifications-layer record (for ANY subject — the test is the
// incident's own regression guard, not necessarily declared against the
// incident itself) is treated as closed even with nothing declared for
// "incident:<ID>" directly.
//
// The heading/Outcome grammar mirrored here is the SAME one
// internal/intent's own audit (U6, internal/intent/audit.go) reads;
// certify does not depend on that file's unexported walk, so this is a
// second, independent reader of the same on-disk grammar (docs/intent/
// incidents.md), exactly like incidentHeadingRe (resolve.go) is already a
// second reader of intent's own incidentRe.
import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// incidentOutcomeCertificateRe matches "certificate <Test>" inside an
// Outcome: line's own text — the same bare Go-identifier grammar a
// `// certifies:` marker's test name uses, so a name copied from one
// grammar into the other always matches.
var incidentOutcomeCertificateRe = regexp.MustCompile(`\bcertificate\s+([A-Za-z_][A-Za-z0-9_]*)\b`)

// IncidentCertificates parses corpusRoot's docs/intent/incidents.md
// (when present) for the "### INC-N" / "Outcome: … certificate <Test>"
// grammar and returns, for every incident id the file names, the
// certificate test its LATEST Outcome: line names — "" when the outcome
// text uses no such grammar (e.g. "pending", or a plain-prose fix with no
// named regression test). An incident with more than one "Outcome:" line
// takes the last one, the SAME "last one wins" rule internal/intent's own
// audit walk uses. A missing or unreadable file yields an empty map — the
// SAME "nothing to report" honesty incidentIDs (gaps.go) already carries
// for an absent docs/intent/.
func IncidentCertificates(corpusRoot string) map[string]string {
	out := map[string]string{}
	body, err := os.ReadFile(filepath.Join(corpusRoot, "docs", "intent", "incidents.md"))
	if err != nil {
		return out
	}
	var cur string
	for _, line := range strings.Split(string(body), "\n") {
		t := strings.TrimSpace(line)
		if m := incidentHeadingRe.FindStringSubmatch(t); m != nil {
			cur = m[1]
			if _, ok := out[cur]; !ok {
				out[cur] = ""
			}
			continue
		}
		if cur == "" || !strings.HasPrefix(t, "Outcome:") {
			continue
		}
		if m := incidentOutcomeCertificateRe.FindStringSubmatch(t); m != nil {
			out[cur] = m[1]
		}
	}
	return out
}
