package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const journalVersion = "prose-lab.journal/1"
const maxJournalAttempts = 4999 // two files per attempt, header and lock fit 10000 entries

var attemptName = regexp.MustCompile(`^reserve-([0-9]{6})\.json$`)

type reservation struct {
	Version     string `json:"version"`
	Sequence    int    `json:"sequence"`
	CaseID      string `json:"case_id"`
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	RequestHash string `json:"request_digest"`
	RequestJSON string `json:"request_json"`
	MicroUSD    int64  `json:"reserved_micro_usd"`
	Pricing     lane   `json:"pricing"`
	At          int64  `json:"at_unix_nano"`
}

type outcome struct {
	Version        string `json:"version"`
	Sequence       int    `json:"sequence"`
	Reservation    string `json:"reservation_digest"`
	Status         string `json:"status"`
	Model          string `json:"resolved_model,omitempty"`
	UsagePresent   bool   `json:"usage_present"`
	ActualMicroUSD *int64 `json:"usage_priced_micro_usd,omitempty"`
	LatencyNanos   int64  `json:"latency_nanos"`
	HTTPStatus     int    `json:"http_status"`
	Response       string `json:"response_json,omitempty"`
	At             int64  `json:"at_unix_nano"`
}

type journalHeader struct {
	Version        string `json:"version"`
	StudyHash      string `json:"study_digest"`
	BudgetMicroUSD int64  `json:"budget_micro_usd"`
}

// journal retains reservations even when provider usage is missing. It never
// treats transport failure or an unfinished request as free. Values are a local
// accounting bound under the study's declared pricing, not a provider billing cap.
type journal struct {
	mu          sync.Mutex
	root        *os.Root
	header      journalHeader
	now         func() time.Time
	next        int
	charged     int64
	storedBytes int64
	pending     map[int]reservation
	closed      bool
}

func hashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func jsonBytes(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func openJournal(path string, study []byte, budget int64, now func() time.Time) (_ *journal, err error) {
	if path == "" || budget < 1 || budget > 10_000_000 || now == nil {
		return nil, errors.New("invalid journal configuration")
	}
	if err := os.Mkdir(path, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	st, err := os.Lstat(path)
	if err != nil || !st.IsDir() || st.Mode().Perm()&0077 != 0 {
		return nil, errors.New("journal requires a private non-symlink directory")
	}
	r, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			r.Close()
		}
	}()
	if err = r.Mkdir(".lock", 0700); err != nil {
		return nil, errors.New("journal is locked; inspect prior process and uncertain attempts before recovery")
	}
	defer func() {
		if err != nil {
			r.Remove(".lock")
		}
	}()
	h := journalHeader{Version: journalVersion, StudyHash: hashBytes(study), BudgetMicroUSD: budget}
	var stored journalHeader
	if err = readJournalJSON(r, "study.json", &stored); errors.Is(err, os.ErrNotExist) {
		err = writeExclusive(r, "study.json", h)
	} else if err == nil && stored != h {
		err = errors.New("journal study or budget binding changed")
	}
	if err != nil {
		return nil, err
	}
	j := &journal{root: r, header: h, now: now, pending: map[int]reservation{}, next: 1}
	if err = j.load(); err != nil {
		return nil, err
	}
	return j, nil
}

func (j *journal) load() error {
	_, err := j.readHistory(false)
	return err
}

// auditJournal never creates a journal or header. Its snapshot has no writer port.
func auditJournal(path string, study []byte, budget int64) (_ []journalAttempt, charged int64, err error) {
	return auditJournalAfterLock(path, study, budget, nil)
}

func auditJournalAfterLock(path string, study []byte, budget int64, afterLock func(*os.Root)) (_ []journalAttempt, charged int64, err error) {
	if path == "" || budget < 1 || budget > 10_000_000 {
		return nil, 0, errors.New("invalid audit configuration")
	}
	st, err := os.Lstat(path)
	if err != nil || !st.IsDir() || st.Mode().Perm()&0077 != 0 {
		return nil, 0, errors.New("private journal unavailable")
	}
	r, err := os.OpenRoot(path)
	if err != nil {
		return nil, 0, err
	}
	defer r.Close()
	if err = r.Mkdir(".lock", 0700); err != nil {
		return nil, 0, errors.New("journal is locked")
	}
	defer r.Remove(".lock")
	if afterLock != nil {
		afterLock(r)
	}
	h := journalHeader{Version: journalVersion, StudyHash: hashBytes(study), BudgetMicroUSD: budget}
	var stored journalHeader
	if err = readJournalJSON(r, "study.json", &stored); err != nil || stored != h {
		return nil, 0, errors.New("journal header unavailable or mismatched")
	}
	j := &journal{root: r, header: h, pending: map[int]reservation{}, next: 1}
	attempts, err := j.readHistory(true)
	if err != nil {
		return nil, 0, err
	}
	return attempts, j.charged, nil
}

