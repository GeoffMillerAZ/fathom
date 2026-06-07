package license_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/analysis/license"
)

// Minimal representative extracts from actual license texts.
var licenseTexts = map[string]string{
	"MIT": `MIT License

Copyright (c) 2024 Example Corp

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.`,

	"Apache-2.0": `Apache License
Version 2.0, January 2004
http://www.apache.org/licenses/

TERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION`,

	"BSD-3-Clause": `Copyright (c) 2024, Example Corp
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice, this
   list of conditions and the following disclaimer.
2. Redistributions in binary form must reproduce the above copyright notice.
3. Neither the name of the copyright holder nor the names of its contributors
   may be used to endorse or promote products derived from this software.`,

	"GPL-3.0": `GNU GENERAL PUBLIC LICENSE
Version 3, 29 June 2007

Copyright (C) 2007 Free Software Foundation, Inc.`,

	"ISC": `ISC License

Copyright (c) 2024, Example Corp

Permission to use, copy, modify, and/or distribute this software for any
purpose with or without fee is hereby granted, provided that the above
copyright notice and this permission notice appear in all copies.`,

	"Unlicense": `This is free and unencumbered software released into the public domain.

Anyone is free to copy, modify, publish, use, compile, sell, or distribute
this software, either in source code form or as compiled binary, for any
purpose, commercial or non-commercial, and by any means.`,

	"UNKNOWN": `Some random text that doesn't match any known license.`,
}

func TestDetect(t *testing.T) {
	for want, text := range licenseTexts {
		t.Run(want, func(t *testing.T) {
			got := license.Detect([]byte(text))
			if got != want {
				t.Errorf("Detect(): got %q, want %q", got, want)
			}
		})
	}
}

func TestDetect_Deterministic(t *testing.T) {
	for id, text := range licenseTexts {
		a := license.Detect([]byte(text))
		b := license.Detect([]byte(text))
		if a != b {
			t.Errorf("non-deterministic for %s: %q vs %q", id, a, b)
		}
	}
}

func TestPolicy_Allow(t *testing.T) {
	ok, reason := license.Policy("MIT", []string{"MIT", "Apache-2.0"})
	if !ok {
		t.Fatalf("expected ok=true, got reason=%q", reason)
	}
	if reason != "" {
		t.Fatalf("expected empty reason, got %q", reason)
	}
}

func TestPolicy_Deny(t *testing.T) {
	ok, reason := license.Policy("GPL-3.0", []string{"MIT", "Apache-2.0"})
	if ok {
		t.Fatal("expected ok=false")
	}
	if reason == "" {
		t.Fatal("expected non-empty reason")
	}
}

func TestPolicy_EmptyAllowed(t *testing.T) {
	ok, reason := license.Policy("MIT", nil)
	if ok {
		t.Fatal("expected ok=false for nil allowed list")
	}
	if reason == "" {
		t.Fatal("expected non-empty reason")
	}
}
