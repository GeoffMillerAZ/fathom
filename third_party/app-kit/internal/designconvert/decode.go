package designconvert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"runtime"
	"strings"
	"unicode/utf8"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// MaxTranslationBytes bounds canonical JSON before decoding. A 4 MiB artifact
// can expand sixfold under JSON escaping, with additional byte-map metadata.
const MaxTranslationBytes = 32 << 20

// CurrentTransformer identifies the current emitter, not a compatibility claim.
// Storage and decoding live outside the emitter digest so changing persistence
// cannot invalidate an otherwise identical syntax translation.
func CurrentTransformer() Transformer {
	return Transformer{RuleID, ruleDigest(), runtime.Version(), HTMLParserVersion, RequiredTempl}
}

// DecodeCanonical accepts only the exact compact JSON representation emitted by
// json.Marshal(Translation). This rejects duplicate/case-aliased/unknown fields,
// alternate field ordering, trailing data and lossy invalid UTF-8 decoding. It
// validates historical syntax envelopes without asserting current-rule replay.
// Callers must still bind Source to a verified archive and check VerifyReplay
// before describing a record as reproducible under the current transformer.
func DecodeCanonical(body []byte) (Translation, error) {
	var result Translation
	if len(body) > MaxTranslationBytes || !utf8.Valid(body) || !json.Valid(body) {
		return result, fmt.Errorf("designconvert: stored translation size or JSON encoding invalid")
	}
	if err := preflightTranslationJSON(body); err != nil {
		return result, err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return Translation{}, fmt.Errorf("designconvert: decode stored translation: %w", err)
	}
	if err := validateReceiptBounds(result); err != nil {
		return Translation{}, err
	}
	ctx := cuecontext.New()
	schema, err := implementation.ReadFile("schema.cue")
	if err != nil {
		return Translation{}, err
	}
	value := ctx.CompileBytes(body).Unify(ctx.CompileBytes(schema).LookupPath(cue.ParsePath("#StoredTranslation")))
	if err := value.Validate(cue.Concrete(true)); err != nil {
		return Translation{}, fmt.Errorf("designconvert: stored translation schema rejected")
	}
	canonical, err := json.Marshal(result)
	if err != nil || !bytes.Equal(body, canonical) {
		return Translation{}, fmt.Errorf("designconvert: stored translation is not canonical JSON")
	}
	source := result.Source
	if source.Path == "." || source.Path == ".." || path.Clean(source.Path) != source.Path ||
		strings.HasPrefix(source.Path, "/") || strings.HasPrefix(source.Path, "../") ||
		strings.ContainsAny(source.Path, "\\\x00\r\n") {
		return Translation{}, fmt.Errorf("designconvert: stored source path is not an exact archive-relative member")
	}
	for _, d := range result.Diagnostics {
		if d.Offset > source.Size {
			return Translation{}, fmt.Errorf("designconvert: stored diagnostic exceeds original source")
		}
	}
	if a := result.Artifact; a != nil {
		if a.SHA256 != digest([]byte(a.Content)) {
			return Translation{}, fmt.Errorf("designconvert: stored artifact hash mismatch")
		}
		for _, m := range a.Mappings {
			if m.SourceEnd > source.Size || m.OutputEnd > len(a.Content) {
				return Translation{}, fmt.Errorf("designconvert: stored mapping exceeds original or artifact")
			}
		}
	}
	return result, nil
}

// Bound arrays before typed decoding: a compact JSON array of empty objects
// must not allocate millions of mappings before the receipt limit rejects it.
func preflightTranslationJSON(body []byte) error {
	d := json.NewDecoder(bytes.NewReader(body))
	var walk func(depth, arrayLimit int) error
	walk = func(depth, arrayLimit int) error {
		if depth > 8 {
			return fmt.Errorf("designconvert: stored JSON nesting exceeds limit")
		}
		token, err := d.Token()
		if err != nil {
			return fmt.Errorf("designconvert: invalid stored JSON")
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			keys := map[string]bool{}
			for d.More() {
				token, err := d.Token()
				key, ok := token.(string)
				if err != nil || !ok || len(key) > 128 || keys[key] || len(keys) >= 16 {
					return fmt.Errorf("designconvert: duplicate or invalid stored JSON key")
				}
				keys[key] = true
				limit := 0
				if depth == 0 && key == "diagnostics" {
					limit = MaxDiagnostics
				} else if depth == 1 && key == "mappings" {
					limit = maxTokens
				}
				if err := walk(depth+1, limit); err != nil {
					return err
				}
			}
			if end, err := d.Token(); err != nil || end != json.Delim('}') {
				return fmt.Errorf("designconvert: invalid stored JSON object")
			}
		case '[':
			count := 0
			for d.More() {
				if count >= arrayLimit {
					return fmt.Errorf("designconvert: stored JSON array exceeds limit")
				}
				count++
				if err := walk(depth+1, 0); err != nil {
					return err
				}
			}
			if end, err := d.Token(); err != nil || end != json.Delim(']') {
				return fmt.Errorf("designconvert: invalid stored JSON array")
			}
		default:
			return fmt.Errorf("designconvert: invalid stored JSON delimiter")
		}
		return nil
	}
	if err := walk(0, 0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("designconvert: trailing stored JSON")
	}
	return nil
}
