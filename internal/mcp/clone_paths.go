package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/worktree"
)

type clonePathKey struct{}
type clonePathValue struct{ resolution worktree.Resolution }

func clonePath(ctx context.Context, service string) (worktree.Resolution, bool) {
	value, ok := ctx.Value(clonePathKey{}).(clonePathValue)
	r := value.resolution
	return r, ok && r.Class != "" && (service == "" || r.Service == service)
}

func resolveMCPPath(ctx context.Context, b QueryInterface, target string) (worktree.Resolution, bool, error) {
	if b == nil || !filepath.IsAbs(target) {
		return worktree.Resolution{}, false, nil
	}
	services, err := b.Services(ctx)
	if err != nil {
		return worktree.Resolution{}, false, err
	}
	var roots []worktree.IndexedRoot
	for _, service := range services {
		root, commit, found, e := b.ServiceMeta(ctx, service)
		if e != nil {
			return worktree.Resolution{}, false, e
		}
		if found && filepath.IsAbs(root) {
			roots = append(roots, worktree.IndexedRoot{Service: service, Root: root, IndexedCommit: commit})
		}
	}
	r, ok := worktree.Resolve(ctx, target, roots)
	return r, ok, nil
}

// clonePathHandler translates only verified clone/worktree paths. The
// existing handlers retain their relative-path and source-read boundaries.
// The resulting note and unknown marks are included in the normal budget.
func clonePathHandler(name string, b QueryInterface, h func(context.Context, json.RawMessage) (any, *rpcError)) func(context.Context, json.RawMessage) (any, *rpcError) {
	switch name {
	case "search", "read_source", "find_symbol", "symbol_context", "blast_radius":
	default:
		return h
	}
	return func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		var p map[string]json.RawMessage
		if json.Unmarshal(args, &p) != nil {
			return h(ctx, args)
		}
		str := func(key string) string { var s string; _ = json.Unmarshal(p[key], &s); return s }
		target := str("path")
		service := str("service")
		if name == "search" {
			target = str("root")
		}
		if name == "read_source" {
			target = str("file")
		}
		var files []string
		if name == "blast_radius" {
			_ = json.Unmarshal(p["changed_files"], &files)
			if target == "" {
				for _, f := range files {
					if filepath.IsAbs(f) {
						target = f
						break
					}
				}
			}
		}
		if target == "" && filepath.IsAbs(service) {
			target = service
			service = ""
		}
		if !filepath.IsAbs(target) {
			return h(ctx, args)
		}
		r, ok, err := resolveMCPPath(ctx, b, target)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "resolve path: " + err.Error()}
		}
		if !ok {
			if name == "blast_radius" || name == "read_source" {
				return h(ctx, args)
			}
			return nil, &rpcError{Code: ErrCodeParams, Message: fmt.Sprintf("absolute path %q is not inside an indexed service's root or a verified clone; identity unknown", target)}
		}
		if service != "" && service != r.Service {
			return nil, &rpcError{Code: ErrCodeParams, Message: "path resolves to a different indexed service"}
		}
		if r.Class == "" {
			return h(ctx, args)
		}
		target = r.Target
		if name == "search" {
			p["root"], _ = json.Marshal(target)
		}
		if name != "search" {
			p["service"], _ = json.Marshal(r.Service)
		}
		if name == "read_source" {
			rel, e := filepath.Rel(r.Root, target)
			if e != nil {
				return nil, &rpcError{Code: ErrCodeParams, Message: e.Error()}
			}
			p["file"], _ = json.Marshal(filepath.ToSlash(rel))
		}
		if name == "blast_radius" {
			for i, f := range files {
				if filepath.IsAbs(f) {
					rel, e := filepath.Rel(r.Root, f)
					if e != nil || rel == ".." || strings.HasPrefix(rel, "../") {
						return nil, &rpcError{Code: ErrCodeParams, Message: "changed files must share one resolved checkout"}
					}
					files[i] = filepath.ToSlash(rel)
				}
			}
			p["changed_files"], _ = json.Marshal(files)
		}
		args, _ = json.Marshal(p)
		ctx = context.WithValue(ctx, clonePathKey{}, clonePathValue{r})
		var unknownFiles []map[string]string
		for _, file := range files {
			if same, reason := cloneFileState(ctx, b, r, file); !same {
				unknownFiles = append(unknownFiles, map[string]string{"file": file, "status": "unknown", "reason": reason})
			}
		}
		sort.Slice(unknownFiles, func(i, j int) bool { return unknownFiles[i]["file"] < unknownFiles[j]["file"] })
		out, rpcErr := h(ctx, args)
		if rpcErr != nil {
			// A missing base-index entry for new clone code is unknown, never absent.
			if len(unknownFiles) > 0 && strings.Contains(rpcErr.Message, "not indexed") {
				return map[string]any{"status": "unknown", "reason": "changed in clone", "unknown_files": unknownFiles, "freshness_note": r.Note}, nil
			}
			if name == "read_source" && str("ref") == "" && strings.Contains(rpcErr.Message, "file not found") {
				return map[string]any{"status": "unknown", "reason": "changed in clone", "freshness_note": r.Note}, nil
			}
			return out, rpcErr
		}
		data, err := json.Marshal(out)
		if err != nil {
			return out, nil
		}
		var body map[string]any
		if json.Unmarshal(data, &body) != nil {
			return out, nil
		}
		body["freshness_note"] = r.Note
		if len(unknownFiles) > 0 {
			body["unknown_files"] = unknownFiles
			body["status"] = "unknown"
			body["reason"] = "changed in clone"
			delete(body, "not_indexed")
			body["summary"] = "base reach; clone files unknown"
		}
		if name == "blast_radius" || name == "symbol_context" {
			if symbols := cloneSymbolStates(ctx, b, r, body, files); len(symbols) > 0 {
				body["unknown_symbols"] = symbols
				body["status"] = "unknown"
				body["reason"] = symbols[0]["reason"]
			}
		}
		// Mark index-backed symbol locations, including callers and tests. A
		// changed symbol cannot masquerade as an absent one after a lookup miss.
		unknown := markCloneLocations(ctx, b, r, body)
		if name != "search" && str("ref") == "" {
			rel, e := filepath.Rel(r.Root, target)
			info, statErr := os.Stat(target)
			if e == nil && (name == "read_source" || info != nil && !info.IsDir() || os.IsNotExist(statErr) && filepath.Ext(target) != "") {
				if same, reason := cloneFileState(ctx, b, r, filepath.ToSlash(rel)); !same {
					unknown = reason
				}
			}
			if unknown != "" {
				body["status"] = "unknown"
				body["reason"] = unknown
				delete(body, "not_found")
				delete(body, "not_indexed")
				delete(body, "hint")
				body["summary"] = "unknown (" + unknown + "); base index only"
			}
		}
		return body, nil
	}
}

