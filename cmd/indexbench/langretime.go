package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

const retimeNotePrefix = "throughput (design 1.9 T1, T2)"
const retimeUsage = "usage: indexbench lang-retime -reading <reading.json> -subject <name>=<dir> [-subject ...] [-runs 3] [-write] [-compile-db <file>] [-load-log <workspace/file.json>]"
const retimeReadingCap = 16 << 20

type retimeLoadRun struct {
	Number     int     `json:"number"`
	RateMBCPUS float64 `json:"rate_mb_cpu_s"`
	LoadBefore float64 `json:"load_before"`
	LoadAfter  float64 `json:"load_after"`
}

type retimeLoadObserver struct {
	read  func() (float64, error)
	start float64
}

var retimeLoadReader = retimeOneMinuteLoad
var retimeProductIdentityReader = retimeProductIdentity

type retimeProduct struct {
	revision     string
	binarySHA256 string
}

func retimeProductIdentity() (retimeProduct, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return retimeProduct{}, fmt.Errorf("product build info unavailable")
	}
	var revision, modified string
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value
		}
	}
	if revision == "" || modified != "false" {
		return retimeProduct{}, fmt.Errorf("product VCS identity missing or modified")
	}
	path, err := os.Executable()
	if err != nil {
		return retimeProduct{}, err
	}
	hash, err := retimeBoundedBinarySHA(path)
	if err != nil {
		return retimeProduct{}, err
	}
	return retimeProduct{revision: revision, binarySHA256: hash}, nil
}

