package ask

// Price is a model's listed price in US dollars per million tokens.
type Price struct {
	InPerMTok  float64 `json:"in_per_mtok"`
	OutPerMTok float64 `json:"out_per_mtok"`
}

// ListedPrice is Jev's listed price as the kit records it
// (third_party/agentic-kit/foundations/decision/typesafe, dated
// 2026-09-29): US$0.042 per million input tokens, output free.
var ListedPrice = Price{InPerMTok: 0.042, OutPerMTok: 0}

// DefaultCapUSD is stage 0's cap (EXP-1).
const DefaultCapUSD = 2.0

// OutputTokensPerQuestion is the conservative output estimate for one
// predicate answer.
const OutputTokensPerQuestion = 32

// EstimateTokens is the conservative token estimate for n bytes of text:
// one token per three bytes, rounded up (English and Go run nearer four).
func EstimateTokens(n int) int { return (n + 2) / 3 }

// Budget is the running spend tally against a cap. Admit is asked with a
// chunk's conservative estimate BEFORE the chunk is sent and refuses (and
// stops the run for good) when that estimate would pass the cap; Charge
// adds a sent chunk's cost from the provider's reported usage when it has
// any, else from the estimate, and counts which basis it used.
type Budget struct {
	CapUSD    float64 `json:"cap_usd"`
	Price     Price   `json:"price"`
	SpentUSD  float64 `json:"spent_usd"`
	Stopped   bool    `json:"stopped"`
	Calls     int     `json:"calls"`
	Reported  int     `json:"calls_reported_usage"`
	Estimated int     `json:"calls_estimated_usage"`
}

// Cost is the price of in input and out output tokens.
func (b *Budget) Cost(in, out int) float64 {
	return float64(in)/1e6*b.Price.InPerMTok + float64(out)/1e6*b.Price.OutPerMTok
}

// Admit reports whether a chunk estimated at estIn/estOut tokens may be
// sent; once it refuses, it refuses every later chunk too.
func (b *Budget) Admit(estIn, estOut int) bool {
	if b.Stopped {
		return false
	}
	if b.SpentUSD+b.Cost(estIn, estOut) > b.CapUSD {
		b.Stopped = true
		return false
	}
	return true
}

// Charge records one sent chunk: reported usage when the provider gave any
// (either count non-zero), else the estimate. It returns the cost charged.
func (b *Budget) Charge(repIn, repOut, estIn, estOut int) float64 {
	b.Calls++
	var c float64
	if repIn > 0 || repOut > 0 {
		b.Reported++
		c = b.Cost(repIn, repOut)
	} else {
		b.Estimated++
		c = b.Cost(estIn, estOut)
	}
	b.SpentUSD += c
	return c
}

// ChunkQuestions and ChunkBytes bound one batched request: the bounds the
// obligations re-rank's decisionBatchPlan uses (64 questions, 96 KiB).
const (
	ChunkQuestions = 64
	ChunkBytes     = 96 * 1024
)

// Chunks groups question sizes (bytes) in order into requests of at most
// maxQuestions questions and maxBytes bytes counting stateBytes once per
// request; a question alone over the bound still gets a request of its
// own. Each chunk is a list of indexes into sizes.
func Chunks(sizes []int, stateBytes, maxQuestions, maxBytes int) [][]int {
	var out [][]int
	var cur []int
	curBytes := stateBytes
	for i, s := range sizes {
		if len(cur) > 0 && (len(cur) >= maxQuestions || curBytes+s > maxBytes) {
			out = append(out, cur)
			cur, curBytes = nil, stateBytes
		}
		cur = append(cur, i)
		curBytes += s
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}
