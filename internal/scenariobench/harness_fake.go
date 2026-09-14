// FakeHarness (SB-15) replays committed stream transcripts
// (testdata/scenariobench/streams/*.jsonl) instead of shelling out to
// `claude -p`, so RunSessions' whole loop — single and planner-workers,
// cold sessions, box-check/plan-row reading — is testable with no spend.
package scenariobench

import (
	"fmt"
	"os"
	"path/filepath"
)

// FakeHarness maps a session request to a canned transcript. Streams is
// keyed by "<taskID>/<armName>/<role>" first, falling back to
// "<taskID>/<armName>" and then to Default — so a corpus of a few small
// synthetic streams can cover many sessions without one entry per turn
// index.
type FakeHarness struct {
	// StreamsDir holds the committed *.jsonl fixtures; Streams maps a
	// lookup key to a file name inside it (e.g. "T1/full" -> "t1-full.jsonl").
	StreamsDir string
	Streams    map[string]string
	// Default is used when no more specific key matches.
	Default string
	// ArmName is set by RunSessions before each call so the harness can
	// build its lookup key without SessionRequest carrying a task/arm
	// identity of its own (it does not — see types.go).
	ArmName string
	TaskID  string

	version string

	// MCPConfigPaths records req.MCPConfigPath for every RunSession call,
	// in order, so a test can assert the rendered config actually
	// propagated through RunSessions/newSessionRequest without shelling
	// out to a real harness.
	MCPConfigPaths []string
}

// NewFakeHarness returns a FakeHarness reporting version "fake-1" unless
// overridden.
func NewFakeHarness(streamsDir string, streams map[string]string, def string) *FakeHarness {
	return &FakeHarness{StreamsDir: streamsDir, Streams: streams, Default: def, version: "fake-1"}
}

// SetIdentity sets TaskID/ArmName — the (task, arm) this harness is about
// to run next. cmd/scenariobench's own resolveHarness instead constructs a
// fresh FakeHarness per cell with these preset; a caller that shares ONE
// harness value across many (task, arm) pairs (RunExternalCorpus) calls
// this before each RunSessions instead, via the HarnessIdentitySetter
// interface.
func (h *FakeHarness) SetIdentity(taskID, armName string) {
	h.TaskID = taskID
	h.ArmName = armName
}

func (h *FakeHarness) Name() string { return "fake" }

func (h *FakeHarness) Version() string {
	if h.version == "" {
		return "fake-1"
	}
	return h.version
}

// RunSession looks up the transcript for (TaskID, ArmName, req.Role), then
// (TaskID, ArmName), then Default, parses it exactly as ClaudeHarness
// would (ParseSessionStream), and stamps in the request's identity —
// exactly what a real harness does, so RunSessions cannot tell the two
// apart except by Name().
func (h *FakeHarness) RunSession(req SessionRequest) (SessionResult, error) {
	h.MCPConfigPaths = append(h.MCPConfigPaths, req.MCPConfigPath)
	name, err := h.resolveStream(req)
	if err != nil {
		return SessionResult{}, err
	}
	path := filepath.Join(h.StreamsDir, name)
	result, err := ParseSessionStream(path)
	if err != nil {
		return SessionResult{}, fmt.Errorf("fake harness: %w", err)
	}
	result.Index = req.Index
	result.Role = req.Role
	result.RowID = req.RowID
	result.RawPath = path
	result.FilesRead = relativizeSessionPaths(req.Worktree, result.FilesRead)
	result.FilesEdited = relativizeSessionPaths(req.Worktree, result.FilesEdited)
	return result, nil
}

func (h *FakeHarness) resolveStream(req SessionRequest) (string, error) {
	keys := []string{
		h.TaskID + "/" + h.ArmName + "/" + req.Role,
		h.TaskID + "/" + h.ArmName,
	}
	for _, k := range keys {
		if name, ok := h.Streams[k]; ok {
			return name, nil
		}
	}
	if h.Default != "" {
		return h.Default, nil
	}
	return "", fmt.Errorf("fake harness: no stream registered for %q (task %s, arm %s) and no default", req.Role, h.TaskID, h.ArmName)
}

// LoadFakeStream is a small helper for tests that want to assert on a
// fixture's shape directly rather than through RunSession.
func LoadFakeStream(dir, name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(dir, name))
}

var _ Harness = (*FakeHarness)(nil)
