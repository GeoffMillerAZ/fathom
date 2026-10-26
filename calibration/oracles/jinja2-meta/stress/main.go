// Stress exercises resource bounds and recovery through the production extractor.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

type result struct {
	Name    string  `json:"name"`
	Bytes   int     `json:"bytes"`
	MS      float64 `json:"ms"`
	Symbols int     `json:"symbols"`
	Faults  int64   `json:"faults"`
	Pages   uint32  `json:"pages"`
	Error   string  `json:"error,omitempty"`
}

func run() error {
	e, err := extractor.NewTSExtractor("jinja2")
	if err != nil {
		return err
	}
	defer e.Close()
	if _, err = e.Extract("stress", "warm.j2", []byte("{% macro warm() %}ok{% endmacro %}")); err != nil {
		return err
	}
	cases := []struct{ name, source string }{
		{"text-2MiB", strings.Repeat("x", 2<<20)},
		{"nested-256", strings.Repeat("{% if enabled %}", 256) + "ok" + strings.Repeat("{% endif %}", 256)},
		{"malformed-64KiB", strings.Repeat("{% if (", 8192)},
		{"recovery", "{% macro after() %}{{ value if enabled }}{% endmacro %}"},
	}
	out := []result{}
	for _, c := range cases {
		before := e.EngineFaults()
		tsengine.ResetPeakModulePages()
		start := time.Now()
		nodes, extractErr := e.Extract("stress", c.name+".j2", []byte(c.source))
		r := result{Name: c.name, Bytes: len(c.source), MS: float64(time.Since(start).Microseconds()) / 1000, Symbols: len(nodes), Faults: e.EngineFaults() - before, Pages: tsengine.PeakModulePages()}
		if extractErr != nil {
			r.Error = extractErr.Error()
		}
		out = append(out, r)
	}
	if out[2].Faults != 1 || out[3].Faults != 0 || out[3].Symbols == 0 {
		return fmt.Errorf("fault containment or recovery failed: malformed=%d recovery=%d symbols=%d", out[2].Faults, out[3].Faults, out[3].Symbols)
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
