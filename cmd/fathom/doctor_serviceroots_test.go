package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// doctorRows runs `fathom doctor --json` against a fake daemon and returns
// the rows by name with the exit code.
func doctorRows(t *testing.T, cfg fakeDaemonCfg, extra ...string) (map[string]doctorCheck, int) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	svc := strings.ToLower(filepath.Base(dir))
	head := writeAllPassFixture(t, dir, svc)
	cfg.services = append([]string{svc}, cfg.services...)
	if cfg.obligRules == 0 {
		cfg.obligRules, cfg.watermark, cfg.agentCalls = 3, "2026-01-01T00:00:00Z", 5
	}
	srv := fakeDoctorDaemon(t, cfg)
	defer srv.Close()
	writeClaudeRegistration(t, home, srv.URL)
	old := doctorBinaryRevision
	defer func() { doctorBinaryRevision = old }()
	doctorBinaryRevision = func() (string, bool, bool) { return head, false, true }

	args := append([]string{"--path", dir, "--service", svc, "--url", srv.URL, "--harness", "claude", "--json"}, extra...)
	var out, errOut bytes.Buffer
	code := cmdDoctor(args, &out, &errOut)
	var report struct {
		Checks []doctorCheck `json:"checks"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("doctor output is not JSON: %v\n%s", err, out.String())
	}
	rows := map[string]doctorCheck{}
	for _, c := range report.Checks {
		rows[c.Name] = c
	}
	return rows, code
}

// TestDoctorNamesServiceWithMissingRoot is EW-20 (O127): a registered service
// whose recorded root no longer exists — a scratch clone that captured the
// service and was then deleted — is named by `fathom doctor`, with the ingest
// call that re-points it, instead of every answer silently resolving against
// a tree that is gone.
func TestDoctorNamesServiceWithMissingRoot(t *testing.T) {
	live := t.TempDir()
	gone := filepath.Join(t.TempDir(), "deleted-worker-clone")
	rows, _ := doctorRows(t, fakeDaemonCfg{
		services:     []string{"live-svc", "gone-svc"},
		servicePaths: map[string]string{"live-svc": live, "gone-svc": gone},
	})
	row, ok := rows["service_roots"]
	if !ok {
		t.Fatalf("doctor has no service_roots row: %v", rows)
	}
	if row.Status != doctorWarn {
		t.Fatalf("status = %v, want warn for another repository's service (%s)", row.Status, row.Reason)
	}
	for _, want := range []string{"gone-svc", gone} {
		if !strings.Contains(row.Reason, want) {
			t.Errorf("reason %q does not name %q", row.Reason, want)
		}
	}
	if strings.Contains(row.Reason, "live-svc") {
		t.Errorf("reason %q names a service whose root exists", row.Reason)
	}
	for _, want := range []string{"ingest", `"service":"gone-svc"`, `"remap":true`, `"path"`} {
		if !strings.Contains(row.Remedy, want) {
			t.Errorf("remedy %q does not carry %q, the call that re-points the service", row.Remedy, want)
		}
	}
}

// The doctor's own service with a deleted root is a FAIL, not a warning: this
// repository's index answers against a tree that is gone. The remedy fills
// in this repository's root as the path.
func TestDoctorFailsWhenItsOwnServiceRootIsGone(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	svc := strings.ToLower(filepath.Base(dir))
	head := writeAllPassFixture(t, dir, svc)
	gone := filepath.Join(t.TempDir(), "deleted-clone")
	srv := fakeDoctorDaemon(t, fakeDaemonCfg{services: []string{svc}, obligRules: 3, watermark: "2026-01-01T00:00:00Z", agentCalls: 5,
		servicePaths: map[string]string{svc: gone}})
	defer srv.Close()
	writeClaudeRegistration(t, home, srv.URL)
	old := doctorBinaryRevision
	defer func() { doctorBinaryRevision = old }()
	doctorBinaryRevision = func() (string, bool, bool) { return head, false, true }

	row := checkServiceRoots(&http.Client{Timeout: 2 * time.Second}, srv.URL, dir, svc, true)
	if row.Status != doctorFail {
		t.Fatalf("status = %v, want fail for this repository's own service (%s)", row.Status, row.Reason)
	}
	if !strings.Contains(row.Remedy, dir) || !strings.Contains(row.Remedy, `"remap":true`) {
		t.Errorf("remedy %q must re-point %s with remap:true", row.Remedy, dir)
	}
}

func TestDoctorServiceRootsPassWhenEveryRootExists(t *testing.T) {
	live := t.TempDir()
	rows, code := doctorRows(t, fakeDaemonCfg{services: []string{"live-svc"}, servicePaths: map[string]string{"live-svc": live}})
	if row := rows["service_roots"]; row.Status != doctorPass {
		t.Fatalf("status = %v, want pass (%s)", row.Status, row.Reason)
	}
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
}

// A service with no recorded root (list_services carries no path) is not a
// missing root; the row is skipped, never a guessed pass or a false failure.
func TestDoctorServiceRootsSkipWithoutPaths(t *testing.T) {
	rows, _ := doctorRows(t, fakeDaemonCfg{})
	if row := rows["service_roots"]; row.Status != doctorSkipped {
		t.Fatalf("status = %v, want skipped when no service names a root (%s)", row.Status, row.Reason)
	}
}

// A root that cannot be read (a wedged external volume answers EPERM) is not
// a root that is gone: the row names it as unreadable and offers no
// re-point, because re-pointing a service whose root merely cannot be read
// would move it off a tree that is fine (I38).
func TestDoctorServiceRootsDistinguishUnreadableFromGone(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads every directory")
	}
	parent := t.TempDir()
	locked := filepath.Join(parent, "locked")
	if err := os.MkdirAll(filepath.Join(locked, "repo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	rows, code := doctorRows(t, fakeDaemonCfg{
		services:     []string{"locked-svc"},
		servicePaths: map[string]string{"locked-svc": filepath.Join(locked, "repo")},
	})
	row := rows["service_roots"]
	if row.Status != doctorWarn || !strings.Contains(row.Reason, "locked-svc") || !strings.Contains(strings.ToLower(row.Reason), "unreadable") {
		t.Fatalf("row = %+v, want a warn naming locked-svc as unreadable", row)
	}
	if strings.Contains(row.Remedy, `"remap":true`) {
		t.Errorf("an unreadable root must not be offered a re-point: %q", row.Remedy)
	}
	if code != 0 {
		t.Errorf("exit = %d, want 0 (a warning)", code)
	}
}
