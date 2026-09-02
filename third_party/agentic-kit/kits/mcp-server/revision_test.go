package mcpserver

import (
	"strings"
	"testing"
	"time"

	mcpclient "github.com/geoffmilleraz/agentic-kit/foundations/agent/mcp-client"
)

// floorDate is 2027-07-28, the twelve-month floor after the 2026-07-28 MCP
// revision. Pinned here as a literal so the table cannot drift silently.
var floorDate = time.Date(2027, time.July, 28, 0, 0, 0, 0, time.UTC)

func TestProtocolRevisionIsKnownAndLatest(t *testing.T) {
	if !knownProtocolVersions[ProtocolRevision] {
		t.Fatalf("ProtocolRevision %q is not in knownProtocolVersions", ProtocolRevision)
	}
	if latestProtocolVersion != ProtocolRevision {
		t.Fatalf("latestProtocolVersion = %q, want ProtocolRevision %q", latestProtocolVersion, ProtocolRevision)
	}
	for v := range knownProtocolVersions {
		if v > ProtocolRevision {
			t.Fatalf("knownProtocolVersions has %q newer than ProtocolRevision %q", v, ProtocolRevision)
		}
	}
}

func TestDeprecationsTableShape(t *testing.T) {
	table := Deprecations()
	if len(table) != 7 {
		t.Fatalf("Deprecations() has %d entries, want 7", len(table))
	}
	seen := map[string]bool{}
	still := map[string]bool{}
	for _, d := range table {
		if d.Feature == "" {
			t.Fatal("entry with empty Feature")
		}
		if seen[d.Feature] {
			t.Fatalf("duplicate feature %q", d.Feature)
		}
		seen[d.Feature] = true
		if d.DeprecatedIn.IsZero() {
			t.Fatalf("%s: DeprecatedIn is zero", d.Feature)
		}
		if d.Feature == "HTTP+SSE transport" {
			if !d.RemovableAfter.IsZero() {
				t.Fatalf("%s: registry publishes no removal date; RemovableAfter must stay zero", d.Feature)
			}
			continue
		}
		if !d.RemovableAfter.Equal(floorDate) {
			t.Fatalf("%s: RemovableAfter = %s, want %s", d.Feature, d.RemovableAfter.Format(time.RFC3339), floorDate.Format(time.RFC3339))
		}
		if !d.RemovableAfter.Equal(d.DeprecatedIn.AddDate(1, 0, 0)) {
			t.Fatalf("%s: floor is not DeprecatedIn + 12 months", d.Feature)
		}
		if d.StillImplemented {
			still[d.Feature] = true
		}
	}
	// Exactly what handlers.go speaks today. A new handler that revives a
	// retired feature must flip its row here, not silently.
	want := map[string]bool{"initialize handshake": true, "ping": true}
	if len(still) != len(want) {
		t.Fatalf("StillImplemented set = %v, want %v", still, want)
	}
	for f := range want {
		if !still[f] {
			t.Fatalf("%s: StillImplemented should be true", f)
		}
	}
}

// TestDeprecationFloorNotReached reads the WALL CLOCK on purpose — the one
// deliberate time.Now in this package's tests. It goes red on 2027-07-28,
// the day the initialize handshake and ping pass their twelve-month floor,
// and that failure is the reminder decision 0003 asks for. Do not pin the
// clock, skip, or build-tag it away: retire the feature or record a new
// decision with a new floor.
func TestDeprecationFloorNotReached(t *testing.T) {
	due := DeprecationsDue(time.Now())
	if len(due) == 0 {
		return
	}
	var names []string
	for _, d := range due {
		names = append(names, d.Feature+" (floor "+d.RemovableAfter.Format("2006-01-02")+")")
	}
	t.Fatalf("mcp-server still implements MCP features past their removal floor: %s — "+
		"decision 0003 fired; retire the feature or record a new floor "+
		"(docs/intent/decisions/0003-mcp-revisions-track-never-chase.md)", strings.Join(names, ", "))
}

func TestDeprecationsDueAtFloor(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
		want []string
	}{
		{"day before floor", floorDate.AddDate(0, 0, -1), nil},
		{"on the floor", floorDate, []string{"initialize handshake", "ping"}},
		{"floor plus one day", floorDate.AddDate(0, 0, 1), []string{"initialize handshake", "ping"}},
		{"far future", floorDate.AddDate(5, 0, 0), []string{"initialize handshake", "ping"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			due := DeprecationsDue(tc.now)
			var got []string
			for _, d := range due {
				got = append(got, d.Feature)
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("DeprecationsDue(%s) = %v, want %v", tc.now.Format("2006-01-02"), got, tc.want)
			}
		})
	}
}

func TestDeprecationsDueUndatedFloorFailsClosed(t *testing.T) {
	table := []Deprecation{
		{Feature: "undated, implemented", DeprecatedIn: revision20250326, StillImplemented: true},
		{Feature: "undated, not implemented", DeprecatedIn: revision20250326, StillImplemented: false},
	}
	due := deprecationsDue(table, time.Date(2025, time.March, 27, 0, 0, 0, 0, time.UTC))
	if len(due) != 1 || due[0].Feature != "undated, implemented" {
		t.Fatalf("undated floor on a still-implemented feature must be due immediately; got %v", due)
	}
}

func TestDeprecationsReturnsCopy(t *testing.T) {
	a := Deprecations()
	a[0].Feature = "mutated"
	a[0].StillImplemented = false
	b := Deprecations()
	if b[0].Feature == "mutated" || !b[0].StillImplemented {
		t.Fatal("Deprecations() must return a fresh slice each call")
	}
}

// TestDeprecationsMatchClient pins the server and client ledgers to the same
// features and floors; only StillImplemented may differ between them.
func TestDeprecationsMatchClient(t *testing.T) {
	if mcpclient.ProtocolRevision != ProtocolRevision {
		t.Fatalf("client ProtocolRevision %q != server %q", mcpclient.ProtocolRevision, ProtocolRevision)
	}
	server := Deprecations()
	client := mcpclient.Deprecations()
	if len(server) != len(client) {
		t.Fatalf("server has %d entries, client %d", len(server), len(client))
	}
	for i := range server {
		s, c := server[i], client[i]
		if s.Feature != c.Feature || !s.DeprecatedIn.Equal(c.DeprecatedIn) || !s.RemovableAfter.Equal(c.RemovableAfter) {
			t.Fatalf("entry %d differs: server %+v, client %+v", i, s, c)
		}
	}
}
