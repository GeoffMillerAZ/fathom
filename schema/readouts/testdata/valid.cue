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
