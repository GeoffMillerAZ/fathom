package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
)

// review_diff is bounded as a complete ToolResult, rather than the shared
// first-item-preserving payload target. Never trim serialized JSON: compact
// values, disclose every omission, then measure both copies and all metadata.
func reviewBudgetRefusal(reason string) *rpcError {
	return &rpcError{Code: ErrCodeInternal, Message: "review_diff response budget: " + reason}
}

// reviewBudgetDisabled is the private per-request differential-test seam.
type reviewBudgetDisabled struct{}

type reviewOmission struct {
	Total    int    `json:"total"`
	Returned int    `json:"returned"`
	Unit     string `json:"unit"`
}

func capReviewDiffEnvelope(envelope map[string]any, limit int) (map[string]any, error) {
	if limit <= 0 || limit > defaultCharBudget {
		limit = defaultCharBudget
	}
	n, err := jsonSize(envelope, limit)
	if err != nil {
		return nil, errors.New("malformed result")
	}
	if n <= limit {
		return envelope, nil
	}
	raw, ok := envelope["structuredContent"].(json.RawMessage)
	if !ok {
		return nil, errors.New("structured review result missing")
	}
	var payload map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber() // Counts must survive without a float64 round trip.
	if dec.Decode(&payload) != nil || payload == nil {
		return nil, errors.New("malformed review object")
	}
	dims, ok := payload["dimensions"].([]any)
	if !ok || len(dims) != 8 {
		return nil, errors.New("eight review dimensions required")
	}
	statuses := make(map[string]reviewDimensionStatus, 8)
	for _, value := range dims {
		dim, ok := value.(map[string]any)
		if !ok {
			return nil, errors.New("malformed review dimension")
		}
		name, _ := dim["name"].(string)
		switch name {
		case review.DimImpact, review.DimObligations, review.DimArch, review.DimSpecCoverage, "coverage", review.DimCertifications, "integrity", review.DimConservation:
		default:
			return nil, errors.New("unknown review dimension")
		}
		checked, valid := dim["checked"].(bool)
		n, numeric := dim["count"].(json.Number)
		count, countErr := strconv.Atoi(string(n))
		if _, duplicate := statuses[name]; duplicate || !valid || !numeric || countErr != nil || count < 0 {
			return nil, errors.New("malformed review status")
		}
		status := reviewDimensionStatus{Checked: checked, Count: count}
		if !checked {
			reason, valid := dim["reason"].(string)
			if !valid {
				return nil, errors.New("unchecked review reason missing")
			}
			status.UncheckedReason = reason
		}
		statuses[name] = status
	}

	// Normalize only envelope metadata: never marshal/decode the two payload
	// copies just to obtain refresh, suppression and partial-budget fields.
	metadata := make(map[string]any)
	for key, value := range envelope {
		if key != "content" && key != "structuredContent" {
			metadata[key] = value
		}
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return nil, errors.New("malformed result metadata")
	}
	var originalWire map[string]any
	dec = json.NewDecoder(bytes.NewReader(encoded))
	dec.UseNumber()
	if dec.Decode(&originalWire) != nil {
		return nil, errors.New("malformed result metadata")
	}
	omitted := make(map[string]reviewOmission)
	envelopeOmitted := make(map[string]reviewOmission)
	wire := make(map[string]any, len(originalWire)+3)
	for k, v := range originalWire {
		wire[k] = v
	}
	original := payload
	payload = make(map[string]any, len(original)+3)
	for k, v := range original {
		payload[k] = v
	}
	payload["dimension_status"] = statuses
	payload["details_complete"] = false
	wire["details_complete"] = false

	fit := func() (bool, error) {
		// Counts describe logical items, counting the payload once, never its
		// text mirror. Byte-prefix losses each count as one incomplete value.
		returned, total := reviewItemCounts(original, "", omitted)
		er, et := reviewItemCounts(originalWire, "", envelopeOmitted)
		payload["elided"] = reviewElision{elided: elisionFor(returned+er, total+et,
			"first finding per dimension, then dimension prefixes, changed_files, tests, layers, other fields", "narrow changed_files"), Fields: omitted}
		if len(envelopeOmitted) > 0 {
			wire["envelope_omissions"] = envelopeOmitted
		} else {
			delete(wire, "envelope_omissions")
		}
		// RT-90: measure first, including attempted prefixes that alone can be
		// enormous. Only materialize a candidate known to fit the payload cap.
		if n, err := jsonSize(payload, limit); err != nil {
			return false, errors.New("malformed compact review")
		} else if n > limit {
			return false, nil
		}
		b, err := json.Marshal(payload)
		if err != nil {
			return false, errors.New("malformed compact review")
		}
		wire["content"] = []any{map[string]any{"type": "text", "text": string(b)}}
		wire["structuredContent"] = json.RawMessage(b)
		if n, err := jsonSize(wire, limit); err != nil {
			return false, errors.New("malformed compact result")
		} else if n > limit {
			return false, nil
		}
		b, err = json.Marshal(wire)
		if err != nil {
			return false, errors.New("malformed compact result")
		}
		return len(b) <= limit, nil
	}
	// The alternate renderings are duplicate detail. Try dropping only those
	// before touching findings, the primary receipt or envelope disclosures.
	for _, key := range []string{"receipt_sarif", "receipt_check"} {
		if value, ok := payload[key]; ok {
			b, err := json.Marshal(value)
			if err != nil {
				return nil, errors.New("malformed review receipt")
			}
			omitted["/"+key] = reviewOmission{Total: len(b), Unit: "json_bytes"}
			delete(payload, key)
		}
	}
	if fits, err := fit(); err != nil {
		return nil, err
	} else if fits {
		return wire, nil
	}

	// First pass retains counters, identities and short explanations. Collect
	// original arrays separately so candidate prefixes never mutate the input.
	var prefixes []reviewArrayPrefix
	compactDims := make([]any, len(dims))
	for i, value := range dims {
		dim := value.(map[string]any)
		path := "/dimensions/" + dim["name"].(string)
		out := make(map[string]any, len(dim))
		for _, key := range reviewSortedKeys(dim) {
			if key == "name" || key == "checked" || key == "count" {
				out[key] = dim[key]
			} else {
				out[key] = compactReviewValue(dim[key], path+"/"+reviewPointerToken(key), omitted)
			}
		}
		compactDims[i] = out
		prefixes = append(prefixes, reviewArrayPrefixes(dim, out, path, omitted)...)
	}
	payload["dimensions"] = compactDims
	for name, status := range statuses {
		status.UncheckedReason = compactReviewString(status.UncheckedReason, "/dimension_status/"+name+"/unchecked_reason", omitted)
		statuses[name] = status
	}
	for _, key := range reviewSortedKeys(payload) {
		if key == "dimensions" || key == "dimension_status" || key == "elided" {
			continue
		}
		payload[key] = compactReviewValue(payload[key], "/"+reviewPointerToken(key), omitted)
	}
	prefixes = append(prefixes, reviewArrayPrefixes(original, payload, "", omitted)...)
	for _, key := range reviewSortedKeys(originalWire) {
		wire[key] = compactReviewValue(originalWire[key], "/"+reviewPointerToken(key), envelopeOmitted)
	}
	prefixes = append(prefixes, reviewArrayPrefixes(originalWire, wire, "", envelopeOmitted)...)
	// Stable ties preserve dimension order and sorted field order. Reserve a
	// first item in each nonempty list before greedily extending prefixes;
	// otherwise a large early dimension can starve every later dimension.
	sort.SliceStable(prefixes, func(i, j int) bool { return prefixes[i].priority() < prefixes[j].priority() })
	if fits, err := fit(); err != nil {
		return nil, err
	} else if fits {
		try := func(p reviewArrayPrefix, n int) (bool, error) {
			old := p.parent[p.key]
			loss, had := p.omitted[p.path]
			p.parent[p.key] = p.items[:n]
			if n == len(p.items) {
				delete(p.omitted, p.path)
			} else {
				p.omitted[p.path] = reviewOmission{Total: len(p.items), Returned: n, Unit: "items"}
			}
			fits, err := fit()
			if !fits || err != nil {
				p.parent[p.key] = old
				if had {
					p.omitted[p.path] = loss
				} else {
					delete(p.omitted, p.path)
				}
			}
			return fits, err
		}
		for _, p := range prefixes {
			if _, err := try(p, 1); err != nil {
				return nil, err
			}
		}
		for _, p := range prefixes {
			for n := len(p.parent[p.key].([]any)) + 1; n <= len(p.items); n++ {
				fits, err := try(p, n)
				if err != nil {
					return nil, err
				}
				if !fits {
					break
				} // Only original contiguous prefixes are eligible.
			}
		}
		// A failed candidate leaves wire's serialized copies stale; rebuild from
		// the last accepted prefix and disclosure arithmetic before returning.
		if fits, err := fit(); err != nil {
			return nil, err
		} else if fits {
			return wire, nil
		}
	}
	// Keep the mandatory status floor when even compact dimension headers
	// cannot fit (e.g. every unchecked reason contains escaped control bytes).
	payload["dimensions"] = []any{}
	for path, loss := range omitted {
		if strings.HasPrefix(path, "/dimensions/") {
			loss.Returned = 0
			omitted[path] = loss
		}
	}
	omitted["/dimensions"] = reviewOmission{Total: len(dims), Unit: "items"}
	if fits, err := fit(); err != nil {
		return nil, err
	} else if fits {
		return wire, nil
	}

	return nil, errors.New("eight review statuses and omission metadata cannot fit")
}

