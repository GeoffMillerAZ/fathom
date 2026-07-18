// Package evidencesignature — valid test fixtures.
// Running `cue vet ./...` and `cue vet ./testdata/` both validate these against schema.cue.
package evidencesignature

// existenceSig: a glob-scoped existence signature.
existenceSig: #ConcernSignature & {
	id:        "concern-existence-001"
	archetype: "existence"
	scope: {
		globs: ["foundations/**/*.go"]
	}
	predicates: [
		{kind: "regexp", arg: "TODO\\(security\\)"},
	]
}

// absenceSig: an absence signature with a mandatory canary.
absenceSig: #ConcernSignature & {
	id:        "concern-absence-001"
	archetype: "absence"
	scope: {
		globs: ["foundations/**/*.go"]
	}
	predicates: [
		{kind: "regexp", arg: "password\\s*=\\s*['\"][^'\"]+['\"]"},
	]
	canary: {
		path:    "foundations/evidence/evidence-signature/testdata/canary_planted.go"
		pattern: "password = \"CANARY_VALUE_DO_NOT_SHIP\""
	}
}

// thresholdSig: a threshold signature requiring >= 3 hits.
thresholdSig: #ConcernSignature & {
	id:        "concern-threshold-001"
	archetype: "threshold"
	scope: {
		source_ids: ["source-registry-001", "source-registry-002"]
	}
	predicates: [
		{kind: "contains", arg: "audit_log"},
	]
	threshold: {
		op:    ">="
		value: 3
	}
}

// contradictsSig: a contradicts signature.
contradictsSig: #ConcernSignature & {
	id:        "concern-contradicts-001"
	archetype: "contradicts"
	scope: {
		globs: ["docs/**/*.md"]
	}
	predicates: [
		{kind: "regexp", arg: "DEPRECATED"},
	]
	retriever_hints: ["full-text-search"]
}
