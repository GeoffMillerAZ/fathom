package tasktree

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"

	implementationqueue "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/coordination/implementation-queue"
	swarmorchestrator "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/coordination/swarm-orchestrator"
)

// TestCanonicalState_EveryEnumValue is table-driven over every value of the
// three divergent Go enums plus TaskState's own values, built from the real
// constants so a renamed constant fails here, not in a consumer.
func TestCanonicalState_EveryEnumValue(t *testing.T) {
	cases := []struct {
		src  string
		in   string
		want TaskState
	}{
		// tasktree.Status
		{"tasktree", string(StatusPending), StatePending},
		{"tasktree", string(StatusRunning), StateRunning},
		{"tasktree", string(StatusDone), StateCompleted},
		{"tasktree", string(StatusFailed), StateFailed},
		// swarmorchestrator.TaskStatus
		{"swarm-orchestrator", string(swarmorchestrator.TaskPending), StatePending},
		{"swarm-orchestrator", string(swarmorchestrator.TaskAssigned), StateRunning},
		{"swarm-orchestrator", string(swarmorchestrator.TaskDone), StateCompleted},
		{"swarm-orchestrator", string(swarmorchestrator.TaskFailed), StateFailed},
		// implementationqueue.ItemStatus
		{"implementation-queue", string(implementationqueue.StatusPending), StatePending},
		{"implementation-queue", string(implementationqueue.StatusInProgress), StateRunning},
		{"implementation-queue", string(implementationqueue.StatusDone), StateCompleted},
		{"implementation-queue", string(implementationqueue.StatusFailed), StateFailed},
		// TaskState is a fixed point
		{"tasktree.TaskState", string(StatePending), StatePending},
		{"tasktree.TaskState", string(StateRunning), StateRunning},
		{"tasktree.TaskState", string(StateInputRequired), StateInputRequired},
		{"tasktree.TaskState", string(StateCompleted), StateCompleted},
		{"tasktree.TaskState", string(StateFailed), StateFailed},
		{"tasktree.TaskState", string(StateCancelled), StateCancelled},
	}
	for _, tc := range cases {
		t.Run(tc.src+"/"+tc.in, func(t *testing.T) {
			got, err := CanonicalState(tc.in)
			if err != nil {
				t.Fatalf("CanonicalState(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("CanonicalState(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// enumLineRe matches a one-line CUE string enum such as
// `#TaskStatus: "pending" | "assigned" | "done" | "failed"`.
var enumLineRe = regexp.MustCompile(`(?m)^#(Status|TaskStatus|ItemStatus|TaskState):\s*(.+)$`)

// TestCanonicalState_CoversEverySchemaEnum reads the three sibling
// schema.cue files (CUE owns the shape) and asserts every enumerated status
// string maps, so an enum value added in CUE without a mapping fails here.
func TestCanonicalState_CoversEverySchemaEnum(t *testing.T) {
	files := []string{
		"schema.cue",
		"../swarm-orchestrator/schema.cue",
		"../implementation-queue/schema.cue",
	}
	found := 0
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for _, m := range enumLineRe.FindAllStringSubmatch(string(data), -1) {
			found++
			for _, alt := range strings.Split(m[2], "|") {
				v := strings.Trim(strings.TrimSpace(alt), `"`)
				if _, err := CanonicalState(v); err != nil {
					t.Errorf("%s #%s value %q has no canonical mapping: %v", f, m[1], v, err)
				}
			}
		}
	}
	if found < 4 {
		t.Fatalf("expected at least 4 enum lines across %v, found %d — regexp or file layout drifted", files, found)
	}
}

func TestCanonicalState_Unknown(t *testing.T) {
	for _, in := range []string{"", "Pending", " pending", "canceled", "blocked", "done!"} {
		got, err := CanonicalState(in)
		if !errors.Is(err, ErrUnknownState) {
			t.Fatalf("CanonicalState(%q) err = %v, want ErrUnknownState", in, err)
		}
		if got != "" {
			t.Fatalf("CanonicalState(%q) = %q, want zero TaskState on error", in, got)
		}
		if !strings.Contains(err.Error(), in) && in != "" {
			t.Fatalf("error %q does not name the offending value %q", err, in)
		}
	}
}

func TestTaskState_Classes(t *testing.T) {
	cases := []struct {
		s           TaskState
		terminal    bool
		interrupted bool
	}{
		{StatePending, false, false},
		{StateRunning, false, false},
		{StateInputRequired, false, true},
		{StateCompleted, true, false},
		{StateFailed, true, false},
		{StateCancelled, true, false},
	}
	for _, tc := range cases {
		if got := tc.s.Terminal(); got != tc.terminal {
			t.Errorf("%s.Terminal() = %v, want %v", tc.s, got, tc.terminal)
		}
		if got := tc.s.Interrupted(); got != tc.interrupted {
			t.Errorf("%s.Interrupted() = %v, want %v", tc.s, got, tc.interrupted)
		}
		if tc.terminal && tc.interrupted {
			t.Errorf("%s is both terminal and interrupted; the classes are disjoint", tc.s)
		}
	}
}
