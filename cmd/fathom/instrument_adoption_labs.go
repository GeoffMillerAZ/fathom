package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/labs"
)

// labsCardsRelPath and labsLedgerRelPath are the committed lab files the
// adoption report reads (docs/design/feature-labs.md section 10).
const (
	labsCardsRelPath  = "calibration/labs.d"
	labsLedgerRelPath = "docs/bench/labs-ledger.jsonl"
)

// maxLabsTextIDs caps each observation id list in the text report; --json
// carries every id.
const maxLabsTextIDs = 20

// AdoptionLabs is the labs section of the adoption report (LAB-6). It reads
// files only. Each *Error field names a read that failed (I38): the values
// beside it are what could be read, never a silent zero.
type AdoptionLabs struct {
	CardsDir string            `json:"cards_dir"`
	Cards    []AdoptionLabCard `json:"cards"`
	// CardError is LoadLabs' error; when set, Cards is empty and the closure
	// was not checked.
	CardError string `json:"card_error"`
	// ClosureGap is CheckLabClosure's error text, verbatim.
	ClosureGap    string              `json:"closure_gap"`
	LedgerPath    string              `json:"ledger_path"`
	NewestResults []labs.LabResultRow `json:"newest_results"`
	LedgerError   string              `json:"ledger_error"`
	Cases         []labs.LabCaseCount `json:"cases"`
	CaseError     string              `json:"case_error"`
	// Fixed lists entries whose last Status line starts with "fixed".
	Fixed AdoptionFixedObservations `json:"fixed_observations"`
	// Unlinked lists entries with no Lab link, counted by status word.
	Unlinked         AdoptionUnlinkedObservations `json:"unlinked_observations"`
	ObservationError string                       `json:"observation_error"`
}

// AdoptionLabCard is one loaded card.
type AdoptionLabCard struct {
	ID       string `json:"id"`
	Surfaces int    `json:"surfaces"`
}

// AdoptionFixedObservations carries every fixed id in numeric order and the
// fix-proof stage, which stays not measured until LAB-24 pins a format.
type AdoptionFixedObservations struct {
	Count    int      `json:"count"`
	IDs      []string `json:"ids"`
	FixProof string   `json:"fix_proof"`
}

// AdoptionUnlinkedObservations carries every unlinked id in numeric order.
// ByStatus buckets each by its last Status line's prefix, as the open and
// fixed counts read it: "open", "fixed", or "other" (no Status line too).
type AdoptionUnlinkedObservations struct {
	Count    int            `json:"count"`
	IDs      []string       `json:"ids"`
	ByStatus map[string]int `json:"by_status"`
}