func retimeBoundedBinarySHA(path string) (string, error) {
	const maxBinaryBytes = 128 << 20
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !stat.Mode().IsRegular() || stat.Size() > maxBinaryBytes {
		return "", fmt.Errorf("product executable is not a regular file at most 128 MiB")
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, maxBinaryBytes+1))
	if err != nil {
		return "", err
	}
	if n > maxBinaryBytes {
		return "", fmt.Errorf("product executable exceeds 128 MiB")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (o *retimeLoadObserver) before() error {
	var err error
	o.start, err = o.read()
	return err
}

func (o *retimeLoadObserver) after(number int, rate float64) (retimeLoadRun, error) {
	end, err := o.read()
	return retimeLoadRun{Number: number, RateMBCPUS: rate, LoadBefore: o.start, LoadAfter: end}, err
}

func retimeOneMinuteLoad() (float64, error) {
	var raw []byte
	var err error
	switch runtime.GOOS {
	case "darwin":
		ctx, cancel := context.WithTimeout(context.Background(), retimeProbeTimeout)
		defer cancel()
		raw, err = exec.CommandContext(ctx, "sysctl", "-n", "vm.loadavg").Output()
	case "linux":
		raw, err = os.ReadFile("/proc/loadavg")
	default:
		return 0, fmt.Errorf("one-minute load is unavailable on %s", runtime.GOOS)
	}
	if err != nil {
		return 0, fmt.Errorf("one-minute load unavailable: %w", err)
	}
	fields := strings.Fields(strings.Trim(strings.TrimSpace(string(raw)), "{} "))
	if len(fields) == 0 {
		return 0, fmt.Errorf("one-minute load missing")
	}
	load, err := strconv.ParseFloat(fields[0], 64)
	if err != nil || load < 0 || math.IsNaN(load) || math.IsInf(load, 0) {
		return 0, fmt.Errorf("invalid one-minute load %q", fields[0])
	}
	return load, nil
}

// openRetimeLoadLog reserves a new regular file under the current checkout's
// workspace. Every parent from workspace to the file must already exist and
// be a real directory, so a symlink cannot redirect the log elsewhere.
func openRetimeLoadLog(path, workspace string) (*os.File, error) {
	workspace, err := filepath.Abs(workspace)
	if err != nil {
		return nil, err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(workspace, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("load log must be a new file under workspace/")
	}
	parent := filepath.Dir(path)
	for dir := parent; ; dir = filepath.Dir(dir) {
		info, err := os.Lstat(dir)
		if err != nil {
			return nil, fmt.Errorf("load log parent: %w", err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("load log parent is not a regular directory: %s", dir)
		}
		if dir == workspace {
			break
		}
	}
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
}

func retimeSHA(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// retimeBoundedReading parses and hashes exactly the same bounded snapshot.
func retimeBoundedReading(path string) (langbench.Reading, string, error) {
	var reading langbench.Reading
	f, err := os.Open(path)
	if err != nil {
		return reading, "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, retimeReadingCap+1))
	if err != nil {
		return reading, "", err
	}
	if len(data) > retimeReadingCap {
		return reading, "", fmt.Errorf("reading exceeds 16 MiB")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&reading); err != nil {
		return reading, "", fmt.Errorf("%s: %w", path, err)
	}
	return reading, retimeSHA(data), nil
}

// retimeProbeTimeout bounds the short host probes lang-retime runs (the load
// average and the checkout root), so a hung subprocess cannot stall a retime.
const retimeProbeTimeout = 10 * time.Second

func retimeWorkspaceRoot() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), retimeProbeTimeout)
	defer cancel()
	root, err := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("checkout root unavailable: %w", err)
	}
	workspace := filepath.Join(strings.TrimSpace(string(root)), "workspace")
	if err := exec.CommandContext(ctx, "git", "-C", strings.TrimSpace(string(root)), "check-ignore", "-q", "--", "workspace/").Run(); err != nil {
		return "", fmt.Errorf("checkout workspace is not git-ignored: %w", err)
	}
	return workspace, nil
}

func cmdLangRetime(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lang-retime", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("reading", "", "reading to re-time (required)")
	runs := fs.Int("runs", 3, "number of runs (at least 3)")
	write := fs.Bool("write", false, "save the re-timed reading")
	loadLog := fs.String("load-log", "", "write per-run one-minute system load and throughput to a new workspace JSON file")
	tenancyOnly := fs.Bool("tenancy-only", false, "check identity on the timing run's own facts only (sha, a clean tree, files, bytes, faults, parse errors), for a reading whose counts were measured on a different scope than its tenancy block; the note says so")
	compileDB := fs.String("compile-db", "", "companion compile_commands.json whose directory and paths are relative to the subject root; written to <subject>/build/compile_commands.json before measuring (refused when a different database is already there), for a subject checkout that lacks the recorded build directory")
	var subjects multiFlag
	fs.Var(&subjects, "subject", "recorded subject name=checkout directory (repeatable, required for every subject)")
	fs.Usage = func() { fmt.Fprintln(stderr, retimeUsage); fs.PrintDefaults() }
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *path == "" || *runs < 3 || fs.NArg() != 0 {
		fs.Usage()
		return 2
	}
	roots := map[string]string{}
	for _, value := range subjects {
		name, dir := splitNamed(value)
		if name == "" || dir == "" || roots[name] != "" {
			fmt.Fprintln(stderr, "lang-retime: -subject requires a unique name=dir")
			return 2
		}
		roots[name] = dir
	}
	refuse := func(err error) int { fmt.Fprintln(stderr, "lang-retime: refused:", err); return 1 }
	var readingBefore string
	var r langbench.Reading
	var err error
	if *loadLog != "" {
		r, readingBefore, err = retimeBoundedReading(*path)
	} else {
		r, err = loadReading(*path)
	}
	if err != nil {
		return refuse(err)
	}
	if len(r.Subjects) == 0 {
		return refuse(fmt.Errorf("reading has no subjects"))
	}
	for _, sub := range r.Subjects {
		if roots[sub.Name] == "" {
			fmt.Fprintf(stderr, "lang-retime: missing -subject mapping for %s\n", sub.Name)
			return 2
		}
	}
	var loadFile *os.File
	loadLogSaved := false
	var product retimeProduct
	if *loadLog != "" {
		workspace, rootErr := retimeWorkspaceRoot()
		if rootErr != nil {
			return refuse(rootErr)
		}
		loadFile, err = openRetimeLoadLog(*loadLog, workspace)
		if err != nil {
			return refuse(err)
		}
		defer func() {
			loadFile.Close()
			if !loadLogSaved {
				os.Remove(*loadLog)
			}
		}()
		product, err = retimeProductIdentityReader()
		if err != nil {
			return refuse(err)
		}
	}
	ctx := context.Background()
	if *compileDB != "" {
		for _, sub := range r.Subjects {
			if err := retimeInstallCompileDB(*compileDB, roots[sub.Name]); err != nil {
				return refuse(fmt.Errorf("%s: -compile-db: %w", sub.Name, err))
			}
		}
	}
	spec, source, err := resolveBenchSpec(r.Dialect, "")
	if err != nil {
		return refuse(err)
	}
	if source != r.Extractor {
		return refuse(fmt.Errorf("extractor recorded %q, resolved %q; the reading needs a full re-measure", r.Extractor, source))
	}
	with, closeExtractor, err := langOracleExtractor(spec.Manifest, source)
	if err != nil {
		return refuse(err)
	}
	defer closeExtractor()
	// Subject order is only a traversal order; the saved reading retains its order.
	order := make([]int, len(r.Subjects))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool { return r.Subjects[order[i]].Name < r.Subjects[order[j]].Name })
	scopes := map[string][]string{}
	for _, sub := range r.Subjects {
		for _, note := range r.Notes {
			if globs, ok := parseOnlyScopeNote(sub.Name, note); ok {
				if old, exists := scopes[sub.Name]; exists && !slices.Equal(old, globs) {
					return refuse(fmt.Errorf("%s: conflicting -only disclosures", sub.Name))
				}
				scopes[sub.Name] = globs
			}
		}
	}
	type timedRun struct {
		subjects   []langbench.Subject
		throughput float64
		number     int
	}
	timings := make([]timedRun, *runs)
	values := make([]string, *runs)
	var loadRuns []retimeLoadRun
	loadObserver := retimeLoadObserver{read: retimeLoadReader}
	for run := range timings {
		if loadFile != nil {
			if err := loadObserver.before(); err != nil {
				return refuse(fmt.Errorf("run %d before: %w", run+1, err))
			}
		}
		timed := timedRun{subjects: make([]langbench.Subject, len(r.Subjects)), number: run + 1}
		var totalBytes int64
		var cpuMS float64
		for _, i := range order {
			original := r.Subjects[i]
			root := roots[original.Name]
			if err := retimeCleanSubject(ctx, root); err != nil {
				return refuse(fmt.Errorf("run %d %s: %w", run+1, original.Name, err))
			}
			sha := gitOut(ctx, root, "rev-parse", "HEAD")
			if sha == "" || sha != original.SHA {
				return refuse(fmt.Errorf("run %d %s: sha recorded %q, measured %q", run+1, original.Name, original.SHA, sha))
			}
			measured, err := measureSubject(ctx, spec, root, original.Name, oracleOpts{
				only: scopes[original.Name], noOracles: true,
				retimeKinds:   original.KindIntersection,
				retimeImports: original.Families[langbench.FamilyImports] != "",
			}, with)
			if err != nil {
				return refuse(fmt.Errorf("run %d %s: %w", run+1, original.Name, err))
			}
			measurementRefusal := func(err error) int {
				return refuse(fmt.Errorf("run %d %s: %w\n%s", run+1, original.Name, err, strings.Join(measured.notes, "\n")))
			}
			if err := retimeCleanSubject(ctx, root); err != nil {
				return measurementRefusal(err)
			}
			if measured.subject.SHA != original.SHA {
				return measurementRefusal(fmt.Errorf("sha changed during measurement"))
			}
			if diff := retimeIdentityDiff(original, measured.subject, r.Notes, *tenancyOnly); len(diff) > 0 {
				return measurementRefusal(fmt.Errorf("\n  %s\nextraction changed; the reading needs a full re-measure, not a re-time", strings.Join(diff, "\n  ")))
			}
			t := measured.subject.Tenancy
			if t.ExtractCPUMS <= 0 || math.IsNaN(t.ExtractCPUMS) || math.IsInf(t.ExtractCPUMS, 0) {
				return refuse(fmt.Errorf("run %d %s: CPU timing unavailable", run+1, original.Name))
			}
			timed.subjects[i] = measured.subject
			totalBytes += measured.subject.Bytes
			cpuMS += t.ExtractCPUMS
		}
		// Match langbench.Regrade's pooled CPU throughput and rounding.
		timed.throughput = math.Round(float64(totalBytes)/1e6/(cpuMS/1000)*100) / 100
		timings[run] = timed
		values[run] = fmt.Sprintf("run %d: %.2f MB/CPU-s", run+1, timed.throughput)
		if loadFile != nil {
			observed, err := loadObserver.after(run+1, timed.throughput)
			if err != nil {
				return refuse(fmt.Errorf("run %d after: %w", run+1, err))
			}
			loadRuns = append(loadRuns, observed)
		}
	}
	sort.SliceStable(timings, func(i, j int) bool { return timings[i].throughput < timings[j].throughput })
	// For an even number of runs choose the upper central run: timings must
	// come from one actual run, never an average of unrelated subject timings.
	median := timings[len(timings)/2]
	before := r.LevelMet
	for i := range r.Subjects {
		old, fresh := r.Subjects[i].Tenancy, median.subjects[i].Tenancy
		old.ExtractMS, old.ExtractCPUMS, old.MaxFileMS = fresh.ExtractMS, fresh.ExtractCPUMS, fresh.MaxFileMS
	}
	var earlier []string
	kept := make([]string, 0, len(r.Notes)+1)
	for _, note := range r.Notes {
		if strings.HasPrefix(note, retimeNotePrefix) {
			earlier = append(earlier, "earlier: "+note)
		} else {
			kept = append(kept, note)
		}
	}
	note := fmt.Sprintf("%s: %s; %s; median run %d: %.2f MB/CPU-s; every other field is the original measurement at its recorded fathom_commit %s", retimeNotePrefix, retimeMachineClass(ctx), strings.Join(values, "; "), median.number, median.throughput, r.FathomCommit)
	if *tenancyOnly {
		note += "; identity checked on the timing run's sha, clean tree, files, bytes, faults and parse errors only (-tenancy-only: the reading's counts come from a different scope than its tenancy block)"
	}
	if len(earlier) > 0 {
		note += "; " + strings.Join(earlier, "; ")
	}
	r.Notes = append(kept, note)
	sort.Strings(r.Notes)
	langbench.Regrade(&r)
	if err := r.Validate(); err != nil {
		return refuse(err)
	}
	if *write {
		data, err := langbench.MarshalReading(r)
		if err != nil {
			return refuse(err)
		}
		if err := os.WriteFile(*path, data, 0644); err != nil {
			return refuse(err)
		}
	}
	if loadFile != nil {
		_, readingAfter, err := retimeBoundedReading(*path)
		if err != nil {
			return refuse(err)
		}
		log := struct {
			Schema              string          `json:"schema"`
			ReadingBeforeSHA256 string          `json:"reading_before_sha256"`
			ReadingAfterSHA256  string          `json:"reading_after_sha256"`
			OS                  string          `json:"os"`
			Arch                string          `json:"arch"`
			ProductRevision     string          `json:"product_revision"`
			ProductBinarySHA256 string          `json:"product_binary_sha256"`
			ProductModified     bool            `json:"product_modified"`
			Runs                []retimeLoadRun `json:"runs"`
		}{"fathom-lang-retime-load-v1", readingBefore, readingAfter, runtime.GOOS, runtime.GOARCH, product.revision, product.binarySHA256, false, loadRuns}
		if err := json.NewEncoder(loadFile).Encode(log); err != nil {
			return refuse(err)
		}
		if err := loadFile.Close(); err != nil {
			return refuse(err)
		}
		loadLogSaved = true
	}
	for _, value := range values {
		fmt.Fprintln(stdout, value)
	}
	fmt.Fprintf(stdout, "median run %d: %.2f MB/CPU-s\n%s: level_met %s -> %s grade %+v\n", median.number, median.throughput, r.Dialect, before, r.LevelMet, r.Grade)
	return 0
}

