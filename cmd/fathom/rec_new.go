package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/internal/recordids"
	"github.com/geoffmilleraz/fathom/internal/records"
)

func cmdRecNew(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("rec new", stderr)

	root := fs.String("root", ".", "repository root")
	slug := fs.String("lane", "", "trusted lane slug")
	family := fs.String("family", "", "record family")
	target := fs.String("path", "", "exact owned path")
	title := fs.String("title", "", "single-line record title")
	if e := fs.Parse(args); e != nil {
		return 2
	}
	if fs.NArg() != 0 || *slug == "" || *family == "" || *target == "" || !recTitleValid(*title) {
		fmt.Fprintln(stderr, "usage: rec new --root DIR --lane SLUG --family NAME --path REL --title TEXT")
		return 2
	}
	fail := func(e error) int { fmt.Fprintln(stderr, "error:", e); return 1 }
	if e := validateRecordTarget(*root, *target); e != nil {
		return fail(e)
	}
	revision, laneID := os.Getenv("FATHOM_LANE_TRUSTED_BASE"), os.Getenv("FATHOM_LANE_ID")
	if revision == "" || laneID == "" || laneID != *slug {
		return fail(recordids.Refuse(recordids.ErrTrust, "trusted base and matching lane environment are required"))
	}
	state, e := recordids.LoadTrusted(context.Background(), *root, revision)
	if e != nil {
		return fail(e)
	}
	if e := state.CheckManifests(*root); e != nil {
		return fail(e)
	}
	var lane records.Lane
	for _, l := range state.Lanes {
		if l.ID == *slug {
			lane = l
		}
	}
	if lane.ID == "" || !slices.Contains(lane.OwnedPaths, *target) {
		return fail(recordids.Refuse(recordids.ErrTarget, "target is outside trusted lane owned paths: "+*target))
	}
	if e := validateRecordTarget(*root, "workspace/rec-new.lock"); e != nil {
		return fail(e)
	}
	if e := os.MkdirAll(filepath.Join(*root, "workspace"), 0700); e != nil {
		return fail(e)
	}
	lock, e := os.OpenFile(filepath.Join(*root, "workspace/rec-new.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return fail(e)
	}
	defer lock.Close()
	if e := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); e != nil {
		return fail(e)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	// Recheck authority and target under the same lease as fresh inventory/write.
	if e := state.CheckManifests(*root); e != nil {
		return fail(e)
	}
	if e := validateRecordTarget(*root, *target); e != nil {
		return fail(e)
	}
	defs, e := recordids.Inventory(*root, state.Policy)
	if e != nil {
		return fail(e)
	}
	inventory, e := recordids.ValidateInventory(defs, state.Policy, state.Inventory)
	if e != nil {
		return fail(e)
	}
	if *family == string(recordids.FamilyPlan) && !state.PlanTargetAllowed(lane.ID, *target) {
		return fail(recordids.Refuse(recordids.ErrTarget, "plan prefix and trusted target disagree"))
	}

	id, e := records.AllocateRecordID(lane, *family, *target, inventory)
	if e != nil {
		return fail(e)
	}
	payload, e := recPayload(*family, id, *title)
	if e != nil {
		return fail(e)
	}
	path := filepath.Join(*root, *target)
	mode := os.FileMode(0644)
	old, e := os.ReadFile(path)
	if e != nil && !os.IsNotExist(e) {
		return fail(e)
	}
	if e == nil {
		info, e := os.Stat(path)
		if e != nil {
			return fail(e)
		}
		mode = info.Mode().Perm()
	}
	body := payload
	if len(old) > 0 {
		body = strings.TrimRight(string(old), "\n") + "\n\n" + payload
	}

	// A new heading must be discoverable under the effective trusted policy and
	// its complete appended context (including any existing Markdown fence).
	prospective, e := recordids.DefinitionsForFile(*target, []byte(body), state.Policy)
	if e != nil {
		return fail(e)
	}
	count := 0
	for _, d := range prospective {
		if d.Family == recordids.FamilyName(*family) && d.ID == id {
			count++
		}
	}
	if count != 1 {
		return fail(recordids.Refuse(recordids.ErrPolicy, "emitted record is not uniquely discoverable"))
	}
	complete := []recordids.Definition{}
	for _, d := range defs {
		if d.At.Path != *target {
			complete = append(complete, d)
		}
	}
	complete = append(complete, prospective...)
	if _, e := recordids.ValidateInventory(complete, state.Policy, state.Inventory); e != nil {
		return fail(e)
	}
	if e := validateRecordTarget(*root, *target); e != nil {
		return fail(e)
	}
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return fail(e)
	}
	if e := validateRecordTarget(*root, *target); e != nil {
		return fail(e)
	}
	tmp, e := os.CreateTemp(filepath.Dir(path), ".rec-new-*")
	if e != nil {
		return fail(e)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if e := tmp.Chmod(mode); e != nil {
		return fail(e)
	}
	if _, e := io.WriteString(tmp, body); e != nil {
		return fail(e)
	}
	if e := tmp.Sync(); e != nil {
		return fail(e)
	}
	if e := tmp.Close(); e != nil {
		return fail(e)
	}
	if e := validateRecordTarget(*root, *target); e != nil {
		return fail(e)
	}
	if e := os.Rename(tmp.Name(), path); e != nil {
		return fail(e)
	}
	fmt.Fprintf(stdout, "%s %s\n", id, *target)
	return 0
}
func recTitleValid(s string) bool {
	if strings.TrimSpace(s) == "" || !utf8.ValidString(s) || strings.ContainsAny(s, "{}|`") || strings.Contains(s, `\(`) {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return false
		}
	}
	return true
}
func recPayload(family, id, title string) (string, error) {
	switch recordids.FamilyName(family) {
	case recordids.FamilyADR:
		return "## " + id + ": " + title + "\n\n**Status.** Proposed\n", nil
	case recordids.FamilyInvariant:
		return "| " + id + " | " + title + " | stated | |\n", nil
	case recordids.FamilyFuture:
		return "## " + id + ". " + title + "\n", nil
	case recordids.FamilyIncident:
		return "## " + id + ": " + title + "\n", nil
	case recordids.FamilyObservation:
		return "### " + id + ": " + title + "\n", nil
	case recordids.FamilyPlan:
		return "- [ ] " + title + " {id: " + id + "}\n", nil
	}
	return "", recordids.Refuse(recordids.ErrShape, "unknown family")
}

