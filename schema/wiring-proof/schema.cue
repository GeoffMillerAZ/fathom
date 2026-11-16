// Failed computed relations constrain the named non-nullable parent to null,
// preserving a field-local CUE diagnostic rather than an unlocated bottom.
// WPF-1 validates reported shapes and bindings, not source/hash or installation truth.
package wiringproof

#Text:            string & =~"\\S"
#LabID:           string & =~"^lab-[a-z0-9]+(-[a-z0-9]+)*$"
#SHA:             string & =~"^[0-9a-f]{64}$"
#Commit:          string & =~"^([0-9a-f]{40}|[0-9a-f]{64})$"
#Time:            int & >=0
#EnvRung:         "W0" | "W1" | "W2" | "W3" | "W6"
#Verdict:         "PASS" | "FAIL" | "UNKNOWN" | "NOT-APPLICABLE"
#Phase:           "preflight" | "agent"
#Harness:         "claude-code" | "codex-cli"
#EnvironmentKind: "local" | "cloud" | "wsl"
#HostClass:       "macos" | "linux" | "windows"
#SourceKind:      "registration" | "negotiation" | "call_result" | "startup_delivery" | "hook_trigger" | "hook_event" | "beat" | "revision" | "resource_sample" | "channel_inventory" | "eligibility" | "action" | "action_capture"
#BenchHarness:    "replay" | "fork" | "scenario" | "hook-path" | "decide" | "indexbench" | "reviewbench" | "contextbench" | "langbench"
#Channel:         "surface" | "stanza" | "descriptions" | "hooks" | "skills" | "prime" | "maps" | "readouts"
#Metric:          "index_age_ns" | "payload_age_ns" | "hook_p95_ns" | "phys_footprint_bytes" | "resident_bytes" | "cpu_seconds_per_ingest"