func markCloneLocations(ctx context.Context, b QueryInterface, r worktree.Resolution, v any) string {
	reason := ""
	switch x := v.(type) {
	case map[string]any:
		if file, ok := x["file_path"].(string); ok && file != "" {
			if same, why := cloneFileState(ctx, b, r, file); !same {
				x["status"] = "unknown"
				x["reason"] = why
				reason = why
			}
		}
		// Map order must never decide the top-level reason.
		keys := make([]string, 0, len(x))
		for key := range x {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if why := markCloneLocations(ctx, b, r, x[key]); why != "" && (reason == "" || why < reason) {
				reason = why
			}
		}
	case []any:
		for _, child := range x {
			if why := markCloneLocations(ctx, b, r, child); why != "" && (reason == "" || why < reason) {
				reason = why
			}
		}
	}
	return reason
}

func cloneSearchKey(r worktree.Resolution, root, file string) string {
	prefix, err := filepath.Rel(r.Root, root)
	if err != nil {
		return file
	}
	return strings.TrimPrefix(filepath.ToSlash(filepath.Join(prefix, file)), "./")
}

// Stored fingerprints, when available, identify the indexed bytes even if
// the ingest included uncommitted changes. Commit blobs are a conservative
// fallback for backends without the fingerprint port.
func cloneFileState(ctx context.Context, b QueryInterface, r worktree.Resolution, file string) (bool, string) {
	if same, reason := r.Unchanged(ctx, file); !same {
		return false, reason
	}
	if _, known := indexedSourceFingerprint(ctx, b, r.Service, file); known {
		_, status, _, err := readSourceBytes(ctx, b, r.Service, file, false)
		if err != nil {
			return false, "clone file identity unknown"
		}
		if status.State == "" {
			return true, ""
		}
		if status.State == "stale" {
			return false, "changed in clone"
		}
		return false, status.Reason
	}
	return true, ""
}

// String-key reach lists keep their existing shape. Their changed members
// are qualified by this additive, sorted projection from the same index.
func cloneSymbolStates(ctx context.Context, b QueryInterface, r worktree.Resolution, body any, files []string) []map[string]string {
	keys := map[string]bool{}
	var collect func(any)
	collect = func(v any) {
		switch x := v.(type) {
		case string:
			keys[x] = true
		case []any:
			for _, v := range x {
				collect(v)
			}
		case map[string]any:
			for _, v := range x {
				collect(v)
			}
		}
	}
	collect(body)
	requested := map[string]bool{}
	for _, file := range files {
		requested[file] = true
	}
	nodes, err := serviceSymbols(ctx, b, r.Service)
	if err != nil {
		return nil
	}
	nodes = slices.Clone(nodes)
	sort.Slice(nodes, func(i, j int) bool { return nodeLess(nodes[i], nodes[j]) })
	var out []map[string]string
	for _, n := range nodes {
		if !keys[n.Key()] && !requested[n.FilePath] {
			continue
		}
		if same, reason := cloneFileState(ctx, b, r, n.FilePath); !same {
			out = append(out, map[string]string{"symbol": n.Key(), "file": n.FilePath, "status": "unknown", "reason": reason})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i]["symbol"] != out[j]["symbol"] {
			return out[i]["symbol"] < out[j]["symbol"]
		}
		return out[i]["file"] < out[j]["file"]
	})
	return out
}