// validateRecordTarget checks lexical and filesystem containment. Existing
// target/parent symlinks are refused even when their destination is inside root.
func validateRecordTarget(root, rel string) error {
	if !recordids.CanonicalPath(rel) {
		return recordids.Refuse(recordids.ErrTarget, "noncanonical record path: "+rel)
	}
	resolved, e := filepath.EvalSymlinks(root)
	if e != nil {
		return e
	}
	resolved, e = filepath.Abs(resolved)
	if e != nil {
		return e
	}
	info, e := os.Stat(resolved)
	if e != nil {
		return e
	}
	if !info.IsDir() {
		return recordids.Refuse(recordids.ErrTarget, "root is not a directory")
	}
	current := resolved
	parts := strings.Split(rel, "/")
	for i, p := range parts {
		current = filepath.Join(current, p)
		info, e := os.Lstat(current)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return e
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return recordids.Refuse(recordids.ErrTarget, "symlink in record path: "+rel)
		}
		if i < len(parts)-1 && !info.IsDir() {
			return recordids.Refuse(recordids.ErrTarget, "record parent is not a directory")
		}
		if i == len(parts)-1 && !info.Mode().IsRegular() {
			return recordids.Refuse(recordids.ErrTarget, "record target is not a regular file")
		}
	}
	return nil
}

type recRunner struct{}

func (recRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: rec new --root DIR --lane SLUG --family NAME --path REL --title TEXT")
		return 2
	}
	switch args[0] {
	case "new":
		return cmdRecNew(args[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, "error: rec requires new")
		return 2
	}
}
func init() {
	declareVerb(verb{Name: "rec", Group: "govern", Order: 170, Run: recRunner{}, Summary: "Create a record inside an integrator-granted lane range"})
}
