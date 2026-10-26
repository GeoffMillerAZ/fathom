// Wire shape only. Go verifies signatures, exact rosters, path identity, time,
// intersection, and safe output. Closed records reject unknown fields.
package harnessctl

import (
 "list"
 "strings"
 "time"
)

_id: string & =~"^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$"
_hash: string & =~"^[0-9a-f]{64}$"
_time: time.Time & =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$"
_key64: string & =~"^[A-Za-z0-9+/]{42}[AEIMQUYcgkosw048]=$"
_sig64: string & =~"^([A-Za-z0-9+/]{85}[AQgw]|[A-Za-z0-9_-]{85}[AQgw])(==)?$"
// Shape and encoded-length ceiling only; Go decodes and checks canonical bytes.
_payload64: string & strings.MaxRunes(43692) & =~"^(([A-Za-z0-9+/]{4})*([A-Za-z0-9+/][AQgw](==)?|[A-Za-z0-9+/]{2}[AEIMQUYcgkosw048]=?)?|([A-Za-z0-9_-]{4})*([A-Za-z0-9_-][AQgw](==)?|[A-Za-z0-9_-]{2}[AEIMQUYcgkosw048]=?)?)$"
// Lexical cross-platform shape; Go's host filepath and canonical checks decide
// whether the path is an actual protected absolute path on that host.
_abs: string & strings.MaxRunes(4096) & (=~"^/" | =~"^[A-Za-z]:[\\\\/]" | =~"^\\\\\\\\[^\\\\/]+[\\\\/][^\\\\/]+")
_ppm: int & >=0 & <=1000000
_work: int & >=1 & <=30000
_ids0_16: [..._id] & list.MaxItems(16) & list.UniqueItems()
_ids0_32: [..._id] & list.MaxItems(32) & list.UniqueItems()
_ids1_64: [..._id] & list.MinItems(1) & list.MaxItems(64) & list.UniqueItems()

#Key: close({id: _id, publicKey: _key64, revoked: bool})
_controlBase: {id: _id, event: "before-tool" | "after-tool" | "session-start" | "session-stop" | "pre-commit" | "pre-push", minSamplePPM: _ppm, maxSamplePPM: _ppm & >=minSamplePPM, defaultSamplePPM: _ppm & >=minSamplePPM & <=maxSamplePPM, maxWorkMillis: _work, onFault: "hold" | "warn", allowMaintenance: bool, tunableBy: _ids0_32}
// Explicit branches keep the closed shape concrete under CUE 0.17; a
// conditional inside close leaves its class disjunction unresolved there.
#Control: close(_controlBase & ({class: "mandatory", minSamplePPM: 1000000, maxSamplePPM: 1000000, defaultSamplePPM: 1000000, onFault: "hold", allowMaintenance: false, tunableBy: []} | {class: "advisory"}))
#LayerRef: close({id: _id, kind: "installation" | "organization" | "team" | "project" | "personal", targetId: _id, path: _abs, keyId: _id, revision: int & >=1 & <=2147483647, payloadSHA256: _hash, maxAgeSeconds: int & >=1 & <=2592000, priority: int & >=0 & <=31, controlIds: _ids1_64})
#Project: close({id: _id, root: _abs, organizationId?: _id, teamIds: _ids0_16, principalId: _id, controlIds: _ids1_64, layers: [...#LayerRef] & list.MinItems(1) & list.MaxItems(32)})
#Registration: close({version: "harnessctl.registration/1", epoch: int & >=1 & <=2147483647, issuedAt: _time, expiresAt: _time, hostId: _id, keys: [...#Key] & list.MinItems(1) & list.MaxItems(512), controls: [...#Control] & list.MinItems(1) & list.MaxItems(256), projects: [...#Project] & list.MinItems(1) & list.MaxItems(256)})
#Signature: close({sig: _sig64, keyid?: string & strings.MaxRunes(128) & =~"^[ -~]*$"})
#Envelope: close({payloadType: "application/vnd.agentic-kit.harness-layer.v1+json", payload: _payload64, signatures: [#Signature]})
#Constraint: close({id: _id, minSamplePPM: _ppm, maxSamplePPM: _ppm, maxWorkMillis: _work, onFault: "hold" | "warn", allowMaintenance: bool, tunableBy: _ids0_32})
#Preference: close({controlId: _id, samplePPM: _ppm})
#Layer: close({version: "harnessctl.layer/1", id: _id, kind: "installation" | "organization" | "team" | "project" | "personal", targetId: _id, revision: int & >=1 & <=2147483647, issuedAt: _time, expiresAt: _time, controls: [...#Constraint] & list.MinItems(1) & list.MaxItems(64), preferences: [...#Preference] & list.MaxItems(64)})

#SourceReport: close({id: _id, kind: "installation" | "organization" | "team" | "project" | "personal", priority: int & >=0 & <=31, revision: int & >=1 & <=2147483647})
#Effective: close({id: _id, event: "before-tool" | "after-tool" | "session-start" | "session-stop" | "pre-commit" | "pre-push", class: "mandatory" | "advisory", minSamplePPM: _ppm, maxSamplePPM: _ppm, samplePPM: _ppm, maxWorkMillis: _work, onFault: "hold" | "warn", allowMaintenance: bool, tunableBy: _ids0_32, constrainedBy: [..._id] & list.MinItems(1) & list.MaxItems(33), tunedBy?: _id})
_baseReport: {version: "harnessctl.report/1", noActionAuthority: true, coverage: "selected-project-only", installation: "unobserved"}
_failureBase: {_baseReport, status: "hold", expectedLayers: int & >=0 & <=32, loadedLayers: int & >=0 & <=expectedLayers, authenticatedLayers: int & >=0 & <=loadedLayers, sourceOrdinal?: int & >=0 & <expectedLayers}
#SourceFailure: close({_failureBase, category: "source", code: "input_invalid" | "registration_invalid" | "project_unregistered" | "clock_invalid" | "source_unavailable" | "source_changed" | "envelope_invalid" | "signature_invalid" | "binding_mismatch"})
#PolicyFailure: close({_failureBase, category: "policy", code: "policy_conflict" | "preference_denied"})
#Failure: #SourceFailure | #PolicyFailure
#Check: close({_baseReport, status: "authenticated", registrationEpoch: int & >=1 & <=2147483647, registrationSHA256: _hash, snapshotSHA256: _hash, issuedAt: _time, expiresAt: _time, layerCount: int & >=1 & <=32, controlCount: int & >=1 & <=64})
#Explain: close({#Check, projectId: _id, sources: [...#SourceReport] & list.MinItems(1) & list.MaxItems(32), effectiveControls: [...#Effective] & list.MinItems(1) & list.MaxItems(64)})
#Report: #Failure | #Check | #Explain