#Pins: {
	harness!:                    #Harness
	harness_pin!:                #Text
	model_pin!:                  #Text
	transcript_contract_digest!: #SHA
	product_commit!:             #Commit
	subject_commit!:             #Commit
	plugin_digest!:              #SHA
	stanza_sha!:                 #SHA
	description_sha!:            #SHA
}
#RelativePath: #Text & !~"(^/|^[A-Za-z]:|\\\\|(^|/)(\\.|\\.\\.)($|/)|//|/$)"
#Translation: {guest_path!: #RelativePath, fixture_path!: #RelativePath}
#Environment: {
	kind!:        #EnvironmentKind
	host_class!:  #HostClass
	harness_pin!: #Text
	permitted_tools!: [...#Text]
	startup_instructions_digest!: #SHA
	action_capture_digest!:       #SHA
	for i, x in permitted_tools if i > 0 {
		if permitted_tools[i-1] >= x {permitted_tools!: null}
	}
	if kind == "wsl" {
		host_class!:     "windows"
		guest_revision!: #Text
		path_translation!: [#Translation, ...#Translation]
		for i, x in path_translation if i > 0 {
			if path_translation[i-1].guest_path >= x.guest_path {path_translation!: null}
		}
	}
}
#ChannelSpec: {channel!: #Channel, enabled!: bool, content_digest!: #SHA}
#HookSpec: {id!: #Text, enabled!: bool, trigger_digest!: #SHA, event!: #Text, timeout_ns!: int & >0}
#Requirement: {rung!: #EnvRung, applicable!: bool, reason!: #Text}
#Bound: {metric!: #Metric, maximum!: number & >=0}
#Budget: {
	tenant_policy_digest!: #SHA
	bounds!: [#Bound, ...#Bound]
	for i, x in bounds if i > 0 {
		bounds: [for j, y in bounds {if j == i {metric: >bounds[i-1].metric}}]
	}
}
#SourceDigest: {id!: #Text, digest!: #SHA}
#Profile: {
	id!:               #Text
	expected_service!: #Text
	product_present!:  bool
	lab!:              #LabID
	bench_harness!:    #BenchHarness
	pins!:             #Pins
	environment!: #Environment & {harness_pin!: pins.harness_pin}
	if environment.kind == "wsl" {pins!: harness!: "codex-cli"}
	channels!: [
		#ChannelSpec & {channel!: "descriptions"},
		#ChannelSpec & {channel!: "hooks"},
		#ChannelSpec & {channel!: "maps"},
		#ChannelSpec & {channel!: "prime"},
		#ChannelSpec & {channel!: "readouts"},
		#ChannelSpec & {channel!: "skills"},
		#ChannelSpec & {channel!: "stanza"},
		#ChannelSpec & {channel!: "surface"},
	]
	hooks!: [...#HookSpec]
	_enabledHooks: [for h in hooks if h.enabled {h}]
	channels!: [{}, {enabled!: len(_enabledHooks) > 0}, {}, {}, {}, {}, {}, {}]
	for i, h in hooks if i > 0 {
		if hooks[i-1].id >= h.id {hooks!: null}
	}
	surface_tools!: [...#Text]
	for i, x in surface_tools if i > 0 {
		if surface_tools[i-1] >= x {surface_tools!: null}
	}
	requirements!: [
		#Requirement & {rung!: "W0", applicable!: product_present},
		#Requirement & {rung!: "W1", applicable!: channels[7].enabled},
		#Requirement & {rung!: "W2", applicable!: channels[3].enabled},
		#Requirement & {rung!: "W3", applicable!: len(_enabledHooks) > 0},
		#Requirement & {rung!: "W6", applicable!: true},
	]
	if channels[7].enabled || channels[3].enabled || len(_enabledHooks) > 0 {product_present!: true}
	if !channels[7].enabled {surface_tools!: []}
	workload_digest!: #SHA
	budget!:          #Budget
	_hostMetric:      string
	if environment.host_class == "macos" {_hostMetric: "phys_footprint_bytes"}
	if environment.host_class != "macos" {_hostMetric: "resident_bytes"}

	// Host resource accounting is required even without a daemon.
	if !(len([for bound in budget.bounds if bound.metric == _hostMetric {bound}]) > 0) {budget!: null}
	if product_present {
		for m in ["index_age_ns", "payload_age_ns"] {
			if !(len([for bound in budget.bounds if bound.metric == m {bound}]) > 0) {budget!: null}
		}
	}
	if len(_enabledHooks) > 0 {
		if !(len([for bound in budget.bounds if bound.metric == "hook_p95_ns" {bound}]) > 0) {budget!: null}
	}
	budget_digest!: #SHA
	source_digests!: [#SourceDigest, ...#SourceDigest]
	for i, x in source_digests if i > 0 {
		if source_digests[i-1].id >= x.id {source_digests!: null}
	}
	if !(len([for member in [for x in source_digests {x.id}] if member == "delivery_parameters" {member}]) > 0) {source_digests!: null}
	profile_digest!:     #SHA
	environment_digest!: #SHA
	frozen_at!:          #Time
}
#CaptureSummary: {
	capability!: #Text
	complete!:   bool
	action_evidence_ids!: [...#Text]
	for i, x in action_evidence_ids if i > 0 {
		if action_evidence_ids[i-1] >= x {action_evidence_ids!: null}
	}
}
#Evidence: {
	id!:          #Text
	service?:     #Text
	source_kind!: #SourceKind
	phase!:       #Phase
	session_id!:  #Text
	call_id!:     #Text
	range!: {artifact_digest!: #SHA, start!: int & >=0, end!: int & >start}
	observed_at!:    #Time
	product_commit!: #Commit
	subject_commit!: #Commit
	profile_digest!: #SHA
	harness_pin!:    #Text
	reason!:         #Text
	if source_kind == "action_capture" {
		phase!:   "agent"
		capture!: #CaptureSummary
	}
}
#RungResult: {
	verdict!: #Verdict
	reason!:  #Text
	evidence_ids!: [...#Text]
	if verdict != "UNKNOWN" {evidence_ids!: [#Text, ...#Text]}
	for i, x in evidence_ids if i > 0 {
		if evidence_ids[i-1] >= x {evidence_ids!: null}
	}
}
#Absence: {
	channel!: #Channel
	evidence_ids!: [#Text, ...#Text]
	for i, x in evidence_ids if i > 0 {
		if evidence_ids[i-1] >= x {evidence_ids!: null}
	}
}
#Registration: {
	service!: #Text
	surface_tools!: [...#Text]
	evidence_ids!: [#Text, ...#Text]
	for i, x in surface_tools if i > 0 {
		if surface_tools[i-1] >= x {surface_tools!: null}
	}
	for i, x in evidence_ids if i > 0 {
		if evidence_ids[i-1] >= x {evidence_ids!: null}
	}
}
#HookObservation: {hook_id!: #Text, trigger_evidence_id!: #Text, event_evidence_id!: #Text, beat_evidence_id!: #Text}
#Sample: {
	metric!: #Metric
	value!:  number & >=0
	evidence_ids!: [#Text, ...#Text]
	for i, x in evidence_ids if i > 0 {
		if evidence_ids[i-1] >= x {evidence_ids!: null}
	}
}
#Preflight: {
	session_id!: #Text
	cell_id!:    #Text
	arm!:        #Text
	phase!:      "preflight"
	profile!: #Profile & {frozen_at: <=started_at}
	started_at!:                 #Time & >=profile.frozen_at
	finished_at!:                #Time & >=started_at
	decision_at!:                #Time & >=finished_at & <agent_start_at
	agent_start_at!:             #Time
	startup_environment_digest!: profile.environment_digest
	startup_profile_digest!:     profile.profile_digest
	workload_digest!:            profile.workload_digest
	budget_digest!:              profile.budget_digest
	rungs!: {w0!: #RungResult, w1!: #RungResult, w2!: #RungResult, w3!: #RungResult, w6!: #RungResult}
	served_product_commit?:    #Commit
	installed_product_commit?: #Commit
	index_commit?:             #Commit
	let ReceiptEvidence = evidence
	registration?: #Registration & {
		evidence_ids!: [...#Text]
		for id in evidence_ids {
			if len([for e in ReceiptEvidence if e.id == id && e.source_kind == "registration" {e}]) != 1 {evidence_ids: null}
		}
	}
	primer_digest?: #SHA
	negotiated_tools!: [...#Text]
	answered_probe_ids!: [...#Text]
	hook_observations!: [...#HookObservation]
	absences!: [...#Absence]
	samples!: [...#Sample]
	evidence!: [...#Evidence]
	_ids: [for e in evidence {e.id}]
	for i, e in evidence {
		if i > 0 {if evidence[i-1].id >= e.id {evidence!: null}}
	}

	// Parent aliases keep all identity comparisons outside nested lexical scopes.
	let Session = session_id
	let Product = profile.pins.product_commit
	let Subject = profile.pins.subject_commit
	let Digest = profile.profile_digest
	let HarnessPin = profile.pins.harness_pin
	let Service = profile.expected_service
	let Start = started_at
	let Finish = finished_at
	evidence!: [for e in evidence {#Evidence & {
		phase!:          "preflight"
		session_id!:     Session
		product_commit!: Product
		subject_commit!: Subject
		profile_digest!: Digest
		harness_pin!:    HarnessPin
		observed_at!:    >=Start & <=Finish
		service?:        Service
	}}]
	for name, r in rungs {
		for id in r.evidence_ids {if !(len([for member in _ids if member == id {member}]) > 0) {rungs!: "\(name)": evidence_ids!: null}}
	}
	for i, req in profile.requirements {
		let Key = ["w0", "w1", "w2", "w3", "w6"][i]
		if req.applicable {rungs!: "\(Key)": verdict!: !="NOT-APPLICABLE"}
		if !req.applicable {
			rungs!: "\(Key)": verdict!: "NOT-APPLICABLE"
			for id in rungs[Key].evidence_ids {
				if [for observation in evidence if observation.id == id {observation}][0].source_kind != "channel_inventory" {rungs!: "\(Key)": evidence_ids!: null}
			}
		}
	}
	if rungs.w0.verdict == "PASS" {
		served_product_commit!:    Product
		installed_product_commit!: Product
		if !(len([for member in [for id in rungs.w0.evidence_ids {[for observation in evidence if observation.id == id {observation}][0].source_kind}] if member == "revision" {member}]) > 0) {rungs!: w0!: evidence_ids!: null}
	}
	if !profile.product_present {
		profile: hooks: [...(#HookSpec & {enabled: false})]
		served_product_commit?:    _|_
		installed_product_commit?: _|_
		index_commit?:             _|_
		baseline_checks!: {
			harness_evidence_id!:        #Text
			tools_evidence_id!:          #Text
			startup_evidence_id!:        #Text
			action_capture_evidence_id!: #Text
		}
		if len([for e in evidence if e.id == baseline_checks.harness_evidence_id && e.source_kind == "revision" {e}]) != 1 {baseline_checks: null}
		if len([for e in evidence if e.id == baseline_checks.tools_evidence_id && e.source_kind == "channel_inventory" {e}]) != 1 {baseline_checks: null}
		if len([for e in evidence if e.id == baseline_checks.startup_evidence_id && e.source_kind == "startup_delivery" {e}]) != 1 {baseline_checks: null}
		if len([for e in evidence if e.id == baseline_checks.action_capture_evidence_id && e.source_kind == "action" {e}]) != 1 {baseline_checks: null}
	}
	if !profile.channels[7].enabled {
		registration?: _|_
		negotiated_tools!: []
		answered_probe_ids!: []
	}
	if !profile.channels[3].enabled {primer_digest?: _|_}
	for i, x in negotiated_tools if i > 0 {if negotiated_tools[i-1] >= x {negotiated_tools!: null}}
	for i, x in answered_probe_ids if i > 0 {if answered_probe_ids[i-1] >= x {answered_probe_ids!: null}}
	for id in answered_probe_ids {
		if !(len([for member in _ids if member == id {member}]) > 0) {answered_probe_ids!: null}
		if [for observation in evidence if observation.id == id {observation}][0].source_kind != "call_result" {answered_probe_ids!: null}
	}
	if rungs.w1.verdict == "PASS" {
		let Tools = profile.surface_tools
		registration!: #Registration & {service!: Service, surface_tools!: Tools}
		negotiated_tools!: Tools
		answered_probe_ids!: [#Text, ...#Text]
		if !(len([for member in [for id in rungs.w1.evidence_ids {[for observation in evidence if observation.id == id {observation}][0].source_kind}] if member == "negotiation" {member}]) > 0) {rungs!: w1!: evidence_ids!: null}
		for id in registration.evidence_ids {
			if !(len([for member in _ids if member == id {member}]) > 0) {registration!: evidence_ids!: null}
			if [for observation in evidence if observation.id == id {observation}][0].source_kind != "registration" {registration!: evidence_ids!: null}
			if !(len([for member in rungs.w1.evidence_ids if member == id {member}]) > 0) {rungs!: w1!: evidence_ids!: null}
			evidence!: [for e in evidence {if e.id == id {service!: Service}}]
		}
		for id in answered_probe_ids {
			if !(len([for member in rungs.w1.evidence_ids if member == id {member}]) > 0) {rungs!: w1!: evidence_ids!: null}
			evidence!: [for e in evidence {if e.id == id {service!: Service}}]
		}
		evidence!: [for e in evidence {if (len([for member in rungs.w1.evidence_ids if member == e.id {member}]) > 0) && e.source_kind == "negotiation" {service!: Service}}]
	}
	if rungs.w2.verdict == "PASS" {
		primer_digest!: profile.channels[3].content_digest
		if !(len([for member in [for id in rungs.w2.evidence_ids {[for observation in evidence if observation.id == id {observation}][0].source_kind}] if member == "startup_delivery" {member}]) > 0) {rungs!: w2!: evidence_ids!: null}
		evidence!: [for e in evidence {if (len([for member in rungs.w2.evidence_ids if member == e.id {member}]) > 0) && e.source_kind == "startup_delivery" {service!: Service}}]
	}
	_hookIDs: [for h in profile.hooks if h.enabled {h.id}]
	for i, h in hook_observations {
		if i > 0 {if hook_observations[i-1].hook_id >= h.hook_id {hook_observations!: null}}
		if !(len([for member in _hookIDs if member == h.hook_id {member}]) > 0) {hook_observations!: null}
		for key, kind in {trigger_evidence_id: "hook_trigger", event_evidence_id: "hook_event", beat_evidence_id: "beat"} {
			let ID = h[key]
			if !(len([for member in _ids if member == ID {member}]) > 0) {hook_observations!: null}
			evidence!: [for e in evidence {if e.id == ID {source_kind!: kind}}]
			if rungs.w3.verdict == "PASS" {if !(len([for member in rungs.w3.evidence_ids if member == ID {member}]) > 0) {rungs!: w3!: evidence_ids!: null}}
		}
		let Trigger = [for observation in evidence if observation.id == h.trigger_evidence_id {observation}][0]
		let Event = [for observation in evidence if observation.id == h.event_evidence_id {observation}][0]
		evidence!: [for e in evidence {
			if e.id == h.event_evidence_id {call_id!: Trigger.call_id, observed_at!: >=Trigger.observed_at}
			if e.id == h.beat_evidence_id {call_id!: Trigger.call_id, observed_at!: >=Event.observed_at}
		}]
	}
	if len(_hookIDs) == 0 {hook_observations!: []}
	if rungs.w3.verdict == "PASS" {
		if [for h in hook_observations {h.hook_id}] != _hookIDs {hook_observations!: null}
	}
	_off: [for c in profile.channels if !c.enabled {c.channel}]
	if [for a in absences {a.channel}] != _off {absences!: null}
	for i, a in absences {
		if i > 0 {if absences[i-1].channel >= a.channel {absences!: null}}
		for id in a.evidence_ids {
			if !(len([for member in _ids if member == id {member}]) > 0) {absences!: null}
			if [for observation in evidence if observation.id == id {observation}][0].source_kind != "channel_inventory" {absences!: null}
		}
	}
	for i, s in samples {
		if i > 0 {if samples[i-1].metric >= s.metric {samples!: null}}
		if !(len([for member in [for b in profile.budget.bounds {b.metric}] if member == s.metric {member}]) > 0) {samples!: null}
		for id in s.evidence_ids {
			if !(len([for member in _ids if member == id {member}]) > 0) {samples!: null}
			if !(len([for member in ["resource_sample", "revision"] if member == [for observation in evidence if observation.id == id {observation}][0].source_kind {member}]) > 0) {samples!: null}
			if [for observation in evidence if observation.id == id {observation}][0].source_kind == "revision" && !(len([for member in ["index_age_ns", "payload_age_ns"] if member == s.metric {member}]) > 0) {samples!: null}
		}
	}
	if rungs.w6.verdict == "PASS" {
		if profile.product_present {index_commit!: Subject}
		if [for s in samples {s.metric}] != [for b in profile.budget.bounds {b.metric}] {samples!: null}
		for i, s in samples {
			if s.value > profile.budget.bounds[i].maximum {samples!: null}
			for id in s.evidence_ids {if !(len([for member in rungs.w6.evidence_ids if member == id {member}]) > 0) {rungs!: w6!: evidence_ids!: null}}
		}
	}
}
#Count: {state!: "known", count!: int & >=0} | {state!: "unavailable", reason!: #Text}
#Provenance: {
	state!:                     "validated" | "unvalidated" | "unavailable"
	reason!:                    #Text
	set_digest?:                #SHA
	classifier_digest?:         #SHA
	validation_reading_digest?: #SHA
	evidence_ids!: [...#Text]
	if state == "validated" {set_digest!: #SHA, evidence_ids!: [#Text, ...#Text]}
	for i, x in evidence_ids if i > 0 {if evidence_ids[i-1] >= x {evidence_ids!: null}}
}
#Denominator: {
	eligible_opportunities!: #Count
	delivered_targets!:      #Count
	eligibility!:            #Provenance
	delivery!:               #Provenance
	if eligibility.state == "validated" {eligibility!: {classifier_digest!: #SHA, validation_reading_digest!: #SHA}}
}
#Rate: {
	state!:       "measured"
	numerator!:   int & >=0 & <=denominator
	denominator!: int & >0
} | {
	state!:       "not_measured"
	denominator!: #Count
	cause!:       "zero_denominator" | "unavailable" | "unvalidated"
	reason!:      #Text
}
#CapabilityOutcome: {
	capability!:       #Text
	eligible!:         #Count
	successful_calls!: #Count
	acted!:            #Count
	should_not_calls!: #Count
	unknown!:          #Count
	denominator!:      #Denominator
	eligible!:         Margins.eligible_opportunities
	action_capture!:   #Provenance
	w4!:               #Rate
	w5!:               #Rate
	bars!: {use_minimum!: number & >=0 & <=1, acted_minimum!: number & >=0 & <=1}
	if action_capture.state == "unavailable" {acted!: state!: "unavailable"}
	let Margins = denominator

	// Rates are a deterministic function of declared counts and provenance states.
	if Margins.eligible_opportunities.state == "unavailable" {w4: {state!: "not_measured", denominator!: Margins.eligible_opportunities, cause!: "unavailable"}}
	if Margins.eligible_opportunities.state == "known" {
		if Margins.eligible_opportunities.count == 0 {w4: {state!: "not_measured", denominator!: Margins.eligible_opportunities, cause!: "zero_denominator"}}
		if Margins.eligible_opportunities.count > 0 {
			if successful_calls.state == "unavailable" || (len([for member in [Margins.eligibility.state] if member == "unavailable" {member}]) > 0) {
				w4: {state!: "not_measured", denominator!: Margins.eligible_opportunities, cause!: "unavailable"}
			}
			if successful_calls.state == "known" && !(len([for member in [Margins.eligibility.state] if member == "unavailable" {member}]) > 0) {
				if (len([for member in [Margins.eligibility.state] if member == "unvalidated" {member}]) > 0) {
					w4: {state!: "not_measured", denominator!: Margins.eligible_opportunities, cause!: "unvalidated"}
				}
				if !(len([for member in [Margins.eligibility.state] if member == "unvalidated" {member}]) > 0) {
					w4: {state!: "measured", denominator!: Margins.eligible_opportunities.count}
				}
			}
		}
	}
	if Margins.delivered_targets.state == "unavailable" {w5: {state!: "not_measured", denominator!: Margins.delivered_targets, cause!: "unavailable"}}
	if Margins.delivered_targets.state == "known" {
		if Margins.delivered_targets.count == 0 {w5: {state!: "not_measured", denominator!: Margins.delivered_targets, cause!: "zero_denominator"}}
		if Margins.delivered_targets.count > 0 {
			if acted.state == "unavailable" || (len([for member in [Margins.delivery.state, action_capture.state] if member == "unavailable" {member}]) > 0) {
				w5: {state!: "not_measured", denominator!: Margins.delivered_targets, cause!: "unavailable"}
			}
			if acted.state == "known" && !(len([for member in [Margins.delivery.state, action_capture.state] if member == "unavailable" {member}]) > 0) {
				if (len([for member in [Margins.delivery.state, action_capture.state] if member == "unvalidated" {member}]) > 0) {
					w5: {state!: "not_measured", denominator!: Margins.delivered_targets, cause!: "unvalidated"}
				}
				if !(len([for member in [Margins.delivery.state, action_capture.state] if member == "unvalidated" {member}]) > 0) {
					w5: {state!: "measured", denominator!: Margins.delivered_targets.count}
				}
			}
		}
	}
	if w4.state == "measured" {w4!: numerator!: <=successful_calls.count}
	if w5.state == "measured" {w5!: numerator!: acted.count}
}
#Outcome: {
	session_id!:                 #Text
	cell_id!:                    #Text
	arm!:                        #Text
	phase!:                      "agent"
	pins!:                       #Pins
	profile_digest!:             #SHA
	environment_digest!:         #SHA
	agent_start_at!:             #Time
	delivery_parameters_digest!: #SHA
	k!:                          int & >0
	k_frozen_at!:                #Time & <agent_start_at
	capabilities!: [...#CapabilityOutcome]
	evidence!: [...#Evidence]
	let Session = session_id
	let Product = pins.product_commit
	let Subject = pins.subject_commit
	let Digest = profile_digest
	let HarnessPin = pins.harness_pin
	let Start = agent_start_at
	evidence!: [for e in evidence {#Evidence & {
		phase!:          "agent", session_id!: Session, product_commit!: Product, subject_commit!: Subject
		profile_digest!: Digest, harness_pin!: HarnessPin, observed_at!: >=Start
	}}]
	_ids: [for e in evidence {e.id}]
	for i, e in evidence {
		if i > 0 {if evidence[i-1].id >= e.id {evidence!: null}}
		if e.source_kind == "action_capture" {
			for id in e.capture.action_evidence_ids {
				if !(len([for member in _ids if member == id {member}]) > 0) {evidence!: null}
				if [for observation in evidence if observation.id == id {observation}][0].source_kind != "action" {evidence!: null}
			}
		}
	}
	for i, c in capabilities {
		if i > 0 {if capabilities[i-1].capability >= c.capability {capabilities!: null}}
		for p in [
			{value: c.denominator.eligibility, kind: "eligibility"},
			{value: c.denominator.delivery, kind: "call_result"},
			{value: c.action_capture, kind: "action_capture"},
		] {
			for id in p.value.evidence_ids {
				if !(len([for member in _ids if member == id {member}]) > 0) {capabilities!: null}
				if [for observation in evidence if observation.id == id {observation}][0].source_kind != p.kind {capabilities!: null}
			}
		}
		if c.action_capture.state == "validated" {
			if len(c.action_capture.evidence_ids) != 1 {capabilities!: null}
			let ID = c.action_capture.evidence_ids[0]
			evidence!: [for e in evidence {if e.id == ID {capture!: {capability!: c.capability, complete!: true}}}]
			if len([for observation in evidence if observation.id == ID {observation}][0].capture.action_evidence_ids) == 0 {
				capabilities!: [for x in capabilities {if x.capability == c.capability {acted!: {state!: "known", count!: 0}}}]
			}
		}
	}
}
#Conformance: {
	preflight!: #Preflight
	let P = preflight
	outcome?: #Outcome & {
		session_id!:         P.session_id
		cell_id!:            P.cell_id
		arm!:                P.arm
		pins!:               P.profile.pins
		agent_start_at!:     P.agent_start_at
		profile_digest!:     P.profile.profile_digest
		environment_digest!: P.profile.environment_digest
		delivery_parameters_digest!: [for s in P.profile.source_digests if s.id == "delivery_parameters" {s.digest}][0]
	}
}