// reviewElision extends the standard disclosure with payload-only loss paths.
type reviewElision struct {
	*elided
	Fields map[string]reviewOmission `json:"fields"`
}

type reviewArrayPrefix struct {
	parent    map[string]any
	key, path string
	items     []any
	omitted   map[string]reviewOmission
}

func (p reviewArrayPrefix) priority() int {
	switch {
	case strings.HasPrefix(p.path, "/dimensions/") && strings.HasSuffix(p.path, "/findings"):
		return 0
	case p.path == "/changed_files":
		return 1
	case strings.HasPrefix(p.path, "/test_impact/") || strings.HasSuffix(p.path, "/tests"):
		return 2
	case p.path == "/layers":
		return 3
	default:
		return 4
	}
}

// Arrays are restored as complete original items; nested arrays inside a
// restored item therefore return with it, rather than claiming empty detail.
func reviewArrayPrefixes(original, compact map[string]any, path string, omitted map[string]reviewOmission) []reviewArrayPrefix {
	var out []reviewArrayPrefix
	for _, key := range reviewSortedKeys(compact) {
		if key == "dimensions" && path == "" {
			continue
		} // Collected in dimension order.
		next := path + "/" + reviewPointerToken(key)
		switch value := original[key].(type) {
		case []any:
			if len(value) > 0 {
				out = append(out, reviewArrayPrefix{compact, key, next, value, omitted})
			}
		case map[string]any:
			if child, ok := compact[key].(map[string]any); ok {
				out = append(out, reviewArrayPrefixes(value, child, next, omitted)...)
			}
		}
	}
	return out
}

