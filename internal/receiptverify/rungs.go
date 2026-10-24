package receiptverify

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/contextbench/publicloc"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/receipt"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// VerifyExported checks the recorded git evidence before re-deriving the
// envelope and each fact with the existing offline Go oracle. A missing
// rung is reported explicitly; it is never represented as confirmation.
// Legacy receipts without facts use Verify unchanged.
func VerifyExported(ctx context.Context, root string, r receipt.ExportedReceipt) (Outcome, error) {
	if len(r.Facts) == 0 {
		return Verify(ctx, root, r.Receipt)
	}
	facts := orderedFacts(r.Facts)
	out, filtered, err := verifyAnchors(ctx, root, facts)
	if err != nil {
		return out, err
	}
	if filtered > 0 {
		return withUnchecked(out, 0, 0, filtered), nil
	}
	// Uncommitted bytes skip only their own toolchain rung. The callers
	// envelope still has to reproduce against IndexCommit.
	eligible := 0
	for _, f := range facts {
		if toolchainSkip(f) == "" {
			eligible++
		}
	}
	// The exporter anchors at IndexCommit. Evidence from another commit is
	// a difference, even when that evidence reproduces at its own commit.
	for _, f := range facts {
		for _, a := range f.Anchors {
			if a.Commit != r.IndexCommit && (a.Commit != "" || a.State != receipt.StateUncommitted) {
				out.OK = false
				out.Diff = factLabel(f) + ": anchor commit differs from index_commit"
				out.Rungs = append(out.Rungs, out.Diff)
				return withUnchecked(out, len(facts)-eligible, 0, 0), nil
			}
		}
	}
	if eligible == 0 && r.Kind != core.ReceiptKindCallers {
		for _, f := range facts {
			out.Rungs = append(out.Rungs, factLabel(f)+": "+toolchainSkip(f))
		}
		return withUnchecked(out, len(facts), 0, 0), nil
	}
	dir, cleanup, err := CheckoutModule(ctx, root, r.IndexCommit)
	if err != nil {
		return out, err
	}
	defer cleanup()
	var truth publicloc.GoTruth
	var envelope Outcome
	switch r.Kind {
	case core.ReceiptKindCallers, core.ReceiptKindReferences:
		truth, err = LoadTruth(ctx, dir, r.IndexCommit)
		if err == nil && (r.Kind == core.ReceiptKindCallers || eligible == len(facts)) {
			if r.Kind == core.ReceiptKindCallers {
				envelope, err = verifyCallers(dir, r.Receipt, truth)
			} else {
				envelope = verifyReferences(r.Receipt, truth)
			}
		}
	case core.ReceiptKindSymbolLocation:
		if eligible == len(facts) {
			envelope, err = verifySymbolLocation(dir, r.Receipt)
		}
	default:
		err = fmt.Errorf("receipt: unknown kind %q", r.Kind)
	}
	if err != nil {
		for _, f := range facts {
			status := toolchainSkip(f)
			if status == "" {
				status = "toolchain rung: unknown"
			}
			out.Rungs = append(out.Rungs, factLabel(f)+": "+status)
		}
		return out, err
	}
	if (r.Kind == core.ReceiptKindCallers || eligible == len(facts)) && !envelope.OK {
		out.OK = false
		out.Diff = envelope.Diff
	}
	noToolchain := 0
	var callerSubjects []string
	tree, _ := gitro.Output(ctx, root, "rev-parse", "--verify", "--quiet", "--end-of-options", r.IndexCommit+"^{tree}")
	for _, f := range facts {
		if f.Kind == receipt.FactCaller {
			callerSubjects = append(callerSubjects, f.Subject)
		}
		if f.Kind == receipt.FactAbsence && f.ReadSet != nil && f.ReadSet.TreeHash != "" && f.ReadSet.TreeHash != strings.TrimSpace(string(tree)) {
			out.OK, out.Diff = false, factLabel(f)+": read set names another tree"
			out.Rungs = append(out.Rungs, out.Diff)
			continue
		}
		if status := toolchainSkip(f); status != "" {
			out.Rungs = append(out.Rungs, factLabel(f)+": "+status)
			continue
		}
		result, available, err := verifyFact(dir, r.Receipt, f, truth)
		if err != nil {
			out.Rungs = append(out.Rungs, factLabel(f)+": toolchain rung: unknown")
			return out, err
		}
		label := factLabel(f) + ": "
		if !available {
			out.Rungs = append(out.Rungs, label+"no toolchain rung for this fact kind")
			noToolchain++
			continue
		}
		status := "confirmed"
		if !result.OK {
			status = "refuted"
			out.OK = false
			if out.Diff == "" {
				out.Diff = result.Diff
			}
		}
		line := label + "toolchain rung: " + status
		if result.Diff != "" {
			line += " (" + result.Diff + ")"
		}
		out.Rungs = append(out.Rungs, line)
	}
	if r.Kind == core.ReceiptKindCallers && !equalSlices(sortedCopy(callerSubjects), sortedCopy(r.Callers)) {
		out.OK = false
		out.Rungs = append(out.Rungs, "caller fact subjects differ from envelope callers")
		if out.Diff == "" {
			out.Diff = "caller fact subjects differ from envelope callers"
		}
	}
	out = withUnchecked(out, len(facts)-eligible, noToolchain, 0)
	if !out.OK && out.Diff == "" {
		out.Diff = "git evidence differs from the receipt"
	}
	return out, nil
}

