package readouts

// A well-formed rule file (docs/design/readouts.md §7.1's worked example):
// a diagnostic rule in observe with a repeat recovery, a kept-error fixture
// and a trip fixture.
makeChatter: #File & {
	schema: 1
	rule: [{
		id:       "make-chatter"
		version:  1
		about:    "make: drop Entering/Leaving directory lines"
		match: argv0: ["make", "gmake"]
		mode:     "observe"
		recovery: "repeat"
		stage: [{kind: "drop", pattern: "^make(\\[\\d+\\])?: (Entering|Leaving) directory "}]
		test: [{
			input:  "make[1]: Entering directory '/src/a'\ncc -c a.c\n"
			expect: "cc -c a.c\n[readout make-chatter@1 observe: 2→1 lines; dropped 1 (plain 1); raw: repeat the command]\n"
		}, {
			input: "make[1]: Entering directory '/src/error'\n"
			trip:  true
		}]
	}]
}

// A content rule with every match field, a floor, a join and a fixture file.
lsLong: #File & {
	schema: 1
	rule: [{
		id:      "ls-long"
		version: 2
		about:   "ls -l: names with a type marker"
		match: {
			argv0:      ["ls"]
			flags_any:  ["-l", "-la"]
			flags_none: ["--color=always"]
			compound:   "display_tail"
		}
		when: bytes_gt: 1500
		recovery: "repeat"
		stage: [{
			kind:     "fields"
			pattern:  "^([dl-])\\S+\\s+\\S+\\s+\\S+\\s+\\S+\\s+\\S+\\s+\\S+\\s+\\S+\\s+\\S+\\s+(.+)$"
			captures: ["type", "name"]
			template: "{name}{type:d=/,l=@}"
		}, {kind: "join", join: "head_stamp"}]
		test: [{input_file: "ls-long.txt", expect: "a/\n", stream: "content"}]
	}]
}

// A diff rule with a capping hunks stage and its floor, counts from extract
// templates, and a test rule that also accepts the declared test commands.
gitDiff: #File & {
	schema: 1
	rule: [{
		id:       "git-diff"
		version:  1
		about:    "git diff: every hunk header kept, a long body capped"
		match: {argv0: ["git"], sub: ["diff"], flags_none: ["--stat"], compound: "display_tail"}
		recovery: "repeat"
		stage: [{
			kind: "extract", pattern: "^@@ ", record: "hunk", template: "{count} hunks"
		}, {
			kind: "hunks", above: 60, head: 20, tail: 10, template: "… {n} lines dropped: sed -n {from},{to}p {file}"
		}, {kind: "join", join: "head_stamp"}]
		test: [{input: "diff --git a/x b/x\n", expect: "diff --git a/x b/x\n"}]
	}, {
		id:      "go-test"
		version: 1
		about:   "go test and the declared test commands"
		match: {argv0: ["go"], sub: ["test"], declared_test: true}
		recovery: "saved"
		stage: [{kind: "summarize", pattern: "^ok\\s+", template: "ok: {count} packages"}]
		test: [{input: "ok  \texample.com/a\t0.1s\n", trip: true}]
	}]
}
