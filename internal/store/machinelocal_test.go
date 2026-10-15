package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// machineLocalSpecRow matches one bullet of store-format-v0.md's
// "Machine-local fields" list: "- `table.column`: ...".
var machineLocalSpecRow = regexp.MustCompile("^- `([a-z_]+\\.[a-z_]+)`: ")

// specMachineLocalFields scrapes the spec's machine-local list, sorted.
func specMachineLocalFields(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "spec", "store-format-v0.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(data)
	start := strings.Index(doc, "**Machine-local fields.**")
	if start < 0 {
		t.Fatal("store-format-v0.md has no **Machine-local fields.** paragraph")
	}
	rest := doc[start:]
	if end := strings.Index(rest, "\n#"); end >= 0 {
		rest = rest[:end]
	}
	var out []string
	for _, line := range strings.Split(rest, "\n") {
		if m := machineLocalSpecRow.FindStringSubmatch(line); m != nil {
			out = append(out, m[1])
		}
	}
	sort.Strings(out)
	return out
}

// TestIndexSnapshotCarriesNoMachineLocalField (IXS-0): the store spec and
// the code name the same machine-local fields, and an export's copy
// carries none of them: ClearMachineLocal on a VACUUM INTO copy clears
// every path to "" and every modification time and inode to 0, keeps every
// portable column, leaves the source store untouched, changes no format
// marker, leaves no -wal/-shm beside the copy, still mounts at L0 and
// Writer, and a remap restores service_meta.path.
func TestIndexSnapshotCarriesNoMachineLocalField(t *testing.T) {
	if got, want := MachineLocalFields(), specMachineLocalFields(t); !reflect.DeepEqual(got, want) {
		t.Fatalf("machine-local fields: code %v, docs/spec/store-format-v0.md %v", got, want)
	}
	for _, want := range []string{"file_stats.ino", "file_stats.mtime_ns", "ingest_state.path", "service_meta.path"} {
		if !containsString(MachineLocalFields(), want) {
			t.Errorf("MachineLocalFields() lacks %s", want)
		}
	}

	ctx := context.Background()
	dir := t.TempDir()
	src := filepath.Join(dir, "source.sqlite")
	corpus := "/machine/local/corpus"
	st, err := NewSQLiteStore(src)
	if err != nil {
		t.Fatal(err)
	}
	meta := ServiceMeta{ServiceID: "svc", Path: corpus, IndexedCommit: strings.Repeat("a", 40), IndexedAt: 7}
	files := []FileStat{
		{Path: "a.go", Size: 10, MtimeNS: 1_700_000_000_000_000_000, Ino: 4242, ContentHash: "h-a", Lang: "go"},
		{Path: "b.go", Size: 20, MtimeNS: 1_700_000_000_000_000_001, Ino: 4343, ContentHash: "h-b", Lang: "go"},
	}
	if err := st.WriteServiceSummary(ctx, meta, ServiceSummary{Watermark: meta.IndexedCommit}, files); err != nil {
		t.Fatal(err)
	}
	if err := st.SetIngestState(ctx, IngestState{ServiceID: "svc", State: IngestComplete, PID: 1, StartedAt: 1, TargetCommit: meta.IndexedCommit, Path: corpus}); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	copyPath := filepath.Join(dir, "snapshot.sqlite")
	raw, err := sql.Open("sqlite", src)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, "VACUUM INTO ?", copyPath); err != nil {
		t.Fatal(err)
	}
	_ = raw.Close()

	if err := ClearMachineLocal(ctx, copyPath); err != nil {
		t.Fatalf("ClearMachineLocal: %v", err)
	}
	for _, sidecar := range []string{"-wal", "-shm", "-journal"} {
		if _, err := os.Stat(copyPath + sidecar); err == nil {
			t.Errorf("the cleared copy left a %s sidecar", sidecar)
		}
	}

	cp, err := sql.Open("sqlite", "file:"+copyPath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer cp.Close()
	for _, f := range machineLocalFields {
		var dirty int
		q := "SELECT COUNT(*) FROM " + f.table + " WHERE " + f.column + " != ?"
		if err := cp.QueryRowContext(ctx, q, f.cleared).Scan(&dirty); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		if dirty != 0 {
			t.Errorf("%s.%s: %d row(s) still carry a machine-local value", f.table, f.column, dirty)
		}
	}
	var rows, sized int
	if err := cp.QueryRowContext(ctx, "SELECT COUNT(*), COALESCE(SUM(size), 0) FROM file_stats WHERE content_hash IN ('h-a', 'h-b')").Scan(&rows, &sized); err != nil || rows != 2 || sized != 30 {
		t.Errorf("file_stats portable columns: %d rows, size %d, %v; want both rows with their sizes and hashes", rows, sized, err)
	}
	var commit string
	if err := cp.QueryRowContext(ctx, "SELECT indexed_commit FROM service_meta WHERE service_id = 'svc'").Scan(&commit); err != nil || commit != meta.IndexedCommit {
		t.Errorf("service_meta.indexed_commit = %q (%v), want it kept", commit, err)
	}
	var appID, userVersion int
	if err := cp.QueryRow("PRAGMA application_id").Scan(&appID); err != nil || appID != storeApplicationID {
		t.Errorf("application_id = %#x (%v)", appID, err)
	}
	if err := cp.QueryRow("PRAGMA user_version").Scan(&userVersion); err != nil || userVersion != storeFormatVersion {
		t.Errorf("user_version = %d (%v), want %d", userVersion, err, storeFormatVersion)
	}
	_ = cp.Close()

	for _, level := range []ConformanceLevel{LevelL0, LevelWriter} {
		m, err := MountReadOnly(copyPath, level)
		if err != nil {
			t.Fatalf("the cleared copy fails its %s mount: %v", level, err)
		}
		_ = m.Close()
	}

	orig, err := NewSQLiteStore(src)
	if err != nil {
		t.Fatal(err)
	}
	if got, _, _ := orig.GetServiceMeta(ctx, "svc"); got.Path != corpus {
		t.Errorf("source service_meta.path = %q, want it untouched (%q)", got.Path, corpus)
	}
	_ = orig.Close()

	re, err := NewSQLiteStore(copyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer re.Close()
	got, found, err := re.GetServiceMeta(ctx, "svc")
	if err != nil || !found || got.Path != "" {
		t.Fatalf("cleared service_meta = %+v found=%v err=%v, want path ''", got, found, err)
	}
	got.Path = "/reading/machine/corpus"
	if err := re.SetServiceMeta(ctx, got); err != nil {
		t.Fatal(err)
	}
	if back, _, _ := re.GetServiceMeta(ctx, "svc"); back.Path != "/reading/machine/corpus" {
		t.Errorf("remap: service_meta.path = %q, want the new root", back.Path)
	}
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