type journalAttempt struct {
	Reservation reservation
	Outcome     *outcome
}

// readHistory is the sole history validator for writers and locked audits.
// Audit permits an intact final reservation without an outcome, but charges it.
func (j *journal) readHistory(audit bool) ([]journalAttempt, error) {
	f, err := j.root.Open(".")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries, err := f.ReadDir(10001)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if len(entries) > 10000 {
		return nil, errors.New("journal entry bound")
	}
	names := []string{}
	for _, e := range entries {
		n := e.Name()
		if n != ".lock" {
			st, err := j.root.Lstat(n)
			if err != nil || !st.Mode().IsRegular() || st.Size() > 4<<20 || st.Size() > 64<<20-j.storedBytes {
				return nil, errors.New("journal storage bound or unexpected entry")
			}
			j.storedBytes += st.Size()
		}
		if n == ".lock" || n == "study.json" {
			continue
		}
		if attemptName.MatchString(n) {
			names = append(names, n)
			continue
		}
		if strings.HasPrefix(n, "outcome-") && strings.HasSuffix(n, ".json") {
			continue
		}
		return nil, errors.New("unrecognized journal entry")
	}
	sort.Strings(names)
	seenOutcomes := map[string]bool{}
	attempts := make([]journalAttempt, 0, len(names))
	for _, name := range names {
		var r reservation
		if err := readJournalJSON(j.root, name, &r); err != nil {
			return nil, err
		}
		if r.Version != journalVersion || r.Sequence != j.next || name != fmt.Sprintf("reserve-%06d.json", r.Sequence) || r.MicroUSD < 1 || r.MicroUSD > j.header.BudgetMicroUSD || !validLabID(r.CaseID) || r.RequestHash != hashBytes([]byte(r.RequestJSON)) || r.Model == "" || r.Provider == "" || r.At <= 0 {
			return nil, errors.New("invalid reservation history")
		}
		if r.Pricing.validate() != nil || r.Pricing.Provider != r.Provider || r.Pricing.Model != r.Model || r.Pricing.reserveCost() != r.MicroUSD {
			return nil, errors.New("reservation pricing mismatch")
		}
		outName := fmt.Sprintf("outcome-%06d.json", r.Sequence)
		var out outcome
		if err := readJournalJSON(j.root, outName, &out); err != nil {
			if !audit || !errors.Is(err, os.ErrNotExist) || len(attempts) != len(names)-1 {
				return nil, errors.New("unfinished or corrupt provider attempt; preserve reservation and inspect before another live call")
			}
			if r.MicroUSD > j.header.BudgetMicroUSD-j.charged {
				return nil, errors.New("journal budget exhausted or exceeded")
			}
			j.charged += r.MicroUSD
			attempts = append(attempts, journalAttempt{Reservation: r})
			j.next++
			continue
		}
		if err := validateOutcome(r, out); err != nil {
			return nil, err
		}
		if out.ActualMicroUSD != nil && *out.ActualMicroUSD > r.MicroUSD {
			return nil, errors.New("historical cost overrun requires study review")
		}
		seenOutcomes[outName] = true
		charge := r.MicroUSD
		if out.ActualMicroUSD != nil {
			charge = *out.ActualMicroUSD
		}
		if charge > j.header.BudgetMicroUSD-j.charged {
			return nil, errors.New("journal budget exhausted or exceeded")
		}
		j.charged += charge
		attempts = append(attempts, journalAttempt{Reservation: r, Outcome: &out})
		j.next++
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "outcome-") && !seenOutcomes[e.Name()] {
			return nil, errors.New("orphan provider outcome")
		}
	}
	return attempts, nil
}

