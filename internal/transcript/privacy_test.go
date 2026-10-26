package transcript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func writeTranscriptRound2(t *testing.T, name, text string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTranscriptMetadataHashesAndNoText(t *testing.T) {
	const cwd = "/home/example/work/private-project"
	const branch = "feature/private-project/customer-rollout"
	p := writeTranscriptRound2(t, "private-session-id.jsonl", `{"cwd":"`+cwd+`","gitBranch":"`+branch+`","type":"assistant","message":{"content":[{"type":"tool_use","id":"a","name":"Skill","input":{"skill":"private skill text /home/example/work","args":"private-arguments"}}]}}`+"\ninvalid-private-text\n")
	r := DecodeTranscriptEvents(p, "fixture-salt", 7)
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{cwd, branch, "private-session-id", "private skill text", "private-arguments", "invalid-private-text"} {
		if strings.Contains(string(encoded), secret) {
			t.Errorf("decoder retained %q", secret)
		}
	}
	if len(r.Events) != 3 {
		t.Fatalf("events = %+v", r.Events)
	}
	for _, e := range r.Events {
		if e.Cwd != "f31d8578b718cf56" || e.GitBranch != "734a13f9b8c3a78f" {
			t.Errorf("HMAC metadata = %q/%q", e.Cwd, e.GitBranch)
		}
	}
	if r.Events[2].Name != "skill" {
		t.Errorf("skill = %q", r.Events[2].Name)
	}
	if len(r.Issues) != 1 || r.Issues[0].File != "session:7" {
		t.Errorf("issues = %+v", r.Issues)
	}
	other := DecodeTranscriptEvents(p, "other-salt", 7)
	if other.Events[0].Cwd == r.Events[0].Cwd || other.Events[0].GitBranch == r.Events[0].GitBranch {
		t.Error("salt did not change metadata")
	}
	if !reflect.DeepEqual(r, DecodeTranscriptEvents(p, "fixture-salt", 7)) {
		t.Error("nondeterministic events")
	}
}

func TestTranscriptIssueFileLabels(t *testing.T) {
	for _, tc := range []struct{ name, label string }{
		{"session-secret.jsonl", "session:9"},
		{"subagents/agent-secret.jsonl", "subagent:9"},
		{"subagents/workflows/run-secret/agent-secret.jsonl", "subagent:9"},
	} {
		t.Run(tc.label+tc.name, func(t *testing.T) {
			p := writeTranscriptRound2(t, tc.name, "invalid\n")
			for _, missing := range []bool{false, true} {
				if missing {
					if err := os.Remove(p); err != nil {
						t.Fatal(err)
					}
				}
				r := DecodeTranscriptEvents(p, "fixture-salt", 9)
				if len(r.Issues) != 1 || r.Issues[0].File != tc.label {
					t.Errorf("issues = %+v", r.Issues)
				}
			}
		})
	}
}

func TestTranscriptInvalidToolInputsAreIssues(t *testing.T) {
	for _, tc := range []struct{ name, input, kind, shape string }{
		{"Bash", "", "invalid_tool_input", "invalid"},
		{"Bash", `,"input":1e1000`, "invalid_tool_input", "invalid"},
		{"Skill", `,"input":{"skill":42}`, "invalid_skill_input", "object{number}"},
		{"Skill", `,"input":[]`, "invalid_skill_input", "array[]"},
	} {
		t.Run(tc.name+tc.input, func(t *testing.T) {
			p := writeTranscriptRound2(t, "session.jsonl", `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"a","name":"`+tc.name+`"`+tc.input+`}]}}`)
			r := DecodeTranscriptEvents(p, "fixture-salt", 2)
			if r.MalformedLines != 1 || len(r.Issues) != 1 || r.Issues[0] != (Issue{File: "session:2", Line: 1, Kind: tc.kind}) {
				t.Errorf("coverage = %+v", r)
			}
			uses := ofKind(r, "tool_use")
			if len(uses) != 1 || uses[0].Shape != tc.shape {
				t.Errorf("uses = %+v", uses)
			}
			if tc.name == "Skill" && ofKind(r, "skill")[0].Name != "skill" {
				t.Error("unsafe skill name")
			}
		})
	}
}

func TestTranscriptSkillNamesRestricted(t *testing.T) {
	for _, name := range []string{"intent-design", "a", "plugin:skill_v2", strings.Repeat("a", 64), "", "UPPER", "name with text", "/home/example/work", strings.Repeat("a", 65), "é"} {
		t.Run(name, func(t *testing.T) {
			in, _ := json.Marshal(map[string]string{"skill": name})
			p := writeTranscriptRound2(t, "session.jsonl", `{"message":{"content":[{"type":"tool_use","name":"Skill","input":`+string(in)+`}]}}`)
			r := DecodeTranscriptEvents(p, "fixture-salt", 0)
			want := "skill"
			if regexp.MustCompile(`^[a-z0-9][a-z0-9:_-]{0,63}$`).MatchString(name) {
				want = name
			}
			if skills := ofKind(r, "skill"); len(skills) != 1 || skills[0].Name != want {
				t.Errorf("skills = %+v, want %q", skills, want)
			}
		})
	}
}

func TestTranscriptBlankCRLFLinesAreNoOp(t *testing.T) {
	for _, blank := range []string{"", " ", "\t", "\r\n", " \t\r\n"} {
		l, b, err := DecodeLine([]byte(blank))
		if err != nil || !reflect.DeepEqual(l, Line{}) || len(b) != 0 {
			t.Errorf("blank %q = %+v/%+v/%v", blank, l, b, err)
		}
	}
	p := writeTranscriptRound2(t, "session.jsonl", "\n \t\r\n{\"type\":\"user\"}\r\nnot-json\r\n")
	r := DecodeTranscriptEvents(p, "fixture-salt", 0)
	if r.MalformedLines != 1 || len(r.Events) != 1 || r.Events[0].Line != 3 || len(r.Issues) != 1 || r.Issues[0].Line != 4 {
		t.Errorf("reading = %+v", r)
	}
}

func TestTranscriptInvalidEnvelopeHasNoPartialMetadata(t *testing.T) {
	l, blocks, err := DecodeLine([]byte(`{"type":"assistant","timestamp":42,"message":{"id":"partial","content":[]}}`))
	if err == nil || !reflect.DeepEqual(l, Line{}) || len(blocks) != 0 {
		t.Fatalf("invalid envelope retained partial metadata: %+v/%+v/%v", l, blocks, err)
	}
}
