// Command harnessctl checks protected signed configuration. Each invocation has
// independent state; concurrent invocations are safe when their readers are safe.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"sort"
	"time"
)

type report struct {
	Version             string             `json:"version"`
	Status              string             `json:"status"`
	Category            string             `json:"category,omitempty"`
	Code                string             `json:"code,omitempty"`
	ExpectedLayers      *int               `json:"expectedLayers,omitempty"`
	LoadedLayers        *int               `json:"loadedLayers,omitempty"`
	AuthenticatedLayers *int               `json:"authenticatedLayers,omitempty"`
	SourceOrdinal       *int               `json:"sourceOrdinal,omitempty"`
	NoActionAuthority   bool               `json:"noActionAuthority"`
	Coverage            string             `json:"coverage"`
	Installation        string             `json:"installation"`
	ProjectID           string             `json:"projectId,omitempty"`
	RegistrationEpoch   int                `json:"registrationEpoch,omitempty"`
	RegistrationSHA256  string             `json:"registrationSHA256,omitempty"`
	SnapshotSHA256      string             `json:"snapshotSHA256,omitempty"`
	IssuedAt            string             `json:"issuedAt,omitempty"`
	ExpiresAt           string             `json:"expiresAt,omitempty"`
	LayerCount          int                `json:"layerCount,omitempty"`
	ControlCount        int                `json:"controlCount,omitempty"`
	Sources             []sourceReport     `json:"sources,omitempty"`
	EffectiveControls   []effectiveControl `json:"effectiveControls,omitempty"`
}
type sourceReport struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Priority int    `json:"priority"`
	Revision int    `json:"revision"`
}
type seams struct {
	now  func() time.Time
	read func(string, int) (snapshot, error)
}

func defaults() seams               { return seams{now: time.Now, read: readSnapshot} }
func failure(e error) (report, int) { return failureAt(e, 0, 0, 0, nil) }
func failureAt(e error, expected, loaded, authenticated int, ordinal *int) (report, int) {
	category := "source"
	name := "input_invalid"
	code := 2
	switch {
	case errors.Is(e, errRegistration):
		name = "registration_invalid"
	case errors.Is(e, errProject):
		name = "project_unregistered"
	case errors.Is(e, errClock):
		name = "clock_invalid"
	case errors.Is(e, errUnavailable):
		name = "source_unavailable"
	case errors.Is(e, errChanged):
		name = "source_changed"
	case errors.Is(e, errEnvelope):
		name = "envelope_invalid"
	case errors.Is(e, errSignature):
		name = "signature_invalid"
	case errors.Is(e, errBinding):
		name = "binding_mismatch"
	case errors.Is(e, errPreference):
		name = "preference_denied"
	case errors.Is(e, errPolicy):
		name = "policy_conflict"
	}
	if errors.Is(e, errPolicy) || errors.Is(e, errPreference) {
		category = "policy"
		code = 1
	}
	return report{Version: "harnessctl.report/1", Status: "hold", Category: category, Code: name, ExpectedLayers: &expected, LoadedLayers: &loaded, AuthenticatedLayers: &authenticated, SourceOrdinal: ordinal, NoActionAuthority: true, Coverage: "selected-project-only", Installation: "unobserved"}, code
}
func check(registrationPath, projectRoot string, explain bool, s seams) (report, int) {
	if s.now == nil || s.read == nil {
		return failure(errClock)
	}
	// Signed validity uses wall time. Strip native monotonic metadata so endpoint
	// comparison cannot hide an observed wall-clock rollback.
	start := s.now().Round(0)
	if start.IsZero() {
		return failure(errClock)
	}
	if !canonical(projectRoot) {
		return failure(errSource)
	}
	rootInfo, e := os.Stat(projectRoot)
	if e != nil || !rootInfo.IsDir() {
		return failure(errSource)
	}
	regSnap, e := s.read(registrationPath, 2<<20)
	if e != nil {
		return failure(errUnavailable)
	}
	var reg registration
	if strictJSON(regSnap.data, &reg) != nil {
		return failure(errRegistration)
	}
	p, keys, catalog, e := validateRegistration(reg, projectRoot, registrationPath, start)
	if e != nil {
		return failure(e)
	}
	selected := make([]selectedLayer, 0, len(p.Layers))
	snaps := make([]snapshot, 0, len(p.Layers))
	total := 0
	loaded := 0
	authenticated := 0
	expected := len(p.Layers)
	selectedIDs := map[string]bool{}
	for _, ref := range p.Layers {
		selectedIDs[ref.ID] = true
	}
	for i, ref := range p.Layers {
		snap, e := s.read(ref.Path, 64<<10)
		if e != nil {
			return failureAt(errUnavailable, expected, loaded, authenticated, &i)
		}
		loaded++
		total += len(snap.data)
		if total > 2<<20 {
			return failureAt(errEnvelope, expected, loaded, authenticated, &i)
		}
		l, _, e := verifyLayer(snap.data, ref, keys[ref.KeyID], start)
		if e != nil {
			return failureAt(e, expected, loaded, authenticated, &i)
		}
		authenticated++
		for _, c := range l.Controls {
			for _, tuner := range c.TunableBy {
				if !selectedIDs[tuner] {
					return failureAt(errBinding, expected, loaded, authenticated, &i)
				}
			}
		}
		selected = append(selected, selectedLayer{ref, l})
		snaps = append(snaps, snap)
	}
	effective, e := compose(p, catalog, selected)
	if e != nil {
		return failureAt(e, expected, loaded, authenticated, nil)
	}
	for i := range selected {
		if !sameSnapshot(s.read, snaps[i], 64<<10) {
			return failureAt(errChanged, expected, loaded, authenticated, &i)
		}
	}
	if !sameSnapshot(s.read, regSnap, 2<<20) {
		return failureAt(errChanged, expected, loaded, authenticated, nil)
	}
	finalRoot, e := os.Stat(projectRoot)
	if !canonical(projectRoot) || e != nil || !finalRoot.IsDir() || !os.SameFile(rootInfo, finalRoot) {
		return failureAt(errChanged, expected, loaded, authenticated, nil)
	}
	end := s.now().Round(0)
	if end.Before(start) || end.IsZero() {
		return failureAt(errClock, expected, loaded, authenticated, nil)
	}
	if _, _, e = interval(reg.IssuedAt, reg.ExpiresAt, end, 366*24*time.Hour); e != nil {
		return failureAt(e, expected, loaded, authenticated, nil)
	}
	for i, src := range selected {
		if _, _, e = interval(src.value.IssuedAt, src.value.ExpiresAt, end, 366*24*time.Hour); e != nil {
			return failureAt(e, expected, loaded, authenticated, &i)
		}
		issued, _ := parseTime(src.value.IssuedAt)
		if end.Sub(issued) >= time.Duration(src.ref.MaxAgeSeconds)*time.Second {
			return failureAt(errClock, expected, loaded, authenticated, &i)
		}
	}
	regHash := sha256.Sum256(regSnap.data)
	digest, e := snapshotDigest(regHash, p.ID, append([]layerRef(nil), p.Layers...))
	if e != nil {
		return failureAt(errRegistration, expected, loaded, authenticated, nil)
	}
	validFrom, _ := parseTime(reg.IssuedAt)
	validUntil, _ := parseTime(reg.ExpiresAt)
	for _, src := range selected {
		from, _ := parseTime(src.value.IssuedAt)
		until, _ := parseTime(src.value.ExpiresAt)
		ageUntil := from.Add(time.Duration(src.ref.MaxAgeSeconds) * time.Second)
		if from.After(validFrom) {
			validFrom = from
		}
		if until.Before(validUntil) {
			validUntil = until
		}
		if ageUntil.Before(validUntil) {
			validUntil = ageUntil
		}
	}
	r := report{Version: "harnessctl.report/1", Status: "authenticated", NoActionAuthority: true, Coverage: "selected-project-only", Installation: "unobserved", RegistrationEpoch: reg.Epoch, RegistrationSHA256: hex.EncodeToString(regHash[:]), SnapshotSHA256: digest, IssuedAt: validFrom.Format("2006-01-02T15:04:05Z"), ExpiresAt: validUntil.Format("2006-01-02T15:04:05Z"), LayerCount: len(p.Layers), ControlCount: len(effective)}
	if explain {
		r.ProjectID = p.ID
		r.EffectiveControls = effective
		for _, ref := range p.Layers {
			r.Sources = append(r.Sources, sourceReport{ref.ID, ref.Kind, ref.Priority, ref.Revision})
		}
		sort.Slice(r.Sources, func(i, j int) bool { return r.Sources[i].ID < r.Sources[j].ID })
	}
	return r, 0
}
func sameSnapshot(read func(string, int) (snapshot, error), old snapshot, limit int) bool {
	next, e := read(old.path, limit)
	return e == nil && os.SameFile(old.info, next.info) && string(old.data) == string(next.data)
}