func withUnchecked(out Outcome, uncommitted, noToolchain, filtered int) Outcome {
	var counts []string
	for _, c := range []struct {
		n      int
		reason string
	}{{uncommitted, "uncommitted"}, {noToolchain, "no toolchain check"}, {filtered, "git filters unavailable"}} {
		if c.n > 0 {
			counts = append(counts, fmt.Sprintf("%d unchecked: %s", c.n, c.reason))
		}
	}
	if len(counts) > 0 {
		out.OK = false
		summary := "not reproduced (" + strings.Join(counts, ", ") + ")"
		if out.Diff != "" {
			summary += ": " + out.Diff
		}
		out.Diff = summary
	}
	return out
}

func toolchainSkip(f receipt.Fact) string {
	for _, a := range f.Anchors {
		if a.State == receipt.StateUncommitted {
			return "no toolchain rung (uncommitted)"
		}
	}
	return ""
}

func factLabel(f receipt.Fact) string { return string(f.Kind) + " " + f.Subject }

func orderedFacts(in []receipt.Fact) []receipt.Fact {
	facts := append([]receipt.Fact(nil), in...)
	sort.SliceStable(facts, func(i, j int) bool {
		if facts[i].Kind != facts[j].Kind {
			return facts[i].Kind < facts[j].Kind
		}
		return facts[i].Subject < facts[j].Subject
	})
	return facts
}

// VerifyGitAnchors checks exported facts without running a language oracle.
// The certification command uses it before its existing reach/run verifier.
func VerifyGitAnchors(ctx context.Context, root string, facts []receipt.Fact) (Outcome, error) {
	out, filtered, err := verifyAnchors(ctx, root, orderedFacts(facts))
	return withUnchecked(out, 0, 0, filtered), err
}

// verifyAnchors checks committed blobs and filtered checkout bytes at each
// anchor's own commit. It counts facts whose filters could not run.
func verifyAnchors(ctx context.Context, root string, facts []receipt.Fact) (Outcome, int, error) {
	out := Outcome{OK: true}
	unchecked := 0
	seen := map[struct {
		kind    receipt.FactKind
		subject string
	}]bool{}
	for _, f := range facts {
		switch f.Kind {
		case receipt.FactDefinition, receipt.FactCaller, receipt.FactAbsence:
		default:
			out.OK = false
			out.Diff = fmt.Sprintf("fact kind %q is not one an exported receipt carries", f.Kind)
			out.Rungs = append(out.Rungs, factLabel(f)+": refuted ("+out.Diff+")")
			return out, unchecked, nil
		}
		key := struct {
			kind    receipt.FactKind
			subject string
		}{f.Kind, f.Subject}
		if seen[key] {
			out.OK = false
			if out.Diff == "" {
				out.Diff = "duplicated fact: " + factLabel(f)
			}
			out.Rungs = append(out.Rungs, factLabel(f)+": refuted (duplicated fact)")
			continue
		}
		seen[key] = true
		filteredUnavailable := false
		anchors := append([]receipt.ExportAnchor(nil), f.Anchors...)
		sort.SliceStable(anchors, func(i, j int) bool {
			a, b := anchors[i], anchors[j]
			if a.Path != b.Path {
				return a.Path < b.Path
			}
			if a.StartLine != b.StartLine {
				return a.StartLine < b.StartLine
			}
			return a.EndLine < b.EndLine
		})
		if len(anchors) == 0 {
			out.Rungs = append(out.Rungs, factLabel(f)+": no git rung (no anchors)")
		}
		for _, a := range anchors {
			label := factLabel(f) + ": "
			switch a.State {
			case receipt.StateWithheld:
				if a.BlobID != "" || a.ExcerptSHA256 != "" || a.Excerpt != "" || a.FathomFingerprint != "" {
					out.OK = false
					if out.Diff == "" {
						out.Diff = factLabel(f) + ": withheld anchor carries blob id, digest, excerpt or fingerprint"
					}
					out.Rungs = append(out.Rungs, label+"git rung: refuted (withheld anchor carries evidence)")
					continue
				}
				out.Rungs = append(out.Rungs, label+"no git rung (withheld)")
				continue
			case receipt.StateUncommitted:
				out.Rungs = append(out.Rungs, label+"no git rung ("+string(a.State)+")")
				continue
			case receipt.StateCommitted, receipt.StateFiltered:
			default:
				out.Rungs = append(out.Rungs, label+"no git rung (unknown anchor state)")
				return out, unchecked, fmt.Errorf("receipt: unknown anchor state %q", a.State)
			}
			difference, err := verifyAnchor(ctx, root, a)
			if err != nil {
				out.Rungs = append(out.Rungs, label+"git rung: unknown")
				if a.State == receipt.StateFiltered {
					filteredUnavailable = true
					continue
				}
				return out, unchecked, err
			}
			status := "confirmed"
			if difference != "" {
				status = "refuted"
				out.OK = false
				if out.Diff == "" {
					out.Diff = factLabel(f) + ": " + difference
				}
			}
			line := label + "git rung: " + status + fmt.Sprintf(" (%s:%d-%d", a.Path, a.StartLine, a.EndLine)
			if difference != "" {
				line += ", " + difference
			}
			line += ")"
			out.Rungs = append(out.Rungs, line)
		}
		if filteredUnavailable {
			unchecked++
		}
	}
	return out, unchecked, nil
}

