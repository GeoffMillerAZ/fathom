package console

// decide.go — OC-45 (vision 35 §8, design §3 "Decision port"): the
// decision port's provider and sites, its ruling log, and the calibration
// table from the latest reading.
//
// Every ruling the port answers is one decisions.jsonl row beside the call
// log (internal/decide.Recorder); a hard-failed call is a row carrying an
// "error" (DP-22), so silence in the log means "never asked". The
// calibration is not recomputed here — the log carries no gold labels —
// it is the table of the newest decision-port calibration reading
// (docs/bench/readings/decision-port-calibration-*.md), shown with its path
// and date so a reader can open the evidence behind it.

import (
	"bytes"
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"

	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// DecisionRow is one decisions.jsonl row.
type DecisionRow struct {
	TS           string  `json:"ts"`
	Site         string  `json:"site"`
	Provider     string  `json:"provider"`
	Model        string  `json:"model,omitempty"`
	QuestionID   string  `json:"question_id,omitempty"`
	Kind         string  `json:"kind,omitempty"`
	Choice       string  `json:"choice,omitempty"`
	Confidence   float64 `json:"confidence"`
	LatencyMS    int64   `json:"latency_ms"`
	InputTokens  int     `json:"input_tokens,omitempty"`
	OutputTokens int     `json:"output_tokens,omitempty"`
	Applied      bool    `json:"applied"`
	Questions    int     `json:"questions,omitempty"`
	Error        string  `json:"error,omitempty"`
}

// DecideConfig is the [decide] block the daemon loaded.
type DecideConfig struct {
	Provider        string   `json:"provider"`
	Model           string   `json:"model,omitempty"`
	Endpoint        string   `json:"endpoint,omitempty"`
	Sites           []string `json:"sites"`
	RouteConfidence float64  `json:"route_confidence"`
	Source          string   `json:"source,omitempty"` // which config files it came from
}

// CalibrationRow is one row of a calibration table: a confidence bucket
// (or cutoff) and how often the port was right in it.
type CalibrationRow struct {
	Bucket   string  `json:"bucket"`
	Hits     int     `json:"hits"`
	N        int     `json:"n"`
	Fraction float64 `json:"fraction"`
}

// CalibrationTable is the latest reading's calibration: its buckets and
// its cutoff sweep, with the reading's own verdict sentence.
type CalibrationTable struct {
	Reading string           `json:"reading"`
	Date    string           `json:"date,omitempty"`
	Title   string           `json:"title"`
	Verdict string           `json:"verdict,omitempty"`
	Buckets []CalibrationRow `json:"buckets"`
	Sweep   []CalibrationRow `json:"sweep,omitempty"`
}

// DecideInputs is what DecideView reads. LogReason is non-empty when the
// decisions log could not be read (LogError: it exists and failed);
// ReadingReason when no calibration reading was found.
type DecideInputs struct {
	Config        DecideConfig
	AllSites      []string
	LogPath       string
	Log           []byte
	LogReason     string
	LogError      bool
	Calibration   *CalibrationTable
	ReadingReason string
}

// DecideFilter narrows the ruling log to one site.
type DecideFilter struct {
	Site string `json:"site,omitempty"`
}

// SiteState is one decision site and whether [decide] sites lets it ask.
// Its counts come from the decisions log: with no log they are absent, with
// an unreadable one an error — never a 0 beside "unreadable".
type SiteState struct {
	Site    string     `json:"site"`
	On      bool       `json:"on"`
	Rulings Value[int] `json:"rulings"`
	Applied Value[int] `json:"applied"`
	Errors  Value[int] `json:"errors"`
}

// DecideModel is the decision-port page's projection.
type DecideModel struct {
	Config          DecideConfig      `json:"config"`
	ProviderOn      bool              `json:"provider_on"`
	Sites           []SiteState       `json:"sites"`
	LogPath         string            `json:"log_path"`
	Rulings         Value[int]        `json:"rulings"`
	Applied         Value[int]        `json:"applied"`
	Errors          Value[int]        `json:"errors"`
	MedianLatencyMS Value[int]        `json:"median_latency_ms"`
	Filter          DecideFilter      `json:"filter"`
	Matching        int               `json:"matching"`
	Page            int               `json:"page"`
	Pages           int               `json:"pages"`
	PerPage         int               `json:"per_page"`
	Rows            []DecisionRow     `json:"rows"`
	Calibration     *CalibrationTable `json:"calibration,omitempty"`
	CalibrationNote string            `json:"calibration_note,omitempty"`
}

// DecideView projects the port's configuration, its ruling log (newest
// first, one page) and the latest calibration table.
func DecideView(in DecideInputs, f DecideFilter, page, perPage int) DecideModel {
	if perPage <= 0 {
		perPage = 50
	}
	m := DecideModel{Config: in.Config, LogPath: in.LogPath, Filter: f, PerPage: perPage, Calibration: in.Calibration, CalibrationNote: in.ReadingReason}
	m.ProviderOn = in.Config.Provider != "" && in.Config.Provider != "none"
	source := "decisions log " + in.LogPath
	var rows []DecisionRow
	if in.LogReason == "" {
		for _, line := range bytes.Split(in.Log, []byte{'\n'}) {
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}
			var r DecisionRow
			if json.Unmarshal(line, &r) != nil {
				continue
			}
			// A row written before the kit's generic kind (agentic-kit
			// DP-12) says "noul": read it through decision.ParseKind, so
			// it shows as "predicate" beside the rows written since. An
			// unknown kind is shown as written.
			if k, err := decision.ParseKind(r.Kind); err == nil {
				r.Kind = string(k)
			}
			rows = append(rows, r)
		}
	}
	type siteCount struct{ rulings, applied, errors int }
	perSite := map[string]*siteCount{}
	applied, errs := 0, 0
	var lat []int
	for _, r := range rows {
		s := perSite[r.Site]
		if s == nil {
			s = &siteCount{}
			perSite[r.Site] = s
		}
		s.rulings++
		if r.Error != "" {
			errs++
			s.errors++
			continue
		}
		lat = append(lat, int(r.LatencyMS))
		if r.Applied {
			applied++
			s.applied++
		}
	}
	absent := func() Value[int] {
		if in.LogError {
			return Errored[int](in.LogReason).WithSource(source)
		}
		return Absent[int](in.LogReason).WithSource(source)
	}
	if in.LogReason != "" {
		m.Rulings, m.Applied, m.Errors, m.MedianLatencyMS = absent(), absent(), absent(), absent()
	} else {
		m.Rulings = Measured(len(rows), source).WithDenominator("rows: one per answered question, plus one per failed call")
		m.Applied = Measured(applied, source).WithDenominator("rulings that changed what was served (an unsure-band answer never counts, I42)")
		m.Errors = Measured(errs, source).WithDenominator("calls that failed and answered nothing (DP-22)")
		if _, med, _, ok := quartiles(lat); ok {
			m.MedianLatencyMS = Measured(med, source).WithDenominator("median over " + countNoun(len(lat), "answered ruling"))
		} else {
			m.MedianLatencyMS = Absent[int]("no answered ruling in the log").WithSource(source)
		}
	}
	on := map[string]bool{}
	for _, s := range in.Config.Sites {
		on[s] = true
	}
	seen := map[string]bool{}
	sites := append([]string{}, in.AllSites...)
	for s := range perSite {
		if !containsStr(sites, s) {
			sites = append(sites, s) // a logged site outside today's vocabulary (obligations' wire name)
		}
	}
	for _, s := range sites {
		if seen[s] {
			continue
		}
		seen[s] = true
		st := SiteState{Site: s, On: on[s]}
		switch {
		case in.LogReason != "":
			st.Rulings, st.Applied, st.Errors = absent(), absent(), absent()
		case perSite[s] != nil:
			st.Rulings = Measured(perSite[s].rulings, source)
			st.Applied = Measured(perSite[s].applied, source)
			st.Errors = Measured(perSite[s].errors, source)
		default:
			st.Rulings, st.Applied, st.Errors = Measured(0, source), Measured(0, source), Measured(0, source)
		}
		m.Sites = append(m.Sites, st)
	}

	// The ruling log, newest first, one page.
	var kept []DecisionRow
	for _, r := range rows {
		if f.Site != "" && r.Site != f.Site {
			continue
		}
		kept = append(kept, r)
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].TS > kept[j].TS })
	m.Matching = len(kept)
	m.Pages = max(1, (m.Matching+perPage-1)/perPage)
	page = min(max(page, 1), m.Pages)
	m.Page = page
	start := (page - 1) * perPage
	m.Rows = append([]DecisionRow{}, kept[start:min(start+perPage, m.Matching)]...)
	return m
}