// Logical item accounting follows the same boundaries as compaction:
// arrays count entries; maps count their values; a removed rendering counts
// as one value. Dimensions are the exception: their named finding lists and
// header values are counted individually, unless all dimensions were removed.
func reviewItemCounts(value any, path string, omitted map[string]reviewOmission) (returned, total int) {
	if loss, ok := omitted[path]; ok {
		if loss.Unit == "items" {
			return loss.Returned, loss.Total
		}
		return 0, 1
	}
	switch v := value.(type) {
	case map[string]any:
		for _, key := range reviewSortedKeys(v) {
			r, t := reviewItemCounts(v[key], path+"/"+reviewPointerToken(key), omitted)
			returned += r
			total += t
		}
		return
	case []any:
		if path == "/dimensions" {
			for _, d := range v {
				dim := d.(map[string]any)
				r, t := reviewItemCounts(dim, path+"/"+dim["name"].(string), omitted)
				returned += r
				total += t
			}
			return
		}
		return len(v), len(v)
	default:
		return 1, 1
	}
}

// compactReviewValue is deliberately independent of concrete handler types.
// Arrays lose detail with an exact item count; maps preserve every counter and
// status. Pathological keys or a status floor that still cannot fit are refused.
func compactReviewValue(value any, path string, omitted map[string]reviewOmission) any {
	switch v := value.(type) {
	case string:
		return compactReviewString(v, path, omitted)
	case []any:
		if len(v) > 0 {
			omitted[path] = reviewOmission{Total: len(v), Unit: "items"}
		}
		return []any{}
	case map[string]any:
		out := make(map[string]any, len(v))
		for _, key := range reviewSortedKeys(v) {
			out[key] = compactReviewValue(v[key], path+"/"+reviewPointerToken(key), omitted)
		}
		return out
	default:
		return value
	}
}

func compactReviewString(value, path string, omitted map[string]reviewOmission) string {
	// Keep short explanations intact; long values retain a UTF-8 prefix.
	const inlineBytes, prefixBytes = 192, 96
	if len(value) <= inlineBytes {
		return value
	}
	n := prefixBytes
	for n > 0 && !utf8.RuneStart(value[n]) {
		n--
	}
	omitted[path] = reviewOmission{Total: len(value), Returned: n, Unit: "utf8_bytes"}
	return value[:n]
}

func reviewPointerToken(key string) string {
	return strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}

func reviewSortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
