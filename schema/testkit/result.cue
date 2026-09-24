// result.cue — #TestResult: one envelope, every framework (ADR-0042 §2).
//
// The engine never trusts a framework's stdout as the record. Runner
// adapters convert native output INTO this shape, and this shape is the
// record; JUnit XML and CTRF are emitted FROM it, one-way, forever. A
// surface that can only be fed by reading a projection back is a wrong
// surface, not a reason to relax the rule.
package testkit

// #Verdict — flake-pass is a MEMBER, not a flavour of pass. There is no
// field anywhere in this schema that maps one onto the other, which is
// what makes "a retry-rescued suite is visible on every surface"
// (ADR-0042) structural rather than a reporting convention someone can
// quietly drop.
#Verdict: "pass" | "fail" | "flake-pass" | "quarantined" | "error"

#CaseStatus: "pass" | "fail" | "skip" | "flake"

// #FailureKind is ADR-0042's failure taxonomy. It is load-bearing, not
// decoration: `infra` failures never poison a key's verdict history, so a
// runner that reports every failure as "assertion" is destroying the
// signal prediction and fleet health depend on.
#FailureKind: "assertion" | "timeout" | "panic" | "infra" | "dependency"

#CaseFailure: {
	kind:    #FailureKind
	message: string & !=""
	where?:  string
}

#Case: {
	id:       string & !=""
	name:     string & !=""
	status:   #CaseStatus
	duration: string
	failure?: #CaseFailure
	// attempts is REQUIRED with a default of 1 rather than optional: a
	// runner cannot omit its way out of admitting a retry (ADR-0042's
	// "retry lineage is data, never hidden"). An optional field would
	// make "no retries" and "did not say" the same document.
	attempts: int & >=1 | *1
	// A case that failed and then passed is `flake`, and a flake by
	// definition took more than one attempt. The pairing is enforced here
	// so a runner cannot report a clean single-attempt flake.
	if status == "flake" {
		attempts: int & >=2
	}
}

#Totals: {
	run:     int & >=0
	passed:  int & >=0
	failed:  int & >=0
	skipped: int & >=0
	flaked:  int & >=0
}

#CoverageReport: {
	statements?: number & >=0 & <=100
	branches?:   number & >=0 & <=100
	perPackage?: [string]: number & >=0 & <=100
}

#Timing: {
	startedAt:   string & =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}T"
	wallClock:   string
	perCaseP95?: string
}

#EnvInfo: {
	toolchainDigest: string
	runnerClass:     string
	arch:            string
	os:              string
}

// #ArtifactRef is a CAS reference — logs, coverage profiles, screenshots.
// A digest, never a path: a path is a claim about a filesystem that will
// not exist when somebody reads this envelope.
#ArtifactRef: {
	name:   string & !=""
	digest: string & =~"^sha256:[a-f0-9]{64}$"
}

#TestResult: {
	suite: {
		component: string & !=""
		name:      string & !=""
		class:     #SuiteClass
	}
	// key binds the result to the executor's input closure. Opaque to
	// test-kit for the same reason #Suite.inputs is: whose closure, and
	// how hashed, is the executor's business.
	key:     string
	verdict: #Verdict
	cases: [...#Case]
	totals:    #Totals
	coverage?: #CoverageReport
	timing:    #Timing
	env:       #EnvInfo
	trace?: traceId: string
	artifacts: [...#ArtifactRef] | *[]
	// flake is the policy the verdict was judged UNDER, round-tripped from
	// the manifest. Without it a reader cannot tell whether a flake-pass
	// was within budget, and "was this allowed" becomes a second lookup
	// against a manifest that may since have changed.
	flake: #FlakePolicy

	// A verdict of flake-pass with nothing flaked, or a fail with nothing
	// failed, is an envelope contradicting itself. Refused rather than
	// reported.
	if verdict == "flake-pass" {
		totals: flaked: int & >=1
	}
	if verdict == "fail" {
		totals: failed: int & >=1
	}
}
