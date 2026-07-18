package bedrock

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestSigV4ReferenceVector pins the hand-rolled signing-key chain and final
// HMAC against the published AWS SigV4 reference example, the documented
// ground truth used to validate any SigV4 implementation:
//
//	secret    = wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY
//	date      = 20150830, region = us-east-1, service = iam
//	scope     = 20150830/us-east-1/iam/aws4_request
//	stringToSign (documented):
//	  AWS4-HMAC-SHA256
//	  20150830T123600Z
//	  20150830/us-east-1/iam/aws4_request
//	  f536975d06c0309214f805bb90ccff089219ecd68b2577efef23edd43b7e1a59
//	signature = 5d672d79c15b13162d9279b0855cfba6789a8edb4c82c400e06b5924a6f2b5d7
//
// This exercises deriveSigningKey and the terminating HMAC — the heart of the
// signer — independently of header canonicalization, so a bug in either is
// caught against an external constant rather than a self-consistent one.
func TestSigV4ReferenceVector(t *testing.T) {
	const (
		secret       = "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY"
		dateStamp    = "20150830"
		region       = "us-east-1"
		svc          = "iam"
		stringToSign = "AWS4-HMAC-SHA256\n" +
			"20150830T123600Z\n" +
			"20150830/us-east-1/iam/aws4_request\n" +
			"f536975d06c0309214f805bb90ccff089219ecd68b2577efef23edd43b7e1a59"
		wantSig = "5d672d79c15b13162d9279b0855cfba6789a8edb4c82c400e06b5924a6f2b5d7"
	)
	key := deriveSigningKey(secret, dateStamp, region, svc)
	got := toHex(hmacSHA256(key, []byte(stringToSign)))
	if got != wantSig {
		t.Fatalf("SigV4 reference signature = %q, want %q (signing-key chain or HMAC is wrong)", got, wantSig)
	}
}

// TestSignRequestStructure proves an end-to-end signed request carries a
// well-formed Authorization header: the algorithm, the {accessKey}/{scope}
// credential, the sorted lowercase SignedHeaders list (always including host,
// x-amz-content-sha256, x-amz-date), and a 64-hex-char signature.
func TestSignRequestStructure(t *testing.T) {
	creds := credentials{accessKeyID: "AKIDEXAMPLE", secretAccessKey: "secret"}
	req, err := http.NewRequest(http.MethodPost,
		"https://bedrock-runtime.us-gov-west-1.amazonaws.com/model/anthropic.claude-3%3A0/invoke", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	when := time.Date(2015, 8, 30, 12, 36, 0, 0, time.UTC)
	signRequest(req, creds, service, "us-gov-west-1", when, []byte(`{"x":1}`))

	auth := req.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 ") {
		t.Errorf("auth does not start with the algorithm: %q", auth)
	}
	wantCred := "Credential=AKIDEXAMPLE/20150830/us-gov-west-1/bedrock/aws4_request"
	if !strings.Contains(auth, wantCred) {
		t.Errorf("auth missing credential %q:\n%s", wantCred, auth)
	}
	// accept, content-type, host, x-amz-content-sha256, x-amz-date — sorted.
	wantSigned := "SignedHeaders=accept;content-type;host;x-amz-content-sha256;x-amz-date"
	if !strings.Contains(auth, wantSigned) {
		t.Errorf("auth missing expected SignedHeaders %q:\n%s", wantSigned, auth)
	}
	if got := req.Header.Get("X-Amz-Date"); got != "20150830T123600Z" {
		t.Errorf("X-Amz-Date = %q, want 20150830T123600Z", got)
	}
	sig := signatureHex(t, auth)
	if len(sig) != 64 || strings.TrimLeft(sig, "0123456789abcdef") != "" {
		t.Errorf("signature is not 64-char lowercase hex: %q", sig)
	}
}

// signatureHex extracts the Signature= value from an Authorization header.
func signatureHex(t *testing.T, auth string) string {
	t.Helper()
	i := strings.Index(auth, "Signature=")
	if i < 0 {
		t.Fatalf("no Signature= in %q", auth)
	}
	return auth[i+len("Signature="):]
}