func retimeCleanSubject(ctx context.Context, root string) error {
	status, err := exec.CommandContext(ctx, "git", "-C", root, "status", "--porcelain").Output()
	if err != nil {
		return fmt.Errorf("git status failed: %w", err)
	}
	if len(status) > 0 {
		return fmt.Errorf("dirty checkout (including untracked files); restore the recorded subject before a re-time or do a full re-measure")
	}
	return nil
}

// retimeIdentityDiff compares only facts measured before any oracle is read.
// Oracle-scored counts cannot be reconstructed by a re-time.
// retimeIdentityDiff lists what differs between the recorded and the
// re-measured extraction identity. tenancyOnly stops after the tenancy run's
// own facts, for a reading whose counts were scored on another scope.
func retimeIdentityDiff(recorded, measured langbench.Subject, notes []string, tenancyOnly bool) []string {
	// The compile database comes first: it is an input the subject's clean
	// checkout may lack (a gitignored build directory) and the cause of any
	// include-resolution difference below.
	diff := retimeCompileDBDiff(recorded, measured, notes)
	compare := func(field string, a, b any) {
		if fmt.Sprint(a) != fmt.Sprint(b) {
			diff = append(diff, fmt.Sprintf("%s: recorded %v -> measured %v", field, a, b))
		}
	}
	compare("files", recorded.Files, measured.Files)
	compare("bytes", recorded.Bytes, measured.Bytes)
	if recorded.Tenancy == nil || measured.Tenancy == nil {
		return append(diff, "tenancy: missing extraction identity")
	}
	compare("faults", recorded.Tenancy.Faults, measured.Tenancy.Faults)
	a, b := slices.Clone(recorded.Tenancy.FaultingFiles), slices.Clone(measured.Tenancy.FaultingFiles)
	sort.Strings(a)
	sort.Strings(b)
	compare("faulting_files", fmt.Sprintf("%q", a), fmt.Sprintf("%q", b))
	parseErrors := func(p *int) string {
		if p == nil {
			return "not measured"
		}
		return fmt.Sprint(*p)
	}
	compare("files_with_error", parseErrors(recorded.Tenancy.FilesWithError), parseErrors(measured.Tenancy.FilesWithError))
	if tenancyOnly {
		return diff
	}
	// Include zero values absent from sparse maps, plus future extraction-only
	// counters. Oracle-dependent counters in the reading stay untouched.
	keys := map[string]bool{
		"type_refs": true, langbench.CntFathomDefs: true,
		langbench.CntCallables: true, langbench.CntCallablesSigned: true,
	}
	for k := range measured.Counts {
		keys[k] = true
	}
	for _, k := range sortedKeys(keys) {
		compare("counts."+k, recorded.Counts[k], measured.Counts[k])
	}
	excluded := map[string]bool{}
	for _, counts := range []map[string]int{recorded.ExcludedByKind, measured.ExcludedByKind} {
		for k := range counts {
			if strings.HasPrefix(k, "fathom:") {
				excluded[k] = true
			}
		}
	}
	for _, k := range sortedKeys(excluded) {
		compare("excluded_by_kind."+k, recorded.ExcludedByKind[k], measured.ExcludedByKind[k])
	}
	return diff
}

