//go:build darwin && !server && nativeprobe

package main

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"
)

func observationsFor(id string) []observation {
	kinds := expectedObservationKinds(id)
	result := make([]observation, len(kinds))
	for i, kind := range kinds {
		result[i] = observation{ID: id, Kind: kind, Cookie: kind == "case-request"}
	}
	return result
}

func validReceipt() receipt {
	r := receipt{
		Profile:          profile,
		Go:               runtime.Version(),
		OS:               runtime.GOOS,
		Arch:             runtime.GOARCH,
		Completed:        true,
		ShutdownCallback: true,
	}
	for _, id := range cases {
		got := judge(id, observationsFor(id))
		got.Closed = true
		r.Cases = append(r.Cases, got)
	}
	return r
}

func cloneReceipt(t *testing.T, source receipt) receipt {
	t.Helper()
	data, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	var cloned receipt
	if err := json.Unmarshal(data, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
}

func TestJudgeRequiresExactCaseEvidence(t *testing.T) {
	for _, id := range cases {
		t.Run(id, func(t *testing.T) {
			good := observationsFor(id)
			if got := judge(id, good); !got.Passed {
				t.Fatalf("valid evidence rejected: %s", got.Reason)
			}

			mutations := map[string]func([]observation) []observation{
				"missing": func(in []observation) []observation { return in[:len(in)-1] },
				"duplicate": func(in []observation) []observation {
					return append(in, in[len(in)-1])
				},
				"wrong id": func(in []observation) []observation {
					in[0].ID = "other"
					return in
				},
				"wrong order": func(in []observation) []observation {
					in[0], in[1] = in[1], in[0]
					return in
				},
				"cookie missing": func(in []observation) []observation {
					for i := range in {
						if in[i].Kind == "case-request" {
							in[i].Cookie = false
							break
						}
					}
					return in
				},
			}
			for name, mutate := range mutations {
				t.Run(name, func(t *testing.T) {
					candidate := append([]observation(nil), good...)
					if judge(id, mutate(candidate)).Passed {
						t.Fatal("tampered evidence accepted")
					}
				})
			}
		})
	}
}

func TestJudgeRejectsNativeBridgeSignals(t *testing.T) {
	base := observationsFor("bridge")
	for name, mutate := range map[string]func([]observation) []observation{
		"external object": func(in []observation) []observation { in[3].External = true; return in },
		"runtime":         func(in []observation) []observation { in[3].Runtime = true; return in },
		"post accepted":   func(in []observation) []observation { in[3].PostAccepted = true; return in },
		"host delivered": func(in []observation) []observation {
			return append(in[:3], observation{ID: "bridge", Kind: "host-message"}, in[3])
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := append([]observation(nil), base...)
			if judge("bridge", mutate(candidate)).Passed {
				t.Fatal("native bridge signal accepted")
			}
		})
	}
}

func TestVerifyReceiptRejectsIncompleteOrTamperedEvidence(t *testing.T) {
	valid := validReceipt()
	if err := verifyReceipt(valid); err != nil {
		t.Fatalf("valid receipt rejected: %v", err)
	}

	mutations := map[string]func(*receipt){
		"wrong profile":       func(r *receipt) { r.Profile = "other" },
		"missing go":          func(r *receipt) { r.Go = "" },
		"wrong os":            func(r *receipt) { r.OS = "linux" },
		"missing arch":        func(r *receipt) { r.Arch = "" },
		"incomplete":          func(r *receipt) { r.Completed = false },
		"no shutdown":         func(r *receipt) { r.ShutdownCallback = false },
		"framework error":     func(r *receipt) { r.FrameworkErrors = 1 },
		"framework warning":   func(r *receipt) { r.FrameworkWarnings = 1 },
		"host bridge message": func(r *receipt) { r.HostBridgeMessages = 1 },
		"missing case":        func(r *receipt) { r.Cases = r.Cases[:len(r.Cases)-1] },
		"wrong case id":       func(r *receipt) { r.Cases[0].ID = "other" },
		"not closed":          func(r *receipt) { r.Cases[0].Closed = false },
		"tampered pass":       func(r *receipt) { r.Cases[0].Passed = false },
		"tampered reason":     func(r *receipt) { r.Cases[0].Reason = "claimed" },
		"wrong observation id": func(r *receipt) {
			r.Cases[0].Observations[0].ID = "other"
		},
		"duplicate observation": func(r *receipt) {
			r.Cases[0].Observations = append(r.Cases[0].Observations, r.Cases[0].Observations[0])
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			candidate := cloneReceipt(t, valid)
			mutate(&candidate)
			if err := verifyReceipt(candidate); err == nil {
				t.Fatal("tampered receipt accepted")
			}
		})
	}
}

func TestDecodeReceiptIsStrictAndComplete(t *testing.T) {
	data, err := json.Marshal(validReceipt())
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeReceipt(data)
	if err != nil {
		t.Fatalf("valid JSON rejected: %v", err)
	}
	if err := verifyReceipt(decoded); err != nil {
		t.Fatalf("decoded receipt rejected: %v", err)
	}

	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	object["UnknownField"] = true
	unknown, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	for name, candidate := range map[string][]byte{
		"malformed":      []byte(`{"Completed":`),
		"unknown field":  unknown,
		"trailing value": append(append([]byte(nil), data...), []byte(` {}`)...),
		"empty":          nil,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeReceipt(candidate); err == nil {
				t.Fatal("invalid receipt JSON accepted")
			}
		})
	}

	if _, err := decodeReceipt([]byte(strings.TrimSpace(string(data)) + "\n\t")); err != nil {
		t.Fatalf("trailing whitespace rejected: %v", err)
	}
}
