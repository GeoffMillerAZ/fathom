package main

// modelprice.go — refactor-conservation gap-closing wave 3, item 2
// (docs/plans/refactor-conservation.md): the small, explicit table of
// models this fathom binary has a real per-1k-token USD rate for.
//
// Before this file existed, compileViaStore's dry-run estimate never
// resolved a price at all — WithLLM (the only call site that sets
// Compiler.InRatePerKTok/OutRatePerKTok) ran only inside the `!dryRun`
// branch, so EVERY dry run's cost band computed from a zero rate and
// printed the confident-looking "est. 0.00-0.00", even when --profile
// named a real, priced model. rateForModel lets a dry run resolve the
// SAME price a spending compile of the identical model would use, so the
// estimate is a genuine range when the model is known, and an honest
// disclosure ("cost unknown: no price for model <m>") rather than a
// fabricated zero when it is not.
//
// Deliberately small: the one entry priced here (defaultCompileModel) is
// the SAME flat "API-equivalent accounting" rate compileViaStore's real,
// non-dry-run path already applies once an LLM client is built
// (buildLLMClient's own doc comment explains why every provider/model
// shares one flat rate rather than a literal per-provider price) —
// extending that rate to the dry-run estimate for the one model it is
// calibrated against, never inventing numbers for a model this repo has
// not priced. Growing this table for a second model is a data decision,
// not a default.
const (
	// knownModelInRatePerKTok / knownModelOutRatePerKTok are the flat
	// per-1k-token USD rate for defaultCompileModel — the SAME constants
	// compile.go's and servehttp.go's real (non-dry-run) WithLLM calls
	// reference directly (a review found they still hardcoded their own
	// 0.001/0.005 literal here, so rateForModel and those call sites read
	// the identical number under one name, never a second, silently
	// driftable literal; TestRealRunWithLLMCallSitesShareNamedRateConstants,
	// modelprice_test.go, pins both call sites against a lexical drift
	// check).
	knownModelInRatePerKTok  = 0.001
	knownModelOutRatePerKTok = 0.005
)

// knownModelRates names every model fathom has a real price for.
var knownModelRates = map[string]bool{
	defaultCompileModel: true,
}

// rateForModel reports the per-1k-token USD input/output rate fathom
// knows for model, and whether one is known at all. Never guesses: an
// empty model, an unrecognized --profile pin, an unpriced
// FATHOM_COMPILE_MODEL, or a Bedrock inference-profile ARN this table has
// never been told about all come back ok=false, so a caller can disclose
// "cost unknown" instead of silently printing a wrong (or zero-shaped)
// number.
func rateForModel(model string) (inRatePerKTok, outRatePerKTok float64, ok bool) {
	if model == "" || !knownModelRates[model] {
		return 0, 0, false
	}
	return knownModelInRatePerKTok, knownModelOutRatePerKTok, true
}
