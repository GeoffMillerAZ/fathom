// Package worktree is a fauxtown fixture whose service root contains a stale
// .claude/ agent-worktree directory. Connectors must discover only this file
// and never the duplicate copy under .claude/worktrees/.
package worktree

// Version is the canonical fixture symbol.
const Version = "1.0.0"
