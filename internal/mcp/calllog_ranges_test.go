package mcp_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

func TestCallLogLeavesRangeSnapshotsToHarnessObserver(t *testing.T) {
	root := t.TempDir()
	writeCorpusFile(t, root, "sample.go", "package sample\n")
	var log bytes.Buffer
	srv := mcp.NewServerWithLogger(&extendedFakeBackend{services: []string{"svc"}, svcPath: root}, mcp.NewCallLogger(&log))
	readSourceCall(t, srv, map[string]any{"service": "svc", "file": "sample.go"})
	if log.Len() == 0 || !strings.Contains(log.String(), `"pointers"`) {
		t.Fatalf("call must still log its pointers: %s", log.String())
	}
	if strings.Contains(log.String(), `"range_pointers"`) {
		t.Errorf("daemon computed unread range snapshots: %s", log.String())
	}
}
