// Package modspec is the Go-side companion to schema/module/schema.cue, the
// expert-module manifest shape (docs/spec/expert-module-v0.md §3). Today it
// holds only the CUE-negatives test (schema_test.go) that keeps the shape
// falsifiable — no runtime loader for #Module ships yet (the honest gap
// docs/spec/expert-module-v0.md §7 records: unlike internal/semantic's
// #Layer, #Module is validated but not decoded/executed by any Go code in
// this repo). This file exists so the package is addressable by import path
// and doc comment even while it is test-only, and is the natural future
// home of that loader when the §7 gap closes.
package modspec
