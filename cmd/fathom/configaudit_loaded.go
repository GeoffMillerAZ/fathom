package main

// configaudit_loaded.go — RO-41 (docs/design/readouts.md §20, H11):
// `fathom config audit --loaded [--days N]`. Per instruction file the
// harness loaded in the window: its bytes at HEAD, its loads, and the
// served claims compiled from it. Three reads, none through the daemon:
// the InstructionsLoaded beats in the beat log (file_path, memory_type,
// load_reason), the rule outcome ledger's `served` rows, and the claims in
// the knowledge store opened read-only (resolveKnowledgeStore, the way
// `claim learn` opens it), a claim's source file being its Subject up to
// '#'. It recommends nothing (P11).

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// configAuditLoadedNow is the audit's clock (a var so a test pins it).
var configAuditLoadedNow = time.Now

// configAuditLoadedFile is one instruction file's row.
type configAuditLoadedFile struct {
	File      string `json:"file"`
	Bytes     int64  `json:"bytes"`
	BytesFrom string `json:"bytes_from"` // "HEAD" | "disk" | "none"
	EstTokens int64  `json:"est_tokens"` // bytes/4, an estimate
	Loads     int    `json:"loads"`
	Native    bool   `json:"native,omitempty"`
	Served    int    `json:"served"`
	Status    string `json:"status"`
}

// configAuditLoadedReport is the --json shape.
type configAuditLoadedReport struct {
	Window      string                  `json:"window"`
	WindowStart string                  `json:"window_start,omitempty"`
	Store       string                  `json:"store"`
	StoreNote   string                  `json:"store_note,omitempty"`
	Files       []configAuditLoadedFile `json:"files"`
}

func cmdConfigAuditLoaded(root string, days int, storeFlag string, asJSON bool, stdout, stderr io.Writer) int {
	rep, err := auditLoadedInstructions(root, days, storeFlag, configAuditLoadedNow().UTC())
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return jsonExit(enc.Encode(rep), stderr)
	}
	fmt.Fprintf(stdout, "instruction files loaded, window %s", rep.Window)
	if rep.WindowStart != "" {
		fmt.Fprintf(stdout, " (since %s)", rep.WindowStart)
	}
	fmt.Fprintf(stdout, "; claims read from %s\n", rep.Store)
	if rep.StoreNote != "" {
		fmt.Fprintf(stdout, "note: %s\n", rep.StoreNote)
	}
	for _, f := range rep.Files {
		bytesTxt := "bytes unknown"
		if f.BytesFrom != "none" {
			bytesTxt = fmt.Sprintf("%d bytes at %s, est. tokens %d (bytes/4)", f.Bytes, f.BytesFrom, f.EstTokens)
		}
		loads := fmt.Sprintf("%d loads", f.Loads)
		if f.Native {
			loads = "no load events"
		}
		served := ""
		if f.Served > 0 {
			served = fmt.Sprintf(", %d served", f.Served)
		}
		fmt.Fprintf(stdout, "%s: %s, %s%s: %s\n", f.File, bytesTxt, loads, served, f.Status)
	}
	if len(rep.Files) == 0 {
		fmt.Fprintln(stdout, "no instruction file load in the window")
	}
	return 0
}

