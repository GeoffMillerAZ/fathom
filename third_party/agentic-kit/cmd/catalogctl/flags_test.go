package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCommandsRefuseUnusedFlagsBeforeReadingInputs(t *testing.T) {
	for _, tc := range []struct {
		args []string
		flag string
	}{
		{[]string{"resolve", "-spec", "missing.json", "-bundle", "ignored.json"}, "bundle"},
		{[]string{"doctor", "-bundle", "missing.json", "-policy", "ignored.json"}, "policy"},
		{[]string{"doctor", "-spec", "missing.json", "-format", "rows"}, "format"},
		{[]string{"explain", "-bundle", "missing.json", "-id", "node", "-profile", "ignored.json"}, "profile"},
		{[]string{"query", "-bundle", "missing.json", "-include-hidden=false"}, "include-hidden"},
		{[]string{"search", "-bundle", "missing.json", "-text", "key", "-policy", "ignored.json"}, "policy"},
		{[]string{"diff", "-before", "missing.json", "-after", "missing.json", "-spec", "ignored.json"}, "spec"},
		{[]string{"prepare", "-bundle", "missing.json", "-binding", "missing.json", "-definition", "missing.txt", "-evidence", "missing.json", "-root", "ignored"}, "root"},
		{[]string{"scan-go", "-root", "missing", "-max-bytes=0"}, "max-bytes"},
		{[]string{"doctor", "-bundle", "missing.json", "-spec", "ignored.json"}, "spec"},
	} {
		t.Run(tc.args[0]+"/"+tc.flag, func(t *testing.T) {
			var out, diagnostic bytes.Buffer
			code := execute(tc.args, &out, &diagnostic)
			if code != 2 || out.Len() != 0 || !strings.Contains(diagnostic.String(), "-"+tc.flag) || !strings.Contains(diagnostic.String(), tc.args[0]) || strings.Contains(diagnostic.String(), "no such file") {
				t.Fatalf("ignored or late flag validation: code=%d stdout=%q stderr=%q", code, &out, &diagnostic)
			}
		})
	}
}

func TestCommandValuesRefuseBeforeReadingInputs(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"doctor", "-bundle="}, "-bundle"},
		{[]string{"resolve", "-spec", "missing.json", "-format", "typo"}, "format"},
		{[]string{"scan-go", "-root", "missing", "-format", "typo"}, "format"},
		{[]string{"explain", "-bundle", "missing.json"}, "-id"},
		{[]string{"search", "-bundle", "missing.json", "-text", "key", "-limit", "0"}, "-limit"},
		{[]string{"prepare", "-bundle", "missing.json", "-binding", "missing.json", "-definition", "missing.txt", "-evidence", "missing.json", "-max-bytes", "-1"}, "-max-bytes"},
	} {
		t.Run(tc.args[0]+"/"+tc.want, func(t *testing.T) {
			var out, diagnostic bytes.Buffer
			if code := execute(tc.args, &out, &diagnostic); code != 2 || out.Len() != 0 || !strings.Contains(diagnostic.String(), tc.want) || strings.Contains(diagnostic.String(), "no such file") {
				t.Fatalf("late argument validation: code=%d stdout=%q stderr=%q", code, &out, &diagnostic)
			}
		})
	}
}