// retimeCompileDBDiff compares the compile databases the recorded extraction
// read with the ones this checkout offers. A legacy reading has no identity,
// only the include-path note's database count.
func retimeCompileDBDiff(recorded, measured langbench.Subject, notes []string) []string {
	if measured.Tenancy == nil || measured.Tenancy.CompileDB == nil {
		return nil
	}
	m := measured.Tenancy.CompileDB
	if recorded.Tenancy != nil && recorded.Tenancy.CompileDB != nil {
		rec := recorded.Tenancy.CompileDB
		if rec.Fingerprint == m.Fingerprint && rec.Databases == m.Databases {
			return nil
		}
		short := func(fp string) string {
			if fp == "" {
				return "none"
			}
			return fp[:min(12, len(fp))]
		}
		return []string{fmt.Sprintf("compile_db: recorded %s (%d database(s)) -> measured %s (%d database(s))", short(rec.Fingerprint), rec.Databases, short(m.Fingerprint), m.Databases)}
	}
	re := regexp.MustCompile("^" + regexp.QuoteMeta(recorded.Name) + `: include-path resolution read (\d+) compile database\(s\)`)
	for _, note := range notes {
		if g := re.FindStringSubmatch(note); g != nil {
			n, err := strconv.Atoi(g[1])
			if err == nil && n != m.Databases {
				return []string{fmt.Sprintf("compile_db: reading's note records %d database(s), measured %d", n, m.Databases)}
			}
			return nil
		}
	}
	return nil
}