// auditLoadedInstructions builds the report at now.
func auditLoadedInstructions(root string, days int, storeFlag string, now time.Time) (configAuditLoadedReport, error) {
	ctx := context.Background()
	rep := configAuditLoadedReport{Files: []configAuditLoadedFile{}}
	realRoot := resolveSymlinks(root)

	beats, _, err := loadInstructionsLoadedBeats()
	if err != nil {
		return rep, fmt.Errorf("beat log: %w", err)
	}
	var cutoff time.Time
	if days > 0 {
		cutoff = now.AddDate(0, 0, -days)
	}
	loads := map[string]int{}
	var start time.Time
	for _, b := range beats {
		if !cutoff.IsZero() && b.At.Before(cutoff) {
			continue
		}
		rel, ok := repoRel(realRoot, b.Meta["file_path"])
		if !ok {
			continue
		}
		loads[rel]++
		if start.IsZero() || b.At.Before(start) {
			start = b.At
		}
	}
	// ADR-059: AGENTS.md with no root CLAUDE file is loaded natively, with no event.
	native := map[string]bool{}
	if loads["AGENTS.md"] == 0 && gitHasFile(ctx, root, "AGENTS.md") && !gitHasFile(ctx, root, "CLAUDE.md") && !gitHasFile(ctx, root, ".claude/CLAUDE.md") {
		native["AGENTS.md"] = true
	}
	if start.IsZero() {
		start = cutoff
	}
	if !start.IsZero() {
		rep.WindowStart = start.UTC().Format(time.RFC3339Nano)
		rep.Window = windowLabel(now.Sub(start))
	} else {
		rep.Window = "all time"
	}

	// Claims: which file each compiled claim came from.
	claimFile := map[string]string{}
	compiled := map[string]bool{}
	ks := resolveKnowledgeStore(storeFlag, "")
	rep.Store = ks.String()
	if st, err := openKnowledgeStore(ks); err != nil {
		rep.StoreNote = "no store read, so no file reads as compiled: " + err.Error()
	} else {
		defer st.Close()
		ids, _ := st.ClaimServiceIDs(ctx)
		for _, id := range ids {
			if meta, found, err := st.GetServiceMeta(ctx, id); err != nil || !found || resolveSymlinks(meta.Path) != realRoot {
				continue
			}
			cs, err := st.ClaimsForService(ctx, id, "")
			if err != nil {
				return rep, fmt.Errorf("%s: load claims: %w", id, err)
			}
			for _, c := range cs {
				f, _, _ := strings.Cut(c.Subject, "#")
				f = filepath.ToSlash(filepath.Clean(f))
				claimFile[c.ClaimID] = f
				compiled[f] = true
			}
		}
	}
	served := map[string]int{}
	for _, ev := range readRuleOutcomeLogFrom(ruleOutcomeLogPath(), time.Time{}).Events {
		if ev.Event != semantic.RuleEventServed || (!start.IsZero() && ev.At.Before(start)) {
			continue
		}
		if f, ok := claimFile[ev.RuleID]; ok {
			served[f]++
		}
	}

	files := map[string]bool{}
	for f := range loads {
		files[f] = true
	}
	for f := range native {
		files[f] = true
	}
	for f := range files {
		row := configAuditLoadedFile{File: f, Loads: loads[f], Native: native[f], Served: served[f]}
		row.Bytes, row.BytesFrom = bytesAtHead(ctx, root, f)
		if row.BytesFrom != "none" {
			row.EstTokens = row.Bytes / 4
		}
		switch {
		case row.Native:
			row.Status = "loaded natively, no event"
		case !compiled[f]:
			row.Status = "not compiled"
		case row.Served == 0:
			row.Status = "0 served in " + rep.Window
		default:
			row.Status = "served"
		}
		rep.Files = append(rep.Files, row)
	}
	sort.Slice(rep.Files, func(i, j int) bool { return rep.Files[i].File < rep.Files[j].File })
	return rep, nil
}

// repoRel returns p relative to root (slash form) when p is inside it.
func repoRel(root, p string) (string, bool) {
	if p == "" {
		return "", false
	}
	rel, err := filepath.Rel(root, resolveSymlinks(p))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func windowLabel(d time.Duration) string {
	if d >= 24*time.Hour {
		return fmt.Sprintf("%dd", int(d/(24*time.Hour)))
	}
	return fmt.Sprintf("%dh", int(d/time.Hour))
}

func gitHasFile(ctx context.Context, root, rel string) bool {
	return exec.CommandContext(ctx, "git", "-C", root, "cat-file", "-e", "HEAD:"+rel).Run() == nil
}

// bytesAtHead is the file's size at HEAD, else on disk, else none.
func bytesAtHead(ctx context.Context, root, rel string) (int64, string) {
	if out, err := exec.CommandContext(ctx, "git", "-C", root, "cat-file", "-s", "HEAD:"+rel).Output(); err == nil {
		var n int64
		if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &n); err == nil {
			return n, "HEAD"
		}
	}
	if fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
		return fi.Size(), "disk"
	}
	return 0, "none"
}
