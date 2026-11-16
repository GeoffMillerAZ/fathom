package benchenv

#Text:          string & =~"\\S"
#Digest:        string & =~"^[0-9a-f]{64}$"
#Commit:        string & =~"^[0-9a-f]{40}$"
#Tier:          "public" | "constructed" | "smoke"
#Split:         "dev" | "held-out"
#Axis:          "lines" | "files" | "repositories" | "history"
#ScaleClass:    "S" | "M" | "L" | "XL"
#Prediction:    "effect" | "null"
#Polarity:      "positive" | "negative"
#ExclusionKind: "vendored" | "generated" | "test" | "build"
#Ref: close({id!: #Text, path!: #Text, sha256!: #Digest})
#RepositoryPin: close({id!: #Text, url!: string, commit!: #Commit})
#Pin: close({name!: #Text, version!: #Text, sha256!: #Digest})
#Int64: int & >=0 & <=9223372036854775807
#Int32: int & >=0 & <=2147483647
#ExcludedCount: close({kind!: #ExclusionKind, lines!: #Int64, files!: #Int64})
#Scale: close({
	lines!:        null | #Int64
	files!:        null | #Int64
	repositories!: null | #Int64
	history!:      null | #Int64
	excluded!: [...#ExcludedCount]
	receipt!: #Ref
})
#ScaleClaim: close({axis!: #Axis, class!: #ScaleClass})
#EnvironmentHead: close({
	id!:   #Text
	tier!: #Tier
	repositories!: [#RepositoryPin, ...#RepositoryPin]
	subjects!: [...#Ref]
	licence!: #Ref
	toolchains!: [#Pin, ...#Pin]
	images!: [...#Pin]
	source_policy!: #Ref
	scale!:         #Scale
	split!:         #Split
	prior_reads!: [...#Ref]
	oracles!: [#Ref, ...#Ref]
	receipts!: [#Ref, ...#Ref]
})
#Stratum: close({
	subtype!:  "" | #Text
	language!: #Text
	scale!: [#ScaleClaim, ...#ScaleClaim]
	prediction!: #Prediction
})
#Requirement: close({
	id!:               #Text
	capability!:       #Text
	card!:             #Ref
	checker_contract!: #Ref
	concept!:          #Text
	stratum!:          #Stratum
	positive_minimum!: #Int32 & >=30
	negative_minimum!: #Int32 & >=30
	checker!:          #Pin
})
#Anchor: close({
	repository!: #RepositoryPin
	path!:       #Text
	start_line!: #Int32 & >=1
	end_line!:   #Int32 & >=start_line
})
#Witness: close({
	id!:          #Text
	environment!: #Ref
	requirement!: #Ref
	anchor!:      #Anchor
	polarity!:    #Polarity
	match_id!:    #Text
	origin!:      #Ref
	checker!:     #Pin
	receipt!:     #Ref
})
#CoverageClaim: close({
	id!:          #Text
	capability!:  #Text
	environment!: #Ref
	stratum!:     #Stratum
	requirements!: [#Ref, ...#Ref]
})
#EnvironmentRecords: close({
	environments!: [...#EnvironmentHead]
	requirements!: [...#Requirement]
	witnesses!: [...#Witness]
	coverage!: [...#CoverageClaim]
})

#MetricUnit:      "symbols" | "entry_points" | "paths" | "links" | "repositories" | "services" | "sites" | "changes" | "expressions" | "cases" | "transitions" | "instructions" | "fixes" | "mutants" | "issues" | "pairs" | "commits" | "reads" | "escapes" | "tasks" | "deletions" | "labels" | "inputs" | "envelopes" | "bindings" | "outputs" | "questions"
#MinimumCitation: "BE-D2:False plan claims" | "BE-D2:Cross-repository dependencies" | "BE-D2:Fan-in and dispatch" | "BE-D2:Output fidelity and judgments"
#ExpectedMetric:  "absent_symbols" | "unwired_entries" | "cross_file_paths" | "repository_links" | "linked_repositories" | "linked_services" | "fanin_symbols" | "dispatch_sites" | "hazardous_changes" | "safe_changes" | "module_expressions" | "missing_context" | "provider_transitions" | "stale_instructions" | "live_instructions" | "governing_links" | "public_fixes" | "collateral_mutants" | "nonlexical_issues" | "lexical_issues" | "conflicting_pairs" | "clean_pairs" | "history_commits" | "relevant_changes" | "boundary_sites" | "language_sites" | "data_reads" | "subprocess_reads" | "uncovered_changes" | "acquire_release_pairs" | "missing_release_fixes" | "escapes" | "bounded_sites" | "unbounded_sites" | "shutdown_cases" | "multisession_tasks" | "declared_deletions" | "undeclared_deletions" | "rule_labels" | "pinned_inputs" | "floating_inputs" | "interface_changes" | "valid_envelopes" | "invalid_envelopes" | "resolvable_bindings" | "ambiguous_bindings" | "output_cases" | "long_outputs" | "short_outputs" | "question_cases" | "route_questions" | "rank_questions" | "table_questions"
#MetricExpectation: close({
	metric!:        #ExpectedMetric
	unit!:          #MetricUnit
	minimum:        *30 | (int & >=1 & <=9223372036854775807)
	lower_minimum?: #MinimumCitation
}) & (
	// Projection of the sole canonical relation above, not another policy.
	{metric!: ("absent_symbols" | "fanin_symbols"), unit!: "symbols"} |
	{metric!: ("unwired_entries"), unit!: "entry_points"} |
	{metric!: ("cross_file_paths"), unit!: "paths"} |
	{metric!: ("repository_links" | "governing_links"), unit!: "links"} |
	{metric!: ("linked_repositories"), unit!: "repositories"} |
	{metric!: ("linked_services"), unit!: "services"} |
	{metric!: ("dispatch_sites" | "boundary_sites" | "language_sites" | "bounded_sites" | "unbounded_sites"), unit!: "sites"} |
	{metric!: ("hazardous_changes" | "safe_changes" | "relevant_changes" | "uncovered_changes" | "interface_changes"), unit!: "changes"} |
	{metric!: ("module_expressions"), unit!: "expressions"} |
	{metric!: ("missing_context" | "shutdown_cases"), unit!: "cases"} |
	{metric!: ("provider_transitions"), unit!: "transitions"} |
	{metric!: ("stale_instructions" | "live_instructions"), unit!: "instructions"} |
	{metric!: ("public_fixes" | "missing_release_fixes"), unit!: "fixes"} |
	{metric!: ("collateral_mutants"), unit!: "mutants"} |
	{metric!: ("nonlexical_issues" | "lexical_issues"), unit!: "issues"} |
	{metric!: ("conflicting_pairs" | "clean_pairs" | "acquire_release_pairs"), unit!: "pairs"} |
	{metric!: ("history_commits"), unit!: "commits"} |
	{metric!: ("data_reads" | "subprocess_reads"), unit!: "reads"} |
	{metric!: ("escapes"), unit!: "escapes"} |
	{metric!: ("multisession_tasks"), unit!: "tasks"} |
	{metric!: ("declared_deletions" | "undeclared_deletions"), unit!: "deletions"} |
	{metric!: ("rule_labels"), unit!: "labels"} |
	{metric!: ("pinned_inputs" | "floating_inputs"), unit!: "inputs"} |
	{metric!: ("valid_envelopes" | "invalid_envelopes"), unit!: "envelopes"} |
	{metric!: ("resolvable_bindings" | "ambiguous_bindings"), unit!: "bindings"} |
	{metric!: ("output_cases" | "long_outputs" | "short_outputs"), unit!: "outputs"} |
	{metric!: ("question_cases" | "route_questions" | "rank_questions" | "table_questions"), unit!: "questions"} ) & (
	*{minimum: >=30, lower_minimum?: _|_} |
	{metric!: ("absent_symbols" | "unwired_entries"), minimum: >=1 & <30, lower_minimum!: "BE-D2:False plan claims"} |
	{metric!: ("linked_repositories" | "linked_services"), minimum: >=3 & <30, lower_minimum!: "BE-D2:Cross-repository dependencies"} |
	{metric!: ("fanin_symbols"), minimum: >=3 & <30, lower_minimum!: "BE-D2:Fan-in and dispatch"} |
	{metric!: ("long_outputs" | "short_outputs" | "route_questions" | "rank_questions" | "table_questions"), minimum: >=1 & <30, lower_minimum!: "BE-D2:Output fidelity and judgments"} ) & ({metric!: !="history_commits"} | {metric!: "history_commits", minimum: >=200})
