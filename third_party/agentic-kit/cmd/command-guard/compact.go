package main

import (
	"encoding/json"
	"reflect"
	"strings"
)

// compactObject preserves every ordinary header, including future fields.
// Row eligibility is checked separately so an extended row cannot lose data.
func compactObject(ordinary []byte, version, layout string, rows any) []byte {
	var object map[string]json.RawMessage
	if json.Unmarshal(ordinary, &object) != nil {
		return ordinary
	}
	// A new ordinary header requires a deliberate compact schema update.
	// Retain its ordinary wire until this closed profile knows the field.
	known := "version status code noActionAuthority scanner externalScanner externalClassifier limitation rows "
	switch version {
	case "command-guard.compact-report/1":
		known += "filesRead bytesRead bytesScanned scope totalMatches returned omitted truncated"
	case "command-guard.compact-git-report/1":
		known += "operation scope baseCommit targetCommit attemptedCalls startedCalls completedCalls bytesScanned returned omitted omittedLines omittedBytes projectionTruncated hasMore nonAtomic"
	default:
		return ordinary
	}
	for field := range object {
		if !strings.Contains(" "+known+" ", " "+field+" ") {
			return ordinary
		}
	}
	object["version"], _ = json.Marshal(version)
	object["rowFormat"], _ = json.Marshal(layout)
	object["rows"], _ = json.Marshal(rows)
	candidate, err := json.Marshal(object)
	if err != nil || len(candidate) >= len(ordinary) {
		return ordinary
	}
	return candidate
}

func exactRow(v any, allowed map[string]bool) bool {
	// These command-local rows are plain structs. Reject custom/embedded wire
	// behavior instead of serializing and parsing every row on every cap probe.
	if _, custom := v.(json.Marshaler); custom {
		return false
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Struct {
		return false
	}
	if reflect.PointerTo(rv.Type()).Implements(reflect.TypeFor[json.Marshaler]()) {
		return false
	}
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.Anonymous {
			return false
		}
		if f.PkgPath != "" {
			continue
		}
		key, options, _ := strings.Cut(f.Tag.Get("json"), ",")
		if key == "-" {
			continue
		}
		if key == "" || options != "" && options != "omitempty" {
			return false
		}
		if allowed[key] {
			continue
		}
		if options != "omitempty" {
			return false
		}
		// Conservatively accept only the ordinary empty forms used here.
		// Any unfamiliar kind retains the entire ordinary report.
		value := rv.Field(i)
		switch value.Kind() {
		case reflect.String, reflect.Slice:
			if value.Len() == 0 {
				continue
			}
		case reflect.Int:
			if value.Int() == 0 {
				continue
			}
		}
		return false
	}
	return true
}

func compactInspect(r report, ordinary []byte) []byte {
	if r.Status != "no-findings" || r.Code != "ok" {
		return ordinary
	}
	rows := make([][3]any, 0, len(r.Rows))
	allowed := map[string]bool{"file": true, "line": true, "text": true}
	for _, value := range r.Rows {
		if !exactRow(value, allowed) {
			return ordinary
		}
		rows = append(rows, [3]any{value.File, value.Line, value.Text})
	}
	return compactObject(ordinary, "command-guard.compact-report/1", "file-line-text/1", rows)
}

func compactGit(r gitReport, ordinary []byte) []byte {
	if r.Status != "no-findings" || r.Code != "ok" {
		return ordinary
	}
	var layout string
	var allowed map[string]bool
	rows := make([][]any, 0, len(r.Rows))
	switch r.Operation {
	case "git-status":
		layout = "status/1"
		allowed = map[string]bool{"kind": true, "path": true, "xy": true}
		for _, v := range r.Rows {
			if v.Kind != "tracked" && v.Kind != "untracked" || !exactRow(v, allowed) {
				return ordinary
			}
			rows = append(rows, []any{v.Kind, v.Path, v.XY})
		}
	case "git-object-diff":
		layout = "object-diff/1"
		allowed = map[string]bool{"kind": true, "path": true, "beforeOid": true, "afterOid": true, "before": true, "after": true, "beforeOmittedLines": true, "afterOmittedLines": true}
		for _, v := range r.Rows {
			if v.Kind != "change" || !exactRow(v, allowed) {
				return ordinary
			}
			before := append([]string{}, v.Before...)
			after := append([]string{}, v.After...)
			rows = append(rows, []any{v.Path, v.BeforeOID, v.AfterOID, before, after, v.BeforeOmittedLines, v.AfterOmittedLines})
		}
	case "git-history":
		layout = "history/1"
		allowed = map[string]bool{"kind": true, "oid": true, "parentIds": true, "author": true, "committer": true, "message": true, "messageOmittedBytes": true}
		for _, v := range r.Rows {
			if v.Kind != "commit" || !exactRow(v, allowed) {
				return ordinary
			}
			parents := append([]string{}, v.ParentIDs...)
			rows = append(rows, []any{v.OID, parents, v.Author, v.Committer, v.Message, v.MessageOmittedBytes})
		}
	default:
		return ordinary
	}
	return compactObject(ordinary, "command-guard.compact-git-report/1", layout, rows)
}
