package decide

// gate_bench_test.go — KS-2's per-ask cost: an open port's liveGate.check
// stats the kill file and the user config and reloads nothing while the
// user config's mtime holds.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/config"
)

func BenchmarkLiveGateCheck(b *testing.B) {
	dir := b.TempDir()
	b.Setenv("FATHOM_HOME", dir)
	user := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(user, []byte("[decide]\nprovider = \"typesafe\"\n"), 0o600); err != nil {
		b.Fatal(err)
	}
	g := newLiveGate(config.Decide{Provider: "typesafe"}, "", resolveOptions([]Option{WithUserConfig(user)}))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if v := g.check(); v.Closed {
			b.Fatalf("check = %+v, want open", v)
		}
	}
}