#ConceptDesignRow: "BE-D2" | "BE-D9"
#ConceptCitation: close({card!: #Ref}) | close({design_row!: #ConceptDesignRow})
#ConceptRecord: close({
	required_by!: [#ConceptCitation, ...#ConceptCitation]
	expectations!: [#MetricExpectation, ...#MetricExpectation]
})
#ConceptGroup: close({
	card!:             #Ref
	checker_contract!: #Ref
	concepts!: [#Text, ...#Text]
})
#ConceptInventory: close({concepts!: {[#Text]: #ConceptRecord, [string & !~"\\S"]: _|_}, groups!: [...#ConceptGroup]})

#FleetID: string & =~"^[a-z][a-z0-9-]*$"
#FleetFile: close({
	path!:          #Text, content_id!: #FleetID
	source_sha256!: #Digest, sha256!:   #Digest
	mode!:          "100644" | "100755"
	class!:         "product" | "vendored" | "generated" | "test" | "build"
	seeded!:        bool
})
#FleetCommit: close({id!: #FleetID, parents!: [...#FleetID], files!: [...#FleetFile]})
#FleetRepository: close({id!: #FleetID, head!: #FleetID, commits!: [#FleetCommit, ...#FleetCommit]})
#FleetSize: close({lines!: #Int64, files!: #Int64, repositories!: #Int64, history!: #Int64})
#GeneratorVersion: "env-fleet/v1"
#FleetManifest: close({
	seed!:              int & >=0 & <=18446744073709551615
	generator_version!: #GeneratorVersion, template!: #Text, size!: #FleetSize
	repositories!: [#FleetRepository, ...#FleetRepository]
	truth!: [#FleetFile, ...#FleetFile]
})
#FleetLimits: close({
	max_repositories!: #Int64 & >0, max_commits!: #Int64 & >0
	max_files!:        #Int64 & >0, max_bytes!:   #Int64 & >0
})

