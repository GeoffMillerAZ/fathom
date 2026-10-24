// Shipped command guards. Predicates are private to this embedded pack;
// repository guards continue to use the existing guard schema.
guards: [{
	command_scopes: ["git *"]
	command_check: "git_no_verify"
	enforce: true
	note: "Run git push with hooks enabled; run git commit with hooks enabled too; remove --no-verify (or commit -n) and fix any failing hook."
}, {
	command_scopes: ["git *"]
	command_check: "push_lefthook_off"
	enforce: true
	note: "Remove LEFTHOOK=0 (also false, no or off, in any case) and run git push or commit with hooks enabled."
}, {
	command_scopes: ["git *"]
	command_check: "git_hooks_path"
	enforce: true
	note: "Remove -c core.hooksPath and run git push or commit with the repository's hooks enabled."
}, {
	command_scopes: ["git *"]
	command_check: "push_force_protected"
	enforce: true
	note: "Push main/master without force; do not delete these remote branches, and push rewritten history to a feature branch instead."
}]
