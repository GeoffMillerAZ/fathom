package labs

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/research/bundle"
	"github.com/geoffmilleraz/fathom/internal/transcript"
)

func transcriptFixture(t *testing.T, parts ...string) transcript.Result {
	t.Helper()
	path := filepath.Join(append([]string{"..", "transcript", "testdata"}, parts...)...)
	return transcript.DecodeTranscriptEvents(path, "fixture-salt", 0)
}

func callNames(points []Point) []string {
	names := make([]string, 0, len(points))
	for _, p := range points {
		names = append(names, p.Call.Name)
	}
	return names
}

func TestExtractDecisionPointsCutsCallsInOrder(t *testing.T) {
	points, counts := ExtractDecisionPoints(transcriptFixture(t, "events.jsonl"), "fixture-salt", "session-1")
	if got, want := callNames(points), []string{"Bash", "intent-design", "Agent"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("calls: %v, want %v", got, want)
	}
	if counts.Points != 3 {
		t.Fatalf("counts.Points = %d, want 3", counts.Points)
	}
	session := bundle.HashService("fixture-salt", "session-1")
	for i, p := range points {
		if p.Session != session {
			t.Fatalf("point %d session = %q, want the salted hash", i, p.Session)
		}
		if p.Ordinal != i {
			t.Fatalf("point %d ordinal = %d", i, p.Ordinal)
		}
	}
	if len(points[0].Preceding) != 0 {
		t.Fatalf("first point preceded by %+v, want none", points[0].Preceding)
	}
	if got := len(points[1].Preceding); got != 1 || points[1].Preceding[0].Name != "Bash" {
		t.Fatalf("second point preceding: %+v", points[1].Preceding)
	}
	if got, want := []string{points[0].Followers[0].Name, points[0].Followers[1].Name}, []string{"intent-design", "Agent"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("followers: %v, want %v", got, want)
	}
}

// The decoder emits a Skill tool_use beside its skill event, and a Task or
// Agent tool_use beside its subagent_start: one choice, counted once.
func TestExtractDecisionPointsCountsASkillOnce(t *testing.T) {
	points, counts := ExtractDecisionPoints(transcriptFixture(t, "events.jsonl"), "fixture-salt", "session-1")
	for _, name := range []string{"Skill", "Task"} {
		for _, p := range points {
			if p.Call.Name == name {
				t.Fatalf("paired %s tool_use kept as a decision point: %+v", name, p.Call)
			}
		}
	}
	skills, agents := 0, 0
	for _, p := range points {
		switch p.Call.Kind {
		case "skill":
			skills++
		case "subagent_start":
			agents++
		}
	}
	if skills != 1 || agents != 1 || counts.Points != 3 {
		t.Fatalf("skills = %d, subagents = %d, points = %d; want 1, 1, 3", skills, agents, counts.Points)
	}
}

func TestExtractDecisionPointsKeepsFewerThanThreeFollowers(t *testing.T) {
	points, _ := ExtractDecisionPoints(transcriptFixture(t, "events.jsonl"), "fixture-salt", "session-1")
	last := points[len(points)-1]
	if len(last.Followers) != 0 {
		t.Fatalf("last point followers: %+v", last.Followers)
	}
	if got := len(points[1].Followers); got != 1 {
		t.Fatalf("second point followers = %d, want 1", got)
	}
}

// The subagent fixture is a prompt-only transcript: boundary events alone
// are no decision points.
func TestExtractDecisionPointsIgnoresBareBoundaries(t *testing.T) {
	points, counts := ExtractDecisionPoints(transcriptFixture(t, "subagents", "agent-fixture.jsonl"), "fixture-salt", "session-1")
	if len(points) != 0 || counts.Points != 0 {
		t.Fatalf("points from a prompt-only subagent: %+v %+v", points, counts)
	}
}

func TestExtractDecisionPointsJoinsResults(t *testing.T) {
	points, _ := ExtractDecisionPoints(transcriptFixture(t, "events.jsonl"), "fixture-salt", "session-1")
	if r := points[0].Call.Result; !r.Present || !r.IsError || r.Bytes != 5 {
		t.Fatalf("Bash result: %+v", r)
	}
	if r := points[2].Call.Result; !r.Present || r.IsError || r.Bytes != 6 {
		t.Fatalf("Agent result: %+v", r)
	}
	if r := points[1].Call.Result; r.Present {
		t.Fatalf("skill result: %+v, want none", r)
	}
}

// I38: every count the decoder names survives extraction.
func TestExtractDecisionPointsCarriesDecoderCounts(t *testing.T) {
	malformed := transcriptFixture(t, "malformed.jsonl")
	_, counts := ExtractDecisionPoints(malformed, "fixture-salt", "session-1")
	if counts.MalformedLines != malformed.MalformedLines || len(counts.Issues) != len(malformed.Issues) {
		t.Fatalf("malformed carried: %+v, want %d lines and %d issues", counts, malformed.MalformedLines, len(malformed.Issues))
	}
	unreadable := transcript.DecodeTranscriptEvents(filepath.Join("..", "transcript", "testdata"), "fixture-salt", 0)
	_, counts = ExtractDecisionPoints(unreadable, "fixture-salt", "session-1")
	if counts.UnreadableFiles != 1 || len(counts.Issues) != 1 || counts.Issues[0].Kind != "unreadable_file" {
		t.Fatalf("unreadable carried: %+v", counts)
	}
	duplicates := transcriptFixture(t, "duplicates.jsonl")
	_, counts = ExtractDecisionPoints(duplicates, "fixture-salt", "session-1")
	if counts.Duplicates != duplicates.Duplicates || counts.Duplicates == 0 {
		t.Fatalf("duplicates carried: %+v", counts)
	}
}

func TestDecisionPointsCarryNoText(t *testing.T) {
	points, _ := ExtractDecisionPoints(transcriptFixture(t, "events.jsonl"), "fixture-salt", "session-1")
	rendered := fmt.Sprintf("%#v", points)
	for _, secret := range []string{"sk-fixture-secret-prompt-12345", "private-command", "private-output", "private-task", "session-1"} {
		if strings.Contains(rendered, secret) {
			t.Fatalf("decision point retained %q", secret)
		}
	}
}