// retimeInstallCompileDB writes the companion database src, with its relative
// directories made absolute against root, to root/build/compile_commands.json.
// A different database already there is a refusal.
func retimeInstallCompileDB(src, root string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	var entries []map[string]any
	if err := json.Unmarshal(raw, &entries); err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	for _, en := range entries {
		dir, _ := en["directory"].(string)
		if !filepath.IsAbs(dir) {
			en["directory"] = filepath.Join(abs, dir)
		}
	}
	out, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	dst := filepath.Join(abs, "build", "compile_commands.json")
	if have, err := os.ReadFile(dst); err == nil {
		if bytes.Equal(have, out) {
			return nil
		}
		return fmt.Errorf("a different compile database already exists at %s", dst)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, out, 0o644)
}

// retimeMachineClass never includes machine identity or raw command output.
func retimeMachineClass(ctx context.Context) string {
	family := "CPU family unavailable"
	if raw, err := exec.CommandContext(ctx, "sysctl", "-n", "machdep.cpu.brand_string").Output(); err == nil {
		if apple := regexp.MustCompile(`^Apple (M[0-9]+|A[0-9]+)\b`).FindString(strings.TrimSpace(string(raw))); apple != "" {
			family = apple
		}
	}
	return runtime.GOOS + "/" + runtime.GOARCH + " " + family
}
