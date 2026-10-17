package ask

import "sort"

// AgreementItem is one E2 item's answer against its reference label (the
// pre-registration's section 8, where no computed truth exists): P is the
// band probability (a predicate's Noul, a choice's chosen option's
// probability), Agrees says the model's answer equals the label and
// BaselineAgrees says the constant baseline's answer (the majority label)
// does.
type AgreementItem struct {
	P              float64
	Agrees         bool
	BaselineAgrees bool
}

// AgreementBand is one probability band of an agreement reading: Read's
// columns with agreement in place of accuracy, since a label is a
// reviewer's reading and not a computed truth.
type AgreementBand struct {
	Band              string  `json:"band"`
	N                 int     `json:"n"`
	Share             float64 `json:"share"`
	Agreement         float64 `json:"agreement"`
	WilsonLow         float64 `json:"wilson_low"`
	WilsonHigh        float64 `json:"wilson_high"`
	BaselineAgreement float64 `json:"baseline_agreement"`
	Lift              float64 `json:"lift"`
}

// AgreementReading is one E2 question's reading under EXP-M1's bar:
// Qualifies when one band holding at least 50% of the items reads
// agreement of at least 0.9 and strictly beats the baseline's agreement on
// those items; QualifyingBand names the first such band.
type AgreementReading struct {
	N                 int             `json:"n"`
	Agreement         float64         `json:"agreement"`
	BaselineAgreement float64         `json:"baseline_agreement"`
	Lift              float64         `json:"lift"`
	Bands             []AgreementBand `json:"bands"`
	Qualifies         bool            `json:"qualifies"`
	QualifyingBand    string          `json:"qualifying_band,omitempty"`
}

// ReadAgreement computes an agreement reading over items with E1's bands
// and Wilson bounds; on predicate items (Agrees = (P >= 0.5) == label) it
// reads exactly what Read reads with the label as the truth.
func ReadAgreement(items []AgreementItem) AgreementReading {
	r := AgreementReading{N: len(items), Bands: make([]AgreementBand, len(Bands))}
	type tally struct{ n, model, base int }
	per := make([]tally, len(Bands))
	var model, base int
	for _, it := range items {
		b := bandOf(it.P)
		per[b].n++
		if it.Agrees {
			per[b].model++
			model++
		}
		if it.BaselineAgrees {
			per[b].base++
			base++
		}
	}
	r.Agreement, r.BaselineAgreement = accuracy(model, len(items)), accuracy(base, len(items))
	r.Lift = r.Agreement - r.BaselineAgreement
	for i, b := range Bands {
		t := per[i]
		br := AgreementBand{Band: b.Name, N: t.n, Agreement: accuracy(t.model, t.n), BaselineAgreement: accuracy(t.base, t.n)}
		if len(items) > 0 {
			br.Share = float64(t.n) / float64(len(items))
		}
		br.WilsonLow, br.WilsonHigh = Wilson(t.model, t.n)
		br.Lift = br.Agreement - br.BaselineAgreement
		r.Bands[i] = br
		if !r.Qualifies && t.n > 0 && br.Share >= 0.5 && br.Agreement >= 0.9 && br.Agreement > br.BaselineAgreement {
			r.Qualifies, r.QualifyingBand = true, b.Name
		}
	}
	return r
}

// Majority is the most frequent of labels, the constant answer E2's
// baseline gives: a tie goes to the label earliest in order, and a label
// order does not list comes after every listed one, in byte order. It is
// "" for no labels.
func Majority(labels []string, order []string) string {
	count := map[string]int{}
	for _, l := range labels {
		count[l]++
	}
	rank := func(l string) int {
		for i, o := range order {
			if o == l {
				return i
			}
		}
		return len(order)
	}
	names := make([]string, 0, len(count))
	for l := range count {
		names = append(names, l)
	}
	sort.Slice(names, func(i, j int) bool {
		a, b := names[i], names[j]
		if count[a] != count[b] {
			return count[a] > count[b]
		}
		if ra, rb := rank(a), rank(b); ra != rb {
			return ra < rb
		}
		return a < b
	})
	if len(names) == 0 {
		return ""
	}
	return names[0]
}
