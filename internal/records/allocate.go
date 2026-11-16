package records

import (
	"slices"

	"github.com/geoffmilleraz/fathom/internal/recordids"
)

// AllocateRecordID selects the lowest unused id. Only a successful CLI write
// consumes it; callers must re-inventory and validate before the next allocation.
func AllocateRecordID(lane Lane, family, target string, inventory recordids.ValidatedInventory) (string, error) {
	if e := lane.Validate(); e != nil {
		return "", e
	}
	if !inventory.Valid() {
		return "", recordids.Refuse(recordids.ErrTrust, "invalid inventory")
	}
	grant, ok := inventory.Grant(lane.ID)
	if !ok || !slices.Equal(grant.OwnedPaths, lane.OwnedPaths) || !slices.Equal(grant.IDRanges, lane.IDRanges) {
		return "", recordids.Refuse(recordids.ErrTrust, "lane differs from validated grant")
	}
	if !recordids.CanonicalPath(target) || !slices.Contains(lane.OwnedPaths, target) {
		return "", recordids.Refuse(recordids.ErrTarget, "target is outside exact owned paths: "+target)
	}
	var selected IDRange
	count := 0
	for _, r := range lane.IDRanges {
		if string(r.Family) == family {
			selected = r
			count++
		}
	}
	if count != 1 {
		return "", recordids.Refuse(recordids.ErrShape, "family range is absent or ambiguous")
	}
	used := map[string]bool{}
	for _, d := range inventory.Definitions() {
		used[d.ID] = true
	}
	for n := selected.First; ; n++ {
		candidate := selected.Format(n)
		if !used[candidate] {
			return candidate, nil
		}
		if n == selected.Last {
			break
		}
	}
	return "", recordids.Refuse(recordids.ErrExhausted, "lane range exhausted for "+family)
}
