package core

// CitationStatus qualifies an index-sourced location against the bytes read in
// this request. A nil status is clean and leaves the existing wire shape intact.
type CitationStatus struct {
	State  string `json:"state"`
	Reason string `json:"reason"`
}
