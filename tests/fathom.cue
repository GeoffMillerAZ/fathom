// SCAFFOLD — `surveyor init` wrote this #TestManifest for "fathom".
// It vets clean exactly as written, which is the point: adoption starts
// from a declaration that PASSES and is not yet true, rather than a
// template that fails until it is finished.
//
// Nothing here was invented. The 3 values the wizard could not know are
// marked at the field they belong to — grep this file for ADOPT-TODO.
// Where a number or a path could only have been a guess, the field is
// absent or commented out instead of holding a plausible placeholder.

component: "fathom"

// suites is MAP-KEYED by suite name, never a list, for the reason every
// policy in this fleet is map-keyed: a list lets a later override drop a
// row by omitting it and say nothing, while a missing map key is a
// visible edit. The key is the name that shows up in evidence, so rename
// it to whatever you actually call this suite.
suites: {
	unit: {
		class:  "unit"
		runner: "go-test"

		// inputs is this suite's TESTED BYTES and must be non-empty:
		// ADR-0042's "tests are declared, never discovered" IS this floor,
		// and a suite with no inputs fails `cue vet` rather than quietly
		// globbing the tree. test-kit never looks inside — the shape is
		// whatever your executor keys on.
		//
		// Narrowed from the scaffold's "./..." to "./internal/..." (TC-M1,
		// vision 33 / ADR-043 stage 0): the engine lives there and
		// `go test -json ./internal/...` (no -race) is the run this
		// manifest's evidence is measured against — cmd/fathom and pkg/
		// are not yet in this suite's declared closure.
		inputs: paths: ["./internal/..."]

		// runtime is left at #Runtime's own defaults — class
		// "trusted-spot", arch ["amd64"], timeout "10m". Restating them
		// here would look more explicit and be worse: the copy stops
		// tracking the schema the moment a default moves.
		runtime: {}

		// tier "blocking" is the only default that cannot lie. An advisory
		// suite nobody has to fix is a green gate proving nothing
		// (ADR-0070) — and note #Tier has no "skipped" member: a test you
		// have stopped running is deleted, not hidden behind a vocabulary
		// word that makes the manifest look complete.
		tier: "blocking"

		// #Coverage takes a FLOOR (min) and deliberately has no target: an
		// aspirational number is not a floor. So this stays absent until
		// you have a measurement — a scaffolded number would be a floor
		// nobody chose, and the one number guaranteed never to fail.
		// ADOPT-TODO: suites.unit.coverage: no floor is declared because nothing has been measured — add a min once you have run the suite, or leave it absent
	}

	// A conformance suite is reserved for PORT ADAPTERS, and it is where
	// ADR-0003's "every port has a shared conformance suite" becomes
	// machine-checkable: `port` is REQUIRED on class "conformance" and
	// the file will not vet without it. Uncomment this block once this
	// component has a port with more than one adapter behind it.
	// ADOPT-TODO: suites.<name>.port: a conformance suite must name the port it conforms, and only your code can supply that path — see the commented block below
	//
	// "store-conformance": {
	// 	class:  "conformance"
	// 	runner: "go-test"
	// 	port:   "<your/package.Port>"
	// 	inputs: paths: ["./internal/adapters/..."]
	// 	runtime: {}
	// 	tier: "blocking"
	// }
}