// TestSigV4Deterministic proves the signer is a pure function of its inputs:
// identical (request, creds, service, region, time, payload) yields a
// byte-identical Authorization header across repeated calls. SigV4 has no
// nonce, so any nondeterminism would be a bug (e.g. map iteration leaking into
// the canonical headers).
func TestSigV4Deterministic(t *testing.T) {
	creds := credentials{accessKeyID: "AKID", secretAccessKey: "secret"}
	when := time.Unix(1718000000, 0).UTC()
	body := []byte(`{"anthropic_version":"bedrock-2023-05-31"}`)

	var first string
	for i := 0; i < 32; i++ {
		req, _ := http.NewRequest(http.MethodPost, "https://bedrock-runtime.us-gov-west-1.amazonaws.com/model/m/invoke", nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-Custom", "  a   b  ") // exercises value-trim/collapse
		signRequest(req, creds, service, "us-gov-west-1", when, body)
		got := req.Header.Get("Authorization")
		if i == 0 {
			first = got
			continue
		}
		if got != first {
			t.Fatalf("non-deterministic Authorization at iter %d:\n%s\n%s", i, first, got)
		}
	}
}

// TestSigV4SignsSessionToken proves a session token is added to the signed
// header set and carried in X-Amz-Security-Token.
func TestSigV4SignsSessionToken(t *testing.T) {
	creds := credentials{accessKeyID: "AKID", secretAccessKey: "secret", sessionToken: "FwoGZX...tok"}
	req, _ := http.NewRequest(http.MethodPost, "https://bedrock-runtime.us-gov-west-1.amazonaws.com/model/m/invoke", nil)
	signRequest(req, creds, service, "us-gov-west-1", time.Unix(1718000000, 0).UTC(), []byte(`{}`))

	if got := req.Header.Get("X-Amz-Security-Token"); got != "FwoGZX...tok" {
		t.Errorf("X-Amz-Security-Token = %q, want the session token", got)
	}
	auth := req.Header.Get("Authorization")
	if !strings.Contains(auth, "x-amz-security-token") {
		t.Errorf("session token must be in SignedHeaders:\n%s", auth)
	}
}

// TestSigV4SecretNeverOnWire proves the secret access key never appears in any
// signed header or the Authorization header — only its derivation does.
func TestSigV4SecretNeverOnWire(t *testing.T) {
	const secret = "uniqueSECRET-value-12345"
	creds := credentials{accessKeyID: "AKID", secretAccessKey: secret, sessionToken: "tok"}
	req, _ := http.NewRequest(http.MethodPost, "https://bedrock-runtime.us-gov-west-1.amazonaws.com/model/m/invoke", nil)
	signRequest(req, creds, service, "us-gov-west-1", time.Unix(1718000000, 0).UTC(), []byte(`{"x":1}`))

	for name, vals := range req.Header {
		for _, v := range vals {
			if strings.Contains(v, secret) {
				t.Errorf("secret leaked into header %q: %q", name, v)
			}
		}
	}
}

// TestCanonicalHelpers is a table-driven check of the canonicalization
// primitives the signer composes.
func TestCanonicalHelpers(t *testing.T) {
	t.Run("uriEncode", func(t *testing.T) {
		cases := []struct {
			in          string
			encodeSlash bool
			want        string
		}{
			{"abcABC123-_.~", true, "abcABC123-_.~"},
			{"a b", true, "a%20b"},
			{"a/b", false, "a/b"},
			{"a/b", true, "a%2Fb"},
			{"anthropic.claude-3:0", true, "anthropic.claude-3%3A0"},
			{"k=v&x", true, "k%3Dv%26x"},
		}
		for _, c := range cases {
			if got := uriEncode(c.in, c.encodeSlash); got != c.want {
				t.Errorf("uriEncode(%q, %v) = %q, want %q", c.in, c.encodeSlash, got, c.want)
			}
		}
	})

	t.Run("trimAll", func(t *testing.T) {
		cases := []struct{ in, want string }{
			{"  a  b  ", "a b"},
			{"a\t\tb", "a b"},
			{"plain", "plain"},
			{"   ", ""},
		}
		for _, c := range cases {
			if got := trimAll(c.in); got != c.want {
				t.Errorf("trimAll(%q) = %q, want %q", c.in, got, c.want)
			}
		}
	})

	t.Run("canonicalQuery", func(t *testing.T) {
		cases := []struct{ in, want string }{
			{"", ""},
			{"b=2&a=1", "a=1&b=2"},
			{"a=1&a=2", "a=1&a=2"},
			{"x", "x="},
		}
		for _, c := range cases {
			if got := canonicalQuery(c.in); got != c.want {
				t.Errorf("canonicalQuery(%q) = %q, want %q", c.in, got, c.want)
			}
		}
	})

	t.Run("emptyPayloadHashConstant", func(t *testing.T) {
		// Lock the documented SHA256("") constant used by body-less probes.
		got := hashHex(nil)
		if got != emptyPayloadHash {
			t.Errorf("SHA256(\"\") = %q, want %q", got, emptyPayloadHash)
		}
	})
}

// hashHex is a tiny helper mirroring the signer's payload-hash computation.
func hashHex(payload []byte) string {
	return toHex(sha256Sum(payload))
}

func toHex(b []byte) string {
	const h = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, c := range b {
		out[i*2] = h[c>>4]
		out[i*2+1] = h[c&0xf]
	}
	return string(out)
}