func validateOutcome(r reservation, o outcome) error {
	if o.Version != journalVersion || o.Sequence != r.Sequence || o.Reservation != hashBytes(jsonBytes(r)) || o.LatencyNanos < 0 || o.At <= 0 || (o.Status != "completed" && o.Status != "failed" && o.Status != "unknown") {
		return errors.New("invalid bound provider outcome")
	}
	if o.ActualMicroUSD != nil && (!o.UsagePresent || *o.ActualMicroUSD < 0 || o.Status != "completed") {
		return errors.New("cost requires completed response and reported usage")
	}
	if r.Pricing.validate() != nil || r.Pricing.Model != r.Model || r.Pricing.Provider != r.Provider || r.Pricing.reserveCost() != r.MicroUSD {
		return errors.New("reservation pricing mismatch")
	}
	if o.Status == "completed" {
		model, _, priced, err := readUsage([]byte(o.Response), r.Pricing)
		if err != nil || !o.UsagePresent || o.ActualMicroUSD == nil || *o.ActualMicroUSD != priced || model != o.Model {
			return errors.New("unverifiable usage or price")
		}
	}
	return nil
}

func (j *journal) reserve(caseID string, pricing lane, request []byte) (reservation, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	amount := pricing.reserveCost()
	if j.closed || len(j.pending) != 0 || !validLabID(caseID) || pricing.validate() != nil || len(request) == 0 || len(request) > 1<<20 || amount < 1 || amount > j.header.BudgetMicroUSD-j.charged || j.next > maxJournalAttempts {
		return reservation{}, errors.New("reservation refused: bounds, pending attempt or budget")
	}
	r := reservation{Version: journalVersion, Sequence: j.next, CaseID: caseID, Provider: pricing.Provider, Model: pricing.Model, RequestHash: hashBytes(request), RequestJSON: string(request), MicroUSD: amount, Pricing: pricing, At: j.now().UnixNano()}
	reservationBytes := int64(len(jsonBytes(r)) + 1)
	if reservationBytes+(4<<20) > (64<<20)-j.storedBytes {
		return reservation{}, errors.New("journal storage reservation exhausted")
	}
	if err := writeExclusive(j.root, fmt.Sprintf("reserve-%06d.json", r.Sequence), r); err != nil {
		return reservation{}, err
	}
	j.pending[r.Sequence] = r
	j.storedBytes += reservationBytes
	j.charged += amount
	j.next++
	return r, nil
}

func (j *journal) finish(r reservation, o outcome) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	stored, ok := j.pending[r.Sequence]
	if j.closed || !ok || stored != r {
		return errors.New("unknown reservation")
	}
	o.Version = journalVersion
	o.Sequence = r.Sequence
	o.Reservation = hashBytes(jsonBytes(r))
	o.At = j.now().UnixNano()
	if err := validateOutcome(r, o); err != nil {
		return err
	}
	if err := writeExclusive(j.root, fmt.Sprintf("outcome-%06d.json", r.Sequence), o); err != nil {
		return err
	}
	j.storedBytes += int64(len(jsonBytes(o)) + 1)
	delete(j.pending, r.Sequence)
	if o.ActualMicroUSD != nil {
		actual := *o.ActualMicroUSD
		j.charged = j.charged - r.MicroUSD + actual
		if actual > r.MicroUSD {
			j.closed = true
			return errors.New("actual cost exceeded reservation; stop live study")
		}
	}
	return nil
}

func (j *journal) close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	// Do not release a pending/overrun journal for automatic reuse.
	var err error
	if !j.closed && len(j.pending) == 0 {
		err = j.root.Remove(".lock")
	}
	j.closed = true
	return errors.Join(err, j.root.Close())
}

func writeExclusive(r *os.Root, name string, v any) error {
	b, err := json.Marshal(v)
	if err != nil || len(b) > 4<<20 {
		return errors.New("journal write encoding or byte bound")
	}
	f, err := r.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if err == nil {
		b = append(b, '\n')
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	err = errors.Join(err, f.Close())
	if err == nil {
		dir, openErr := r.Open(".")
		if openErr != nil {
			return openErr
		}
		err = errors.Join(dir.Sync(), dir.Close())
	}
	// A partial write is retained as corruption, never deleted to permit reuse.
	return err
}

func readJournalJSON(r *os.Root, name string, v any) error {
	st, err := r.Lstat(name)
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 || st.Size() > 4<<20 {
		return errors.New("invalid private journal file")
	}
	f, err := r.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, (4<<20)+1))
	if err != nil {
		return err
	}
	if len(b) > 4<<20 {
		return errors.New("journal file bound")
	}
	return decodeLabJSON(b, v)
}
