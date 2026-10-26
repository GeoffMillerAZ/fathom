// Command-local input contract. Go enforces semantic checks and strict JSON decoding.
package commandlab

import "list"

_id: string & =~"^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$"
_model: string & =~"^[A-Za-z0-9][A-Za-z0-9._:/+@-]{0,255}$"
_kind: "read-range" | "search-literal" | "git-status" | "git-diff" | "run-check" | "write-file" | "shell"
#Source: close({id: _id, format: "codex" | "claude" | "normalized", path: string & !=""})
#Manifest: close({version: "command-lab.manifest/1", expectedSourceIds: [..._id] & list.MinItems(1) & list.MaxItems(32) & list.UniqueItems(), sources: [...#Source] & list.MaxItems(32)})
#Scope: close({version: "command-lab.scope/1", requiredLayerIds: [..._id] & list.MinItems(1) & list.MaxItems(32) & list.UniqueItems()})
#Layer: close({id: _id, scope: "organization" | "team" | "global" | "project" | "personal", allowedOperations: [..._kind] & list.MinItems(1) & list.UniqueItems(), maxBytes: int & >0, maxResults: int & >0})
#Classifier: close({provider: _id, resolvedModel: _model})
#Policy: close({version: "command-lab.policy/1", layers: [...#Layer] & list.MinItems(1) & list.MaxItems(32), classifier?: #Classifier})
#Request: close({version: "command-lab.request/1", id: _id, kind: _kind, path?: string, command?: string, maxBytes: int & >0, maxResults: int & >0})
#PortAnswer: close({ID: _id, Kind: "choice", Choice: "concern" | "no-concern" | "unknown", Probabilities: {concern: number & >=0 & <=1, "no-concern": number & >=0 & <=1, unknown: number & >=0 & <=1}, Score?: number, Noul?: number, Confidence: number & >=0 & <=1})
#PortResponse: close({Provider: _id, Model: _model, Answers: [_id]: #PortAnswer, Invalid: [_id]: string, Declined?: [_id]: string, Usage: close({InputTokens: int & >=0, OutputTokens: int & >=0}), Latency: int & >=0})
#Capture: close({version: "command-lab.capture/1", requestKey: string & =~"^sha256:[0-9a-f]{64}$", response?: #PortResponse, cause?: "no-provider" | "timeout" | "transport-error" | "refused"})
