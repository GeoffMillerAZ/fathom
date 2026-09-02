// invalid.cue — negative fixtures for the Agent Skills frontmatter schema,
// documented commented-out so live `cue vet` stays green (repo convention).
// Each case is asserted to actually fail by
// cmd/fathom/skillsschema_test.go (TestSkillsSchema_Negatives).
package skills

// 1. an uppercase name:
// bad1: #Frontmatter & {name: "PDF-Processing", description: "d"}

// 2. a leading hyphen:
// bad2: #Frontmatter & {name: "-pdf", description: "d"}

// 3. a double hyphen:
// bad3: #Frontmatter & {name: "pdf--processing", description: "d"}

// 4. an empty description:
// bad4: #Frontmatter & {name: "pdf", description: ""}

// 5. a Claude Code extension key (closed struct):
// bad5: #Frontmatter & {name: "pdf", description: "d", paths: "cmd/**"}

// 6. non-string metadata values:
// bad6: #Frontmatter & {name: "pdf", description: "d", metadata: {version: 1}}
