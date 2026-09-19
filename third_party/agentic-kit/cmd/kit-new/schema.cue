package kitnew

import "list"

// Runtime mirrors this contract with bounded stdlib validation; CUE remains
// the authoring/interchange contract, not an embedded runtime dependency.
#Host: "claude" | "codex"
#Digest: string & =~"^[a-f0-9]{64}$"
#ControlName: "configuration" | "runtime" | "startup" | "resume" |
	"nested-instructions" | "tool" | "edit" | "shell" | "stdin" |
	"hosted-tools" | "compaction" | "stop" | "hook-failure-closed"
#Status: "supported" | "unsupported" | "unknown"
#Strings: [...string & !=""] & list.MaxItems(128) & list.UniqueItems()
#Policy: {
	version: 1
	deny_tools?: #Strings | null
	// Runtime additionally requires clean project-relative paths and valid globs.
	deny_paths?: (#Strings & [...string & =~"^[\\x20-\\x7e]+$"]) | null
	required_controls?: ([...#ControlName] & list.MaxItems(128) & list.UniqueItems()) | null
	instructions?: string | null
}
#FileChange: {
	path: string
	before: string
	before_exists: bool
	after: string
	after_exists: bool
}
#Plan: {
	version: 1
	host: #Host
	root: string
	operation: "install" | "uninstall"
	changes: [...#FileChange] | null
	digest: #Digest
}
#Control: {
	name: #ControlName
	status: #Status
	basis: string
	detail: string
}
#Receipt: {
	version: 1
	host: #Host
	host_version: string
	generated_at: string
	config_digest: #Digest | ""
	help_digest: #Digest
	controls: [...#Control]
	ready: bool
	scope: string
}
#Evidence: {
	version: 1
	host: #Host
	host_version: string
	config_digest: #Digest
	observations: [...{
		control: #ControlName & !="configuration" & !="runtime"
		status: #Status
		method: "live-host"
		path: string
		digest: #Digest
		detail: string & !=""
	}] & list.MaxItems(14)
}
