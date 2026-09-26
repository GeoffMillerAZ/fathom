// The taskfile pack (CC-12, vision 22 §7): the Taskfile's own links. Enable
// it in .fathom/concerns.cue with `packs: ["taskfile"]`; disable or
// replace an entry by id (law 13).
pack: {
	name:    "taskfile"
	version: 1
	about:   "the Taskfile's own links: every task a task calls, every local include, and every repo script a task runs exists"
}

concerns: {
	taskfile_commands_resolve: {
		want:  "every task a Taskfile calls (a `task:` entry, deps, a shell `task X`), every local include, and every repo script a task runs exists"
		why:   "the Taskfile → scripts link of the gate chain: a target that calls a moved script fails only when someone runs it — usually the gate, at push time"
		check: {kind: "commands_resolve", where: "taskfile"}
	}
}
