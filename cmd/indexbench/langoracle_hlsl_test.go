package main

import "testing"

// HLSL's #include resolves through the include-path strategy (the includer's
// directory, then the source roots), so the file resolver serves it and
// clang-includes' imports are its file-edge truth (LB-F57, LB-Q-hlsl).
func TestFileResolveLangServesHLSL(t *testing.T) {
	if lang, ok := fileResolveLang["hlsl"]; !ok || lang != "hlsl" {
		t.Fatalf("fileResolveLang[hlsl] = %q, %v; want hlsl", lang, ok)
	}
}
