package adoption

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/geoffmilleraz/app-kit/appkit/catalog"
)

// CapabilityRevision binds complete catalog metadata, never implementation bytes.
type CapabilityRevision struct {
	ID     string `json:"id"`
	SHA256 string `json:"sha256"`
}

// PinReport is the version1 pins-command report. Existing inspect/check reports
// remain unchanged. Entries make candidate metadata available for explicit review.
type PinReport struct {
	Report
	CapabilityRevisions []CapabilityRevision `json:"capability_revisions"`
	Entries             []catalog.Entry      `json:"entries"`
}

func capabilityRevision(schemaVersion int, entry catalog.Entry) (CapabilityRevision, error) {
	// Field order, domain/version, full Entry and its array order are the v1 hash
	// contract. A future serialization change needs a separately versioned format.
	envelope := struct {
		Domain               string        `json:"domain"`
		Version              int           `json:"version"`
		CatalogSchemaVersion int           `json:"catalog_schema_version"`
		Entry                catalog.Entry `json:"entry"`
	}{"app-kit/catalog-entry", 1, schemaVersion, entry}
	body, err := json.Marshal(envelope)
	if err != nil {
		return CapabilityRevision{}, err
	}
	return CapabilityRevision{ID: entry.ID, SHA256: hash(body)}, nil
}

// Pins observes current metadata for explicitly selected IDs. It neither checks
// artifact/renderer fitness nor approves/migrates the consumer's authored pins.
func Pins(ctx context.Context, root, manifest string) PinReport {
	r, rd, c := open(ctx, root, "pins")
	p := PinReport{CapabilityRevisions: []CapabilityRevision{}, Entries: []catalog.Entry{}}
	if rd == nil {
		p.Report = r.finish()
		return p
	}
	defer rd.root.Close()
	s := r.selection(rd, manifest, true)
	if s != nil {
		for i, id := range s.Capabilities {
			key := fmt.Sprintf("capability-revision:%d", i+1)
			entry, ok := c.Get(id)
			if !ok {
				r.add(key, "fail", "", fmt.Sprintf("%q is absent from this catalog.", id), "Review the selected exact ID; no replacement is inferred.")
				continue
			}
			pin, err := capabilityRevision(c.SchemaVersion, entry)
			if err != nil {
				r.add(key, "unknown", "", "Selected metadata could not be encoded.", "Rebuild a validated checker; do not accept a missing pin.")
				continue
			}
			p.CapabilityRevisions = append(p.CapabilityRevisions, pin)
			p.Entries = append(p.Entries, entry)
			r.add(key, "pass", "", fmt.Sprintf("%q current metadata observed; no migration or adoption approval.", id), "")
		}
	}
	r.Limitations = append(r.Limitations, "Candidate pins cover catalog metadata only. Review selected records and actual artifacts/runtime evidence before an authorized manifest migration; this command checks neither authored pin agreement nor capability fitness.")
	p.Report = r.finish()
	return p
}

func (r *Report) checkCapabilityRevisions(c *catalog.Catalog, pins []CapabilityRevision) {
	r.add("catalog", "not-applicable", "", "Version2 selects exact capability metadata; whole-catalog revision is informational.", "")
	for i, expected := range pins {
		key := fmt.Sprintf("capability-revision:%d", i+1)
		entry, ok := c.Get(expected.ID)
		if !ok {
			r.add(key, "fail", "", fmt.Sprintf("%q selected metadata is absent.", expected.ID), "Review the missing selected capability; do not infer a replacement.")
			continue
		}
		actual, err := capabilityRevision(c.SchemaVersion, entry)
		if err != nil {
			r.add(key, "unknown", "", "Selected metadata could not be encoded.", "Rebuild a validated checker; do not accept a missing pin.")
		} else if actual.SHA256 != expected.SHA256 {
			r.add(key, "fail", "", fmt.Sprintf("%q selected metadata differs from its authored pin.", expected.ID), "Review this exact record and its evidence with adoption pins; never bless unexplained drift.")
		} else {
			r.add(key, "pass", "", fmt.Sprintf("%q metadata matches its authored pin; source/runtime identity is not proven.", expected.ID), "")
		}
	}
}