func verifyAnchor(ctx context.Context, root string, a receipt.ExportAnchor) (string, error) {
	if !validAnchorPath(a.Path) {
		return "invalid repo-relative anchor path", nil
	}
	if a.Commit == "" {
		return "anchor has no recorded commit", nil
	}
	c, err := receipt.ResolveCommit(ctx, root, a.Commit)
	if err != nil {
		return "", err
	}
	if a.ObjectFormat != c.ObjectFormat {
		return "object format differs", nil
	}
	id, err := gitro.Output(ctx, root, "rev-parse", "--verify", "--quiet", "--end-of-options", c.ID+":"+a.Path)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "recorded path is not present at commit", nil
	}
	blobID := strings.TrimSpace(string(id))
	if a.BlobID != blobID {
		return "blob id differs", nil
	}
	var blob []byte
	if a.State == receipt.StateFiltered {
		cmd, cleanup, commandErr := gitro.Command(ctx, root, "cat-file", "--filters", c.ID+":"+a.Path)
		defer cleanup()
		if commandErr != nil {
			return "", commandErr
		}
		cmd.Env = append(cmd.Env, "GIT_LFS_SKIP_SMUDGE=1")
		blob, err = cmd.Output()
	} else {
		blob, err = gitro.Output(ctx, root, "cat-file", "blob", blobID)
	}
	if err != nil {
		return "", fmt.Errorf("receipt: cannot read recorded blob for %s: %w", a.Path, err)
	}
	digest, ok := verify.ExcerptSHA256(blob, a.StartLine, a.EndLine)
	if !ok {
		return "recorded line range is absent", nil
	}
	if a.ExcerptSHA256 == "" || a.ExcerptSHA256 != digest {
		return "excerpt digest differs", nil
	}
	if a.FathomFingerprint != "" && a.FathomFingerprint != fusion.FingerprintHash(blob) {
		return "fathom fingerprint differs", nil
	}
	if a.Excerpt != "" {
		end := a.EndLine
		if a.ExcerptEndLine != 0 {
			if a.ExcerptEndLine < a.StartLine || (end != 0 && a.ExcerptEndLine > end) {
				return "invalid excerpt end line", nil
			}
			end = a.ExcerptEndLine
		}
		start := a.StartLine
		if start == 0 && a.ExcerptEndLine != 0 {
			start = 1
		}
		excerpt, ok := verify.ExcerptBytes(blob, start, end)
		if !ok || string(excerpt) != a.Excerpt {
			return "excerpt bytes differ", nil
		}
	}
	return "", nil
}

func stripService(s string) string {
	if i := strings.LastIndex(s, "::"); i >= 0 {
		return s[i+2:]
	}
	return s
}

func validAnchorPath(p string) bool {
	return p != "" && path.Clean(p) == p && p != "." && p != ".." && !strings.HasPrefix(p, "../") && !strings.HasPrefix(p, "/") && !strings.Contains(p, "\\")
}

func fileQualifiedName(dir, file, name string) (string, error) {
	if !validAnchorPath(file) {
		return "", fmt.Errorf("receipt: invalid repo-relative path %q", file)
	}
	f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, filepath.FromSlash(file)), nil, parser.PackageClauseOnly)
	if err != nil {
		return "", fmt.Errorf("receipt: read package in %s: %w", file, err)
	}
	pkg := f.Name.Name
	if i := strings.Index(name, "."); i >= 0 {
		// Go method nodes use the base receiver type as Package, rather
		// than the file's package. The oracle already strips pointer and
		// generic receiver syntax from its Recv.Method names.
		pkg = name[:i]
	}
	return core.QualifiedPackage(file, pkg) + "." + name, nil
}