func run(args []string, out io.Writer, s seams) int {
	if len(args) > 0 && args[0] == "maintenance" {
		return runMaintenance(args[1:], out)
	}
	if len(args) > 0 && args[0] == "hooks" {
		return runHooks(args[1:], out, os.Stderr)
	}
	if len(args) == 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") || len(args) == 2 && args[0] == "config" && (args[1] == "--help" || args[1] == "-h") {
		return writeUsage(out)
	}
	if len(args) < 2 || args[0] != "config" || (args[1] != "check" && args[1] != "explain") {
		r, code := failure(errSource)
		return writeReport(out, "json", r, code)
	}
	f := flag.NewFlagSet("config", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	var registrationPath, projectRoot, format string
	f.StringVar(&registrationPath, "registration", "", "")
	f.StringVar(&projectRoot, "project", "", "")
	f.StringVar(&format, "format", "json", "")
	parseErr := f.Parse(args[2:])
	if errors.Is(parseErr, flag.ErrHelp) {
		return writeUsage(out)
	}
	if parseErr != nil || len(f.Args()) != 0 || registrationPath == "" || projectRoot == "" || (format != "json" && format != "text") {
		r, code := failure(errSource)
		return writeReport(out, "json", r, code)
	}
	r, code := check(registrationPath, projectRoot, args[1] == "explain", s)
	return writeReport(out, format, r, code)
}
func writeUsage(out io.Writer) int {
	usage := "harnessctl config check --registration ABS --project ABS --format json|text\n" +
		"harnessctl config explain --registration ABS --project ABS --format json|text\n" +
		"Config subcommands verify signed configuration. They grant no hook or maintenance authority.\n" + maintenanceUsage() + hooksUsage()
	n, e := io.WriteString(out, usage)
	if e != nil || n != len(usage) {
		return 2
	}
	return 0
}
func writeReport(out io.Writer, format string, r report, code int) int {
	var b []byte
	var e error
	if format == "text" {
		b, e = json.MarshalIndent(r, "", "  ")
	} else {
		b, e = json.Marshal(r)
	}
	if e != nil || len(b)+1 > 1<<20 {
		return 2
	}
	b = append(b, '\n')
	n, e := out.Write(b)
	if e != nil || n != len(b) {
		return 2
	}
	return code
}
func main() { os.Exit(run(os.Args[1:], os.Stdout, defaults())) }