// adoptionLabsSection builds the labs section over root. shipped and
// shippedErr are shippedLabSurfaces' result, passed in so a test can name
// its own inventory.
func adoptionLabsSection(root string, shipped []string, shippedErr error) AdoptionLabs {
	out := AdoptionLabs{
		CardsDir: labsCardsRelPath, Cards: []AdoptionLabCard{},
		LedgerPath: labsLedgerRelPath, NewestResults: []labs.LabResultRow{}, Cases: []labs.LabCaseCount{},
		Fixed:    AdoptionFixedObservations{IDs: []string{}, FixProof: labs.FixProofNotMeasured},
		Unlinked: AdoptionUnlinkedObservations{IDs: []string{}, ByStatus: map[string]int{}},
	}
	dir := filepath.Join(root, filepath.FromSlash(labsCardsRelPath))
	switch _, err := os.Stat(dir); {
	case os.IsNotExist(err):
	case err != nil:
		out.CardError = err.Error()
	case shippedErr != nil:
		out.CardError = "the shipped surface inventory: " + shippedErr.Error()
	default:
		cards, err := labs.LoadLabs(dir, shipped)
		if err != nil {
			out.CardError = err.Error()
			break
		}
		for _, c := range cards {
			out.Cards = append(out.Cards, AdoptionLabCard{ID: c.ID, Surfaces: len(c.Surface)})
		}
		if err := labs.CheckLabClosure(dir, shipped); err != nil {
			out.ClosureGap = err.Error()
		}
	}

	rows, err := labs.NewestLabResults(filepath.Join(root, filepath.FromSlash(labsLedgerRelPath)))
	if rows != nil {
		out.NewestResults = rows
	}
	if err != nil {
		out.LedgerError = err.Error()
	}
	cases, err := labs.CountLabCases(dir)
	if cases != nil {
		out.Cases = cases
	}
	if err != nil {
		out.CaseError = err.Error()
	}

	var problems []string
	statuses, err := observationStatuses(root)
	if err != nil {
		problems = append(problems, err.Error())
	}
	var fixed []int
	for id, status := range statuses {
		if strings.HasPrefix(strings.ToLower(status), "fixed") {
			fixed = append(fixed, id)
		}
	}
	out.Fixed.IDs = observationIDs(fixed)
	out.Fixed.Count = len(fixed)

	links, err := labs.ParseObservationLabLinks(root)
	if err != nil {
		problems = append(problems, err.Error())
	}
	// Links come legacy log first, then per-entry files: the last link for
	// an id is its per-entry file's, which wins.
	linked := map[int]bool{}
	for _, l := range links {
		if n, err := strconv.Atoi(strings.TrimPrefix(l.Observation, "O")); err == nil {
			linked[n] = l.Lab != ""
		}
	}
	var unlinked []int
	for id, isLinked := range linked {
		if isLinked {
			continue
		}
		unlinked = append(unlinked, id)
		status := strings.ToLower(statuses[id])
		word := "other"
		switch {
		case strings.HasPrefix(status, "open"):
			word = "open"
		case strings.HasPrefix(status, "fixed"):
			word = "fixed"
		}
		out.Unlinked.ByStatus[word]++
	}
	out.Unlinked.IDs = observationIDs(unlinked)
	out.Unlinked.Count = len(unlinked)
	out.ObservationError = strings.Join(problems, "; ")
	return out
}

// observationIDs sorts ids numerically and spells them O<n>.
func observationIDs(ids []int) []string {
	sort.Ints(ids)
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, "O"+strconv.Itoa(id))
	}
	return out
}

// observationStatuses maps each entry's numeric id to its last Status line,
// read from the legacy log and then the per-entry files, so a per-entry
// file's status wins on a duplicate id. Each entry's status comes from its
// own section, as countOpenObservations reads it. An entry with no Status
// line is absent. Missing files are empty; read failures are named.
func observationStatuses(root string) (map[int]string, error) {
	statuses := map[int]string{}
	scan := func(content string) {
		heads := observationEntryHeader.FindAllStringSubmatchIndex(content, -1)
		for i, h := range heads {
			end := len(content)
			if i+1 < len(heads) {
				end = heads[i+1][0]
			}
			id, err := strconv.Atoi(content[h[2]:h[3]])
			if err != nil {
				continue
			}
			matches := observationStatusLine.FindAllStringSubmatch(content[h[0]:end], -1)
			if len(matches) == 0 {
				continue
			}
			statuses[id] = strings.TrimSpace(matches[len(matches)-1][1])
		}
	}
	data, err := os.ReadFile(observationsPath(root))
	switch {
	case err == nil:
		scan(string(data))
	case !os.IsNotExist(err):
		return statuses, err
	}
	dir := observationsDir(root)
	des, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return statuses, nil
	}
	if err != nil {
		return statuses, err
	}
	type entry struct {
		id   int
		name string
	}
	var entries []entry
	for _, de := range des {
		m := observationEntryFile.FindStringSubmatch(de.Name())
		if m == nil || de.IsDir() {
			continue
		}
		if n, err := strconv.Atoi(m[1]); err == nil {
			entries = append(entries, entry{n, de.Name()})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].id != entries[j].id {
			return entries[i].id < entries[j].id
		}
		return entries[i].name < entries[j].name
	})
	var problems []string
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.name))
		if err != nil {
			problems = append(problems, fmt.Sprintf("observation entry %s unreadable: %v", filepath.ToSlash(filepath.Join(observationsDirRelPath, e.name)), err))
			continue
		}
		scan(string(data))
	}
	if len(problems) > 0 {
		return statuses, fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	return statuses, nil
}