func verifyFact(dir string, r core.Receipt, f receipt.Fact, truth publicloc.GoTruth) (Outcome, bool, error) {
	switch f.Kind {
	case receipt.FactDefinition, receipt.FactCaller:
		if len(f.Anchors) == 0 {
			return Outcome{Diff: "fact has no definition anchor"}, true, nil
		}
		for _, a := range f.Anchors {
			if filepath.Ext(a.Path) != ".go" {
				return Outcome{}, false, nil
			}
			subject := BareSymbolName(f.Subject)
			qualified, err := fileQualifiedName(dir, a.Path, subject)
			if err != nil {
				return Outcome{}, true, err
			}
			if stripService(f.Subject) != subject && stripService(f.Subject) != qualified {
				return Outcome{Diff: "fact subject names a different package"}, true, nil
			}
			line := a.StartLine
			if line == 0 {
				// Whole-file anchors still require a declaration, but pin no line.
				line, err = declarationLine(dir, a.Path, subject)
				if err != nil {
					return Outcome{}, true, err
				}
			}
			located, err := verifySymbolLocation(dir, core.Receipt{Symbol: f.Subject, File: a.Path, Line: line})
			if err != nil || !located.OK {
				return located, true, err
			}
			if f.Kind == receipt.FactCaller {
				found := false
				for _, e := range truth.Edges {
					if e.CallerFile == a.Path && e.Caller == subject && e.Callee == BareSymbolName(r.Symbol) {
						found = true
						break
					}
				}
				if !found {
					return Outcome{Diff: f.Subject + " does not call " + r.Symbol}, true, nil
				}
			}
		}
		return Outcome{OK: true}, true, nil
	case receipt.FactAbsence:
		if f.ReadSet == nil || len(f.ReadSet.Files) == 0 || f.ReadSet.TreeHash == "" || f.ReadSet.Tool == "" {
			return Outcome{Diff: "absence fact has no read set"}, true, nil
		}
		count := map[bool]string{false: "0 ", true: "≥ 0 "}[len(f.MissingReads) > 0]
		if len(f.Anchors) > 0 || f.LowerBound != (len(f.MissingReads) > 0) || f.Count != "" && !strings.HasPrefix(f.Count, count) {
			return Outcome{Diff: "absence fact carries anchors or a count other than zero"}, true, nil
		}
		if r.Kind != core.ReceiptKindCallers {
			return Outcome{}, false, nil
		}
		if q := stripService(f.Subject); q != BareSymbolName(f.Subject) && !declaredQualified(dir, truth, q) {
			return Outcome{Diff: f.Subject + " is not declared at this commit"}, true, nil
		}
		check := r
		check.Symbol = f.Subject
		check.Callers = nil
		result, err := verifyCallers(dir, check, truth)
		return result, true, err
	default:
		return Outcome{}, false, nil
	}
}

// Old saved receipts carry bare Go names; exported receipts carry package
// qualifiers. Keep the former's name-level comparison for compatibility.
func qualifiedCallerReceipt(r core.Receipt, truth publicloc.GoTruth) bool {
	names := declaredNames(truth)
	for _, s := range r.Callers {
		if strings.Contains(s, "::") || strings.Contains(s, "/") || strings.Contains(s, ".") && !names[s] {
			return true
		}
	}
	return false
}

func qualifiedCallers(dir string, r core.Receipt, truth publicloc.GoTruth) ([]string, []string, error) {
	wantSet, gotSet := map[string]bool{}, map[string]bool{}
	for _, s := range r.Callers {
		wantSet[stripService(s)] = true
	}
	for _, e := range truth.Edges {
		if e.Callee != BareSymbolName(r.Symbol) || !validAnchorPath(e.CallerFile) {
			continue
		}
		name, err := fileQualifiedName(dir, e.CallerFile, e.Caller)
		if err != nil {
			return nil, nil, err
		}
		gotSet[name] = true
	}
	keys := func(m map[string]bool) []string {
		out := make([]string, 0, len(m))
		for k := range m {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	return keys(wantSet), keys(gotSet), nil
}

func declarationLine(dir, file, name string) (int, error) {
	full := filepath.Join(dir, filepath.FromSlash(file))
	src, err := os.ReadFile(full)
	if err != nil {
		return 0, err
	}
	spans, err := publicloc.FuncSpans(src, full)
	if err != nil {
		return 0, err
	}
	for _, s := range spans {
		if s.Name == name {
			return s.Start, nil
		}
	}
	return -1, nil
}

func declaredQualified(dir string, truth publicloc.GoTruth, q string) bool {
	for file, names := range truth.DeclsByFile {
		for _, n := range names {
			if got, err := fileQualifiedName(dir, file, n); err == nil && got == q {
				return true
			}
		}
	}
	return false
}
