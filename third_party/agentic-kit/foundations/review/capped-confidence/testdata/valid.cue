// valid.cue — positive fixtures for the cappedconfidence schema;
// `cue vet ./testdata/` from the package directory checks them against
// ../schema.cue.
package cappedconfidence

// percentage: a 0–100 table over five evidence classes. The class names are
// this fixture's, not the package's — the vocabulary is always the caller's.
percentage: #Table & {
	max: 100
	ceilings: [
		{class: "primary-record", ceiling: 100, why: "The owner's own record, read from the system the owner writes to. If this is wrong, the record is wrong."},
		{class: "aggregated", ceiling: 80, why: "Accurate when fresh; an aggregator keeps a stale record on the page long after the owner removes it."},
		{class: "guessed-markup", ceiling: 70, why: "Fetched HTML with the structure guessed from the markup. A template change silently re-maps fields and nothing errors."},
		{class: "delimited-line", ceiling: 50, why: "Fields inferred by splitting a line on a delimiter — a convention authors follow loosely, not a schema anything validates."},
		{class: "model-nominated", ceiling: 40, why: "A model's nomination, unverified against any record for THIS subject."},
	]
}

// fivePoint: the same shape over a 0–5 scale — the fixed tiers of
// foundations/review/scoring, expressed as data a table-driven caller can
// re-tune without a code change.
fivePoint: #Table & {
	max: 5
	ceilings: [
		{class: "verified", ceiling: 5, why: "Directly observed — code read, config inspected, behavior seen first-hand."},
		{class: "pre-scan", ceiling: 4, why: "Produced by an automated pass, not yet directly verified."},
		{class: "doc-claimed", ceiling: 2, why: "Asserted by documentation; unverified hearsay never justifies confidence above 2."},
		{class: "sme-stated", ceiling: 2, why: "Asserted by an expert; unverified hearsay never justifies confidence above 2."},
		{class: "assumed", ceiling: 1, why: "No supporting evidence."},
	]
}

// supported: the evidence supports the raw; it renders as is.
supported: #Score & {
	raw:     92
	ceiling: 100
	class:   "primary-record"
	why:     "The owner's own record."
}

// capped: the raw exceeded its ceiling; it renders as a band.
capped: #Score & {
	raw:     92
	ceiling: 50
	class:   "delimited-line"
	clamped: true
	why:     "Fields inferred by splitting a line on a delimiter."
}

// unscored: nothing bounded the raw, and the record says why.
unscored: #Score & {
	raw:     92
	ceiling: 0
	class:   "unlisted"
	reason:  "no ceiling declared for evidence class \"unlisted\""
}
