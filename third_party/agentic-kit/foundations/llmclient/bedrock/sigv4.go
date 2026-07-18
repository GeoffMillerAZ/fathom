package bedrock

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sort"
	"strings"
	"time"
)

// AWS SigV4 request signing, hand-rolled on crypto/hmac + crypto/sha256 — no
// aws-sdk, no external dependencies. This implements the standard
// "Authorization header" variant of Signature Version 4 for the
// bedrock-runtime service. The four steps mirror the AWS reference:
//
//  1. canonical request   (method, path, query, headers, signed-header list,
//     payload hash)
//  2. string to sign      (algorithm, ISO8601 timestamp, credential scope,
//     SHA256 of the canonical request)
//  3. signing key         (a chain of HMAC-SHA256 over date / region /
//     service / "aws4_request", seeded with "AWS4"+secret)
//  4. signature + header   (HMAC-SHA256(signingKey, stringToSign), assembled
//     into the Authorization header)
//
// The credential carried in the header is the signature, derived from — but
// never equal to — the secret key; the secret itself never travels on the
// wire. A session token, when present, travels in the X-Amz-Security-Token
// header and is itself part of the signed header set.

const (
	// sigV4Algorithm is the algorithm token in the Authorization header and
	// the string-to-sign.
	sigV4Algorithm = "AWS4-HMAC-SHA256"

	// sigV4Request is the terminating component of the credential scope.
	sigV4Request = "aws4_request"

	// amzDateFormat is SigV4's basic-ISO8601 timestamp (UTC), e.g.
	// 20060102T150405Z. shortDate (the YYYYMMDD prefix) is the scope's date.
	amzDateFormat = "20060102T150405Z"
	shortDate     = "20060102"

	// emptyPayloadHash is the SHA256 of the empty string, the payload hash for
	// a body-less request (used by the GovCloud-region probe path).
	emptyPayloadHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

// credentials are the AWS access-key triple. SessionToken is set only for
// temporary (STS / instance-role) credentials and is empty for long-lived
// keys.
type credentials struct {
	accessKeyID     string
	secretAccessKey string
	sessionToken    string
}

// signRequest signs req in place with SigV4 for the given service, region, and
// instant, using payload as the exact bytes of the request body. It sets the
// Host (via req.Host being honoured by net/http), X-Amz-Date,
// X-Amz-Security-Token (when a session token is present), and Authorization
// headers. payloadHash, when non-empty, is used verbatim as the
// x-amz-content-sha256 / payload hash; otherwise it is computed from payload.
//
// req must already carry its final URL (host + path + query) and any headers
// the caller wants included in the signature (e.g. content-type, accept). Only
// host, x-amz-date, and x-amz-security-token are added by the signer; every
// other header present at call time is signed.
func signRequest(req *http.Request, creds credentials, service, region string, t time.Time, payload []byte) {
	t = t.UTC()
	amzDate := t.Format(amzDateFormat)
	dateStamp := t.Format(shortDate)

	hash := hex.EncodeToString(sha256Sum(payload))

	// The host header is what SigV4 signs; net/http derives the wire Host
	// header from req.Host / req.URL.Host, so set req.Host explicitly and sign
	// that exact value.
	host := req.URL.Host
	req.Host = host
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", hash)
	if creds.sessionToken != "" {
		req.Header.Set("X-Amz-Security-Token", creds.sessionToken)
	}

	canonicalHeaders, signedHeaders := canonicalizeHeaders(req, host)

	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURIPath(req.URL.EscapedPath()),
		canonicalQuery(req.URL.RawQuery),
		canonicalHeaders,
		signedHeaders,
		hash,
	}, "\n")

	scope := strings.Join([]string{dateStamp, region, service, sigV4Request}, "/")
	stringToSign := strings.Join([]string{
		sigV4Algorithm,
		amzDate,
		scope,
		hex.EncodeToString(sha256Sum([]byte(canonicalRequest))),
	}, "\n")

	signingKey := deriveSigningKey(creds.secretAccessKey, dateStamp, region, service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	authorization := sigV4Algorithm +
		" Credential=" + creds.accessKeyID + "/" + scope +
		", SignedHeaders=" + signedHeaders +
		", Signature=" + signature
	req.Header.Set("Authorization", authorization)
}