// renderAdoptionLabs writes the labs section of the text report.
func renderAdoptionLabs(w io.Writer, l *AdoptionLabs) {
	fmt.Fprintf(w, "\nlabs (%s):\n", l.CardsDir)
	switch {
	case l.CardError != "":
		fmt.Fprintf(w, "  cards: CARDS UNREADABLE: %s\n  closure: not checked (the cards did not load)\n", l.CardError)
	case len(l.Cards) == 0:
		fmt.Fprintf(w, "  cards: 0 (no %s)\n", l.CardsDir)
	default:
		fmt.Fprintf(w, "  cards: %d\n", len(l.Cards))
		for _, c := range l.Cards {
			fmt.Fprintf(w, "    %s  surfaces=%d\n", c.ID, c.Surfaces)
		}
		if l.ClosureGap != "" {
			fmt.Fprintf(w, "  closure GAP: %s\n", l.ClosureGap)
		} else {
			fmt.Fprintln(w, "  closure: every shipped surface has a card or a no-lab entry")
		}
	}

	fmt.Fprintf(w, "  newest ledger row per lab (%s): %d\n", l.LedgerPath, len(l.NewestResults))
	for _, r := range l.NewestResults {
		fmt.Fprintf(w, "    %s  run=%s task=%s arm=%s verdict=%s\n", r.Lab, r.RunID, r.TaskID, r.Arm, r.Verdict)
	}
	if l.LedgerError != "" {
		fmt.Fprintf(w, "    ⚠ LEDGER: %s\n", l.LedgerError)
	}

	fmt.Fprintf(w, "  cases by split: %d lab(s) with cases\n", len(l.Cases))
	for _, c := range l.Cases {
		fmt.Fprintf(w, "    %s  train=%d select=%d confirm=%d\n", c.Lab, c.Train, c.Select, c.Confirm)
	}
	if l.CaseError != "" {
		fmt.Fprintf(w, "    ⚠ CASES: %s\n", l.CaseError)
	}

	fmt.Fprintf(w, "  fixed observations: %d (fix proof: %s)\n", l.Fixed.Count, l.Fixed.FixProof)
	writeLabsIDs(w, l.Fixed.IDs)
	words := make([]string, 0, len(l.Unlinked.ByStatus))
	for word, n := range l.Unlinked.ByStatus {
		words = append(words, fmt.Sprintf("%s=%d", word, n))
	}
	sort.Strings(words)
	fmt.Fprintf(w, "  observations without a Lab link: %d", l.Unlinked.Count)
	if len(words) > 0 {
		fmt.Fprintf(w, " (%s)", strings.Join(words, ", "))
	}
	fmt.Fprintln(w)
	writeLabsIDs(w, l.Unlinked.IDs)
	if l.ObservationError != "" {
		fmt.Fprintf(w, "    ⚠ OBSERVATIONS: %s\n", l.ObservationError)
	}
}

// writeLabsIDs prints the first maxLabsTextIDs ids and how many more.
func writeLabsIDs(w io.Writer, ids []string) {
	if len(ids) == 0 {
		return
	}
	shown, more := ids, 0
	if len(shown) > maxLabsTextIDs {
		shown, more = ids[:maxLabsTextIDs], len(ids)-maxLabsTextIDs
	}
	fmt.Fprintf(w, "    %s", strings.Join(shown, ", "))
	if more > 0 {
		fmt.Fprintf(w, " +%d more", more)
	}
	fmt.Fprintln(w)
}
