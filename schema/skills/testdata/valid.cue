// valid.cue — positive fixtures for the Agent Skills frontmatter schema.
// This file must pass `cue vet ./...` from schema/skills.
package skills

minimal: #Frontmatter & {
	name:        "skill-name"
	description: "A description of what this skill does and when to use it."
}

full: #Frontmatter & {
	name:          "pdf-processing"
	description:   "Extract PDF text, fill forms, merge files. Use when handling PDFs."
	license:       "Apache-2.0"
	compatibility: "Requires poppler-utils on PATH."
	metadata: {author: "example-org", version: "1.0"}
	"allowed-tools": "Bash(git:*) Bash(jq:*) Read"
}
