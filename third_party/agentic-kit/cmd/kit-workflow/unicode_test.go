package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestProseCLIRejectsMalformedUnicodeIdentity(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"check", "export"} {
		for _, escape := range []string{`\ud800`, `\udfff`, `\ud800\u0041`} {
			t.Run(operation+escape, func(t *testing.T) {
				in := fixtureInput(t)
				if operation == "export" {
					in.Audience = "public"
				}
				raw, err := json.Marshal(in)
				if err != nil {
					t.Fatal(err)
				}
				raw = bytes.Replace(raw, []byte(`"principal":"reader"`), []byte(`"principal":"`+escape+`"`), 1)
				code, _, failure, _ := executeJSON(t, []string{"prose", operation}, raw)
				if code == 0 || failure.Error.Kind != "invalid" {
					t.Fatalf("malformed principal normalized: code=%d error=%+v", code, failure)
				}
			})
		}
	}
}

func TestProseCLIValidUnicodeIdentity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ escaped, want string }{
		{`\ud834\udd1e`, "𝄞"}, {`\ufffd`, "�"}, {`\\ud800`, `\ud800`},
	} {
		t.Run(tc.escaped, func(t *testing.T) {
			raw, err := json.Marshal(fixtureInput(t))
			if err != nil {
				t.Fatal(err)
			}
			raw = bytes.Replace(raw, []byte(`"principal":"reader"`), []byte(`"principal":"`+tc.escaped+`"`), 1)
			code, success, failure, _ := executeJSON(t, []string{"prose", "check"}, raw)
			if code != 0 || success.Artifact == nil || success.Artifact.Request.Principal != tc.want {
				t.Fatalf("valid Unicode rejected/changed: code=%d error=%+v", code, failure)
			}
		})
	}
}
