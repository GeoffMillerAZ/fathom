package main

// ingest_dryrun.go — RT-70 (docs/design/responsible-tenant.md §4.2):
// `fathom ingest --dry-run --stats` measures what an ingest of a tree would
// discover and read, from stats alone. It never reads a file's content and
// never opens the store, so it is safe to run over any registered service
// on a shared machine — the before/after instrument for discovery changes
// (RT-M11).

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// ingestDryRunLargest is how many of the largest files the summary names.
const ingestDryRunLargest = 5

// ingestDryRunStats is the --dry-run --stats result (--json shape).
type ingestDryRunStats struct {
	Service string `json:"service"`
	Path    string `json:"path"`
	DryRun  bool   `json:"dry_run"`
	// Files and Bytes are every discovered file and the sum of their stat
	// sizes — what pass 2 of an ingest would consider.
	Files int   `json:"files"`
	Bytes int64 `json:"bytes"`
	// OversizeFiles/OversizeBytes are the discovered files over the
	// connector's read cap (connector.MaxFetchSize): counted, never read.
	OversizeFiles int   `json:"oversize_files"`
	OversizeBytes int64 `json:"oversize_bytes"`
	ReadCapBytes  int64 `json:"read_cap_bytes"`
	// DefaultIgnoredDirs names each directory the default cache list
	// pruned; SkippedDirs and IgnoredDirs count skipDir and ignore-file
	// prunes (RT-145).
	DefaultIgnoredDirs []string `json:"default_ignored_dirs"`
	SkippedDirs        int      `json:"skipped_dirs"`
	IgnoredDirs        int      `json:"ignored_dirs"`
	// StatFailed counts discovered files whose stat failed; their size is
	// not in Bytes, so Bytes is then a lower bound (I38).
	StatFailed int                    `json:"stat_failed"`
	Largest    []ingestDryRunFileJSON `json:"largest"`
}

type ingestDryRunFileJSON struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// runIngestDryRunStats discovers svc through conn and stats every
// discovered file under svc.Path. It calls Discover only — never Fetch.
func runIngestDryRunStats(ctx context.Context, conn connector.Connector, svc core.ServiceRef, stdout io.Writer, jsonOut bool) int {
	refs, err := conn.Discover(ctx, svc)
	if err != nil {
		fmt.Fprintf(stdout, "error: discover %s: %v\n", svc.Path, err)
		return 1
	}
	out := ingestDryRunStats{
		Service:            svc.ID,
		Path:               svc.Path,
		DryRun:             true,
		ReadCapBytes:       connector.MaxFetchSize,
		DefaultIgnoredDirs: []string{},
		Largest:            []ingestDryRunFileJSON{},
	}
	if er, ok := conn.(connector.ExclusionReporter); ok {
		ex := er.Exclusions(svc.ID)
		if ex.DefaultIgnored != nil {
			out.DefaultIgnoredDirs = ex.DefaultIgnored
		}
		out.SkippedDirs = ex.SkippedDirs
		out.IgnoredDirs = ex.IgnoredDirs
	}
	var sized []ingestDryRunFileJSON
	for _, ref := range refs {
		p := ref.Path
		if !filepath.IsAbs(p) {
			p = filepath.Join(svc.Path, filepath.FromSlash(p))
		}
		fi, serr := os.Lstat(p)
		if serr != nil {
			out.StatFailed++
			continue
		}
		out.Files++
		out.Bytes += fi.Size()
		if fi.Size() > connector.MaxFetchSize {
			out.OversizeFiles++
			out.OversizeBytes += fi.Size()
		}
		sized = append(sized, ingestDryRunFileJSON{Path: ref.Path, Size: fi.Size()})
	}
	sort.SliceStable(sized, func(i, j int) bool {
		if sized[i].Size != sized[j].Size {
			return sized[i].Size > sized[j].Size
		}
		return sized[i].Path < sized[j].Path
	})
	if len(sized) > ingestDryRunLargest {
		sized = sized[:ingestDryRunLargest]
	}
	out.Largest = append(out.Largest, sized...)

	if jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "service:    %s (dry run: stat only, nothing read or written)\n", out.Service)
	fmt.Fprintf(stdout, "path:       %s\n", out.Path)
	fmt.Fprintf(stdout, "files:      %d\n", out.Files)
	fmt.Fprintf(stdout, "bytes:      %s\n", humanBytes(out.Bytes))
	fmt.Fprintf(stdout, "oversize:   %d file(s), %s over the %s read cap (counted, never read)\n",
		out.OversizeFiles, humanBytes(out.OversizeBytes), humanBytes(out.ReadCapBytes))
	printDefaultIgnoredDirs(stdout, out.DefaultIgnoredDirs)
	fmt.Fprintf(stdout, "pruned:     %d always-skipped dir(s) (vendor, .git, testdata, …), %d by ignore files\n", out.SkippedDirs, out.IgnoredDirs)
	if out.StatFailed > 0 {
		fmt.Fprintf(stdout, "stat-failed: %d file(s) — bytes is a lower bound\n", out.StatFailed)
	}
	if len(out.Largest) > 0 {
		fmt.Fprintf(stdout, "largest:\n")
		for _, f := range out.Largest {
			fmt.Fprintf(stdout, "  %s  %s\n", humanBytes(f.Size), f.Path)
		}
	}
	return 0
}

// humanBytes renders n in binary units.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
