// Package readouts defines the CUE schema for a readout rule file
// (docs/design/readouts.md §7, RO-7): TOML with `schema = 1` and one or
// more `[[rule]]` tables, decoded and unified with #File by
// internal/readout.LoadRules, which embeds a byte-identical copy
// (internal/readout/readouts_schema.cue, held to this file by
// TestReadoutsSchemaMatchesSource).
//
// Every definition is closed, so an unknown key is refused. No field names
// an executable, an argv, shell text or a URL: match fields are bare
// command, subcommand and flag tokens the lexed command is compared with,
// joins come from the closed built-in table, and free text refuses "://".
// What CUE cannot say (a stage kind's required parameters, exactly one of
// input and input_file, a lossy rule's recovery and protected-line
// fixture) internal/readout checks in Go and refuses at load.
package readouts

#File: {
	schema: 1
	rule: [#Rule, ...#Rule]
}

#Rule: {
	id:        #ID
	version:   int & >=1
	about:     #Text
	match:     #Match
	when?:     #When
	mode?:     "off" | "observe" | "lean" | "rich"
	recovery?: "repeat" | "span" | "persisted" | "saved"
	stage: [#Stage, ...#Stage]
	test: [#Test, ...#Test]
}

#ID:   =~"^[a-z][a-z0-9-]{0,62}$"
#Name: =~"^[a-z_][a-z0-9_]{0,62}$"
#Text: string & !="" & !~"://"

// #Token is a bare command or subcommand name: never a path, an argv or
// shell text.
#Token: =~"^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$"

// #Flag is one flag word as the lexed command carries it.
#Flag: =~"^--?[A-Za-z0-9][A-Za-z0-9_.=+-]{0,63}$"

// #FileName is a fixture file beside the rule file: a bare name.
#FileName: =~"^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$" & !~"\\.\\."

#Match: {
	argv0: [#Token, ...#Token]
	sub?: [...#Token]
	flags_any?: [...#Flag]
	flags_none?: [...#Flag]
	compound?: "simple" | "display_tail" | "after_quiet"
	// declared_test also accepts a command the test-run matcher (TR-1)
	// says runs a declared test command: no list of commands here.
	declared_test?: bool
	// whole_read accepts only a read that prints one source file whole
	// (`cat <file>`, `sed -n 1,$p <file>`): never a range or several files.
	whole_read?: bool
}

#When: {
	bytes_gt?: int & >=0
	lines_gt?: int & >=0
}

#Stage: {
	kind:       "ansi" | "collapse" | "group" | "extract" | "join" | "fields" | "keep" | "drop" | "cap" | "hunks" | "summarize" | "outline"
	pattern?:   string & !=""
	captures?:  [...#Name]
	key?:       string & !=""
	keep?:      [...#Name]
	head?:      int & >=0
	tail?:      int & >=0
	per_group?: int & >=1
	above?:     int & >=1
	template?:  string & !~"://"
	record?:    #Name
	join?:      #Join
}

// #Join is the closed built-in join table (design §7).
#Join: "outline" | "head_stamp" | "last_failures" | "changed_files" | "edited_in_session" | "guard_for_command"

#Test: {
	input?:      string
	input_file?: #FileName
	expect?:     string
	stream?:     "content" | "diagnostic"
	trip?:       bool
}