// canonicalizeHeaders builds the canonical-headers block and the
// semicolon-joined signed-header list. Every request header is signed (header
// names lowercased and sorted; values trimmed and inner runs of whitespace
// collapsed), plus the host header derived from the URL. This signs MORE than
// the minimum required set, which is always valid: AWS verifies the signature
// over exactly the headers SignedHeaders names.
func canonicalizeHeaders(req *http.Request, host string) (canonical, signed string) {
	// Collect lowercased name -> trimmed/collapsed values.
	type kv struct {
		name string
		vals []string
	}
	byName := map[string][]string{}
	for name, vals := range req.Header {
		ln := strings.ToLower(name)
		for _, v := range vals {
			byName[ln] = append(byName[ln], trimAll(v))
		}
	}
	// host is always signed and comes from the URL, not req.Header.
	byName["host"] = []string{host}

	pairs := make([]kv, 0, len(byName))
	for name, vals := range byName {
		// Multiple values for one header join with commas in the canonical
		// form (after each is trimmed/collapsed).
		pairs = append(pairs, kv{name: name, vals: vals})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].name < pairs[j].name })

	var cb strings.Builder
	names := make([]string, 0, len(pairs))
	for _, p := range pairs {
		cb.WriteString(p.name)
		cb.WriteByte(':')
		cb.WriteString(strings.Join(p.vals, ","))
		cb.WriteByte('\n')
		names = append(names, p.name)
	}
	return cb.String(), strings.Join(names, ";")
}

// canonicalURIPath returns the URI path component for the canonical request.
// Bedrock model IDs in the path may contain characters (':', '.') that AWS
// expects already-encoded by the caller; we pass the EscapedPath through and
// only guarantee a non-empty path. Per SigV4, the path is used as-is for
// already-normalized service paths (Bedrock is on the
// no-double-encode/no-normalize list via the runtime endpoints).
func canonicalURIPath(escaped string) string {
	if escaped == "" {
		return "/"
	}
	return escaped
}

// canonicalQuery returns the canonical query string: parameters sorted by
// key (then value), each key and value URI-encoded per RFC3986. An empty
// query yields the empty string.
func canonicalQuery(raw string) string {
	if raw == "" {
		return ""
	}
	type pair struct{ k, v string }
	var pairs []pair
	for _, part := range strings.Split(raw, "&") {
		if part == "" {
			continue
		}
		k, v, _ := strings.Cut(part, "=")
		pairs = append(pairs, pair{k: uriEncode(decodePercent(k), true), v: uriEncode(decodePercent(v), true)})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].k != pairs[j].k {
			return pairs[i].k < pairs[j].k
		}
		return pairs[i].v < pairs[j].v
	})
	var b strings.Builder
	for i, p := range pairs {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(p.k)
		b.WriteByte('=')
		b.WriteString(p.v)
	}
	return b.String()
}

// deriveSigningKey computes the SigV4 signing key by chaining HMAC-SHA256 over
// the date, region, service, and the "aws4_request" terminator, seeded with
// "AWS4" + secret.
func deriveSigningKey(secret, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte(sigV4Request))
}

// hmacSHA256 returns HMAC-SHA256(key, data).
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// sha256Sum returns the SHA256 digest of data.
func sha256Sum(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}

// trimAll trims leading/trailing whitespace and collapses internal runs of
// whitespace to a single space, per SigV4 header-value normalization.
func trimAll(s string) string {
	s = strings.TrimSpace(s)
	if !strings.ContainsAny(s, " \t") {
		return s
	}
	var b strings.Builder
	space := false
	for _, r := range s {
		if r == ' ' || r == '\t' {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	return b.String()
}

// uriEncode percent-encodes s per RFC3986 as SigV4 requires. The unreserved
// set (A-Z a-z 0-9 - _ . ~) is never encoded; '/' is preserved when
// encodeSlash is false (path segments) and encoded when true (query values).
func uriEncode(s string, encodeSlash bool) string {
	const upperhex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9',
			c == '-', c == '_', c == '.', c == '~':
			b.WriteByte(c)
		case c == '/' && !encodeSlash:
			b.WriteByte(c)
		default:
			b.WriteByte('%')
			b.WriteByte(upperhex[c>>4])
			b.WriteByte(upperhex[c&0x0f])
		}
	}
	return b.String()
}

// decodePercent reverses percent-encoding so query values that arrive already
// encoded in RawQuery are normalized once before canonical re-encoding. Invalid
// escapes are left verbatim.
func decodePercent(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			hi, ok1 := fromHex(s[i+1])
			lo, ok2 := fromHex(s[i+2])
			if ok1 && ok2 {
				b.WriteByte(hi<<4 | lo)
				i += 2
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func fromHex(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