var (
	calibHitsN   = regexp.MustCompile(`(\d+)\s*/\s*(\d+)`)
	calibFrac    = regexp.MustCompile(`(\d*\.\d+|\d+)`)
	calibDate    = regexp.MustCompile(`(\d{4}-\d{2}-\d{2})`)
	calibBold    = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	calibHeading = regexp.MustCompile(`^#{1,6}\s+(.*)$`)
)

// ParseCalibrationReading reads a decision-port calibration reading: its
// title, the bold verdict sentence under "Verdict", the first table under a
// heading naming confidence buckets, and the first table under a heading
// naming the cutoff sweep ("not monotonic"). ok is false when the reading
// carries no bucket table.
func ParseCalibrationReading(path string, md []byte) (CalibrationTable, bool) {
	t := CalibrationTable{Reading: path}
	if d := calibDate.FindString(path); d != "" {
		t.Date = d
	}
	lines := strings.Split(string(md), "\n")
	section := ""
	inVerdict := false
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if h := calibHeading.FindStringSubmatch(line); h != nil {
			if strings.HasPrefix(line, "# ") && t.Title == "" {
				t.Title = strings.TrimSpace(h[1])
			}
			section = strings.ToLower(h[1])
			inVerdict = strings.Contains(section, "verdict")
			continue
		}
		if inVerdict && t.Verdict == "" {
			// The verdict's first bold span, joined across wrapped lines.
			var para []string
			for j := i; j < len(lines) && strings.TrimSpace(lines[j]) != ""; j++ {
				para = append(para, strings.TrimSpace(lines[j]))
			}
			if b := calibBold.FindStringSubmatch(strings.Join(para, " ")); b != nil {
				t.Verdict = b[1]
			}
			continue
		}
		if !strings.HasPrefix(line, "|") {
			continue
		}
		var into *[]CalibrationRow
		switch {
		case strings.Contains(section, "confidence bucket") && len(t.Buckets) == 0:
			into = &t.Buckets
		case strings.Contains(section, "monotonic") && len(t.Sweep) == 0:
			into = &t.Sweep
		default:
			continue
		}
		// A table: header, separator, rows until the first non-table line.
		j := i + 2
		for ; j < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[j]), "|"); j++ {
			if r, ok := parseCalibrationRow(lines[j]); ok {
				*into = append(*into, r)
			}
		}
		i = j - 1
	}
	return t, len(t.Buckets) > 0
}

func parseCalibrationRow(line string) (CalibrationRow, bool) {
	cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
	if len(cells) < 3 {
		return CalibrationRow{}, false
	}
	r := CalibrationRow{Bucket: strings.Trim(strings.TrimSpace(cells[0]), "`*")}
	hn := calibHitsN.FindStringSubmatch(cells[1])
	if hn == nil {
		return CalibrationRow{}, false
	}
	r.Hits, _ = strconv.Atoi(hn[1])
	r.N, _ = strconv.Atoi(hn[2])
	frac := calibFrac.FindString(strings.ReplaceAll(cells[2], "*", ""))
	if frac == "" {
		return CalibrationRow{}, false
	}
	r.Fraction, _ = strconv.ParseFloat(frac, 64)
	return r, true
}
