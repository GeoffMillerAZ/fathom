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

type reviewOmission struct {
	Total    int    `json:"total"`
	Returned int    `json:"returned"`
	Unit     string `json:"unit"`
}

func capReviewDiffEnvelope(envelope map[string]any, limit int) (map[string]any, error) {
	if limit <= 0 || limit > defaultCharBudget {
		limit = defaultCharBudget
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return nil, errors.New("malformed result")
	}
	if len(encoded) <= limit {
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

	omitted := make(map[string]reviewOmission)
	// Keep each status identity and count, even if all detailed findings go.
	// Named field paths identify omitted values, with dimensions keyed by name.
	for _, value := range dims {
		dim := value.(map[string]any)
		name := dim["name"].(string)
		for _, key := range reviewSortedKeys(dim) {
			if key == "name" || key == "checked" || key == "count" {
				continue
			}
			path := "/dimensions/" + name + "/" + reviewPointerToken(key)
			dim[key] = compactReviewValue(dim[key], path, omitted)
		}
	}
	for name, status := range statuses {
		path := "/dimension_status/" + name + "/unchecked_reason"
		status.UncheckedReason = compactReviewString(status.UncheckedReason, path, omitted)
		statuses[name] = status
	}
	for _, key := range reviewSortedKeys(payload) {
		if key == "dimensions" || key == "dimension_status" {
			continue
		}
		path := "/" + reviewPointerToken(key)
		switch key {
		case "receipt", "receipt_sarif", "receipt_check":
			b, err := json.Marshal(payload[key])
			if err != nil {
				return nil, errors.New("malformed review receipt")
			}
			omitted[path] = reviewOmission{Total: len(b), Unit: "json_bytes"}
			delete(payload, key)
		default:
			payload[key] = compactReviewValue(payload[key], path, omitted)
		}
	}
	payload["dimension_status"] = statuses
	payload["details_complete"] = false

	// Normalize metadata through JSON as well: a typed or raw field must not
	// escape compaction through a failed type assertion. UseNumber keeps counters.
	var wire map[string]any
	dec = json.NewDecoder(bytes.NewReader(encoded))
	dec.UseNumber()
	if dec.Decode(&wire) != nil {
		return nil, errors.New("malformed result metadata")
	}
	for _, key := range reviewSortedKeys(wire) {
		if key == "content" || key == "structuredContent" {
			continue
		}
		wire[key] = compactReviewValue(wire[key], "/"+reviewPointerToken(key), omitted)
	}
	wire["details_complete"] = false
	wire["elided"] = map[string]any{"fields": omitted}
	fit := func() (map[string]any, bool, error) {
		payload["elided"] = map[string]any{"fields": omitted, "how_to_get_more": "narrow changed_files; query individual review dimensions"}
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, false, errors.New("malformed compact review")
		}
		wire["content"] = []any{map[string]any{"type": "text", "text": string(b)}}
		wire["structuredContent"] = json.RawMessage(b)
		encoded, err := json.Marshal(wire)
		if err != nil {
			return nil, false, errors.New("malformed compact result")
		}
		return wire, len(encoded) <= limit, nil
	}
	if out, fits, err := fit(); err != nil || fits {
		return out, err
	}
	// The status map is the mandatory floor. Detailed dimension headers may
	// also be omitted, explicitly, when long reasons consume the remaining room.
	payload["dimensions"] = []any{}
	for path, loss := range omitted {
		if strings.HasPrefix(path, "/dimensions/") {
			loss.Returned = 0
			omitted[path] = loss
		}
	}
	omitted["/dimensions"] = reviewOmission{Total: len(dims), Unit: "items"}
	if out, fits, err := fit(); err != nil || fits {
		return out, err
	}
	return nil, errors.New("eight review statuses and omission metadata cannot fit")
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
