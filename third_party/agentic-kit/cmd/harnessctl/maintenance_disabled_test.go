//go:build !harness_maintenance

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestMaintenanceDisabledBeforeReads(t *testing.T) {
	for _, args := range [][]string{{"init", "--controller", "/does-not-exist", "--project", "/also-missing"}, {"prepare", "--candidate", "/private/path", "--controller-sha256", "secret"}, {"enter", "--format", "text"}, {"nonsense"}} {
		var out bytes.Buffer
		code := runMaintenance(args, &out)
		if code != 2 {
			t.Fatal(code)
		}
		var report map[string]any
		if e := json.Unmarshal(out.Bytes(), &report); e != nil {
			t.Fatal(e)
		}
		if report["status"] != "hold" || report["code"] != "unsupported" || report["noHostActionAuthority"] != true {
			t.Fatal(report)
		}
		for _, secret := range []string{"/does-not-exist", "/private/path", "secret"} {
			if strings.Contains(out.String(), secret) {
				t.Fatal("echoed caller data")
			}
		}
	}
}
func TestMaintenanceDisabledRoute(t *testing.T) {
	var out bytes.Buffer
	if code := run([]string{"maintenance", "init", "--controller", "/missing"}, &out, defaults()); code != 2 || !strings.Contains(out.String(), "unsupported") {
		t.Fatal(code, out.String())
	}
}