#UTC: string & =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$"
#PreflightInput: close({
	pair_id!:        #Text, arm!:       #Text, environment!: #Ref
	profile_sha256!: #Digest, receipt!: #Ref
})
#AgentOutcome: close({
	pair_id!: #Text, arm!: #Text, cell_key!: #Digest, run_id!: #Text, raw!: #Ref
})
#FreshReadingInput: close({
	id!:          #Text, run_id!: #Text, measured_at!: #UTC
	environment!: #Ref, matrix!:  #Ref, fingerprint!:  #Ref, raw!: #Ref
	preflights!: [#PreflightInput, ...#PreflightInput]
	outcomes!: [#AgentOutcome, ...#AgentOutcome]
})
#FixturesInput: close({records!: #EnvironmentRecords, catalog!: #Ref, evidence!: [...#Ref]})
#PortfolioInput: close({
	records!: #EnvironmentRecords, catalog!: #Ref, subjects!: [...#Ref], evidence!: [...#Ref]
})
#BaselineInput: close({
	matrix!: #Ref, preflights!: [#PreflightInput, #PreflightInput, ...#PreflightInput]
	outcomes!: [...#AgentOutcome]
})
#AuditInput: close({
	sources!: [#Ref, ...#Ref], runs!: [#Ref, ...#Ref], rulings!: [#Ref, ...#Ref]
})
#SeedInput: close({
	manifest!: #FleetManifest, sources!: [#Ref, ...#Ref], limits!: #FleetLimits
	requirements!: [...#Requirement], catalog!: #Ref
})
#CandidateInput: close({
	subject_input!: #Ref, records!: #EnvironmentRecords, catalog!: #Ref, evidence!: [...#Ref]
})
#SealInput: close({
	admission!: #Ref, environment!: #Ref, prior_reads!: [...#Ref]
	topology!: #Ref, oracle_keys!: [...#Ref]
})
#ReadingInput: close({
	reading!: #FreshReadingInput, claim_time!: #UTC, current_fingerprint!: #Ref
	existing_runs!: [...#Text], existing_cell_keys!: [...#Digest]
})
#SnapshotInput: close({
	product_commit!: #Commit, binary_sha256!: #Digest
	commits!: [#Commit, #Commit, #Commit, #Commit, #Commit, #Commit, #Commit,
		#Commit, #Commit, #Commit, #Commit, #Commit, #Commit, #Commit,
		#Commit, #Commit, #Commit, #Commit, #Commit, #Commit, #Commit]
	service!: #Text, wall_cap_ms!: #Int64 & >0
})
#ModelRerunInput: close({
	reading_id!:     #Text, preregistration!: #Ref, approval!: #Ref, spend!: #Ref
	cell_namespace!: #Text, existing_namespaces!: [...#Text]
	max_usd_micros!: #Int64 & >0, wall_cap_ms!: #Int64 & >0
})
#LandedReplayInput: close({input!: #Ref, wall_cap_ms!: #Int64 & >0})
#EnvironmentRequest:
	close({mode!: "fixtures", fixtures!: #FixturesInput}) |
	close({mode!: "portfolio", portfolio!: #PortfolioInput}) |
	close({mode!: "baseline", baseline!: #BaselineInput}) |
	close({mode!: "audit", audit!: #AuditInput}) |
	close({mode!: "seed", seed!: #SeedInput}) |
	close({mode!: "candidate", candidate!: #CandidateInput}) |
	close({mode!: "seal", seal!: #SealInput}) |
	close({mode!: "reading", reading!: #ReadingInput}) |
	close({mode!: "snapshots", snapshots!: #SnapshotInput}) |
	close({mode!: "model-rerun", model_rerun!: #ModelRerunInput}) |
	close({mode!: "landed-replay", landed_replay!: #LandedReplayInput})
