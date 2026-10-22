package main

import (
	"reflect"
	"testing"
)

func TestBashWriteTargetsAD8(t *testing.T) {
	for _, tt := range []struct {
		name, command string
		want          []string
	}{
		{"sed expressions", `sed -i -e 's/p/q/' -e 's/m/n/' docs/plans/x.md`, []string{"docs/plans/x.md"}},
		{"sed script file", `sed -i -f rules.sed docs/a.md`, []string{"docs/a.md"}},
		{"sed long expression", `sed -i --expression 's/p/q/' docs/a.md`, []string{"docs/a.md"}},
		{"sed attached expressions", `sed -i.bak -es/p/q/ --expression=s/m/n/ docs/a.md`, []string{"docs/a.md"}},
		{"sed BSD", `sed -i '' -e 's/p/q/' docs/a.md`, []string{"docs/a.md"}},
		{"sed suffix", `sed -i.bak 's/p/q/' docs/a.md docs/b.md`, []string{"docs/a.md", "docs/b.md"}},
		{"sed clustered E then i", `sed -Ei 's/p/q/' docs/a.md`, []string{"docs/a.md"}},
		{"sed clustered r then i", `sed -ri 's/p/q/' docs/a.md`, []string{"docs/a.md"}},
		{"docker foreign", `docker cp docs/a.md container:/tmp/a.md`, nil},
		{"docker local", `docker cp container:/tmp/a.md docs/a.md`, []string{"docs/a.md"}},
		{"kubectl foreign", `kubectl cp docs/a.md pod:/tmp/a.md`, nil},
		{"scp foreign", `scp docs/a.md remote:/tmp/a.md`, nil},
		{"scp local", `scp remote:/tmp/a.md docs/a.md`, []string{"docs/a.md"}},
		{"perl", `perl -i -pe 's/p/q/' docs/a.md docs/b.md`, []string{"docs/a.md", "docs/b.md"}},
		{"perl pi", `perl -pi.bak -e 's/p/q/' docs/a.md`, []string{"docs/a.md"}},
		{"perl zero cluster", `perl -0pi -e 's/a/b/' docs/a.md`, []string{"docs/a.md"}},
		{"perl line cluster", `perl -lpi -e 's/a/b/' docs/a.md`, []string{"docs/a.md"}},
		{"perl separate line", `perl -l -pi -e 's/a/b/' docs/a.md`, []string{"docs/a.md"}},
		{"perl separate zero", `perl -0 -pi -e 's/a/b/' docs/a.md`, []string{"docs/a.md"}},
		{"perl octal cluster", `perl -0777pi -e 's/a/b/' docs/a.md`, []string{"docs/a.md"}},
		{"perl line digits cluster", `perl -l012pi -e 's/a/b/' docs/a.md`, []string{"docs/a.md"}},
		{"ruby separate warnings", `ruby -W -pi -e 'gsub(/a/, "b")' docs/a.md`, []string{"docs/a.md"}},
		{"ruby warning cluster", `ruby -W2pi -e 'gsub(/a/, "b")' docs/a.md`, []string{"docs/a.md"}},
		{"ruby zero cluster", `ruby -0pi -e 'gsub(/a/, "b")' docs/a.md`, []string{"docs/a.md"}},
		{"ruby strip cluster", `ruby -xpi -e 'gsub(/a/, "b")' docs/a.md`, []string{"docs/a.md"}},
		{"perl next include", `perl -I lib -pi -e 's/a/b/' docs/a.md`, []string{"docs/a.md"}},
		{"ruby next include", `ruby -I lib -pi -e 'gsub(/a/, "b")' docs/a.md`, []string{"docs/a.md"}},
		{"ruby next require", `ruby -r json -pi -e 'gsub(/a/, "b")' docs/a.md`, []string{"docs/a.md"}},
		{"perl include directory", `perl -Ilib -e 'print' data.txt`, nil},
		{"perl module and inline expression", `perl -Mstrict -ne 'print' data.txt`, nil},
		{"ruby", `ruby -i -pe 'gsub(/p/, "q")' docs/a.md`, []string{"docs/a.md"}},
		{"ruby include directory", `ruby -Ilib script.rb data.txt`, nil},
		{"nested quotes", `echo "$(grep ">" "$F" | wc -l)"`, nil},
		{"nested quotes with write", `echo "$(printf "%s" "$(grep ">" "$F")")" > docs/a.md`, []string{"docs/a.md"}},
		{"perl attached value hides i", `perl -Cpi -e 'print' data.txt`, nil},
		{"ruby attached value hides i", `ruby -Epi -e 'print' data.txt`, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := bashWriteTargets(tt.command)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("targets = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAD8InterpreterAttachedOnlyFlags(t *testing.T) {
	for _, tt := range []struct{ verb, flags string }{
		{"perl", "CdDFxMm"},
		{"ruby", "CEFK"},
	} {
		for _, flag := range tt.flags {
			for _, suffix := range []string{"", "value"} {
				command := tt.verb + " -" + string(flag) + suffix + " -pi -e 'expression' docs/a.md"
				t.Run(command, func(t *testing.T) {
					got, _ := bashWriteTargets(command)
					if !reflect.DeepEqual(got, []string{"docs/a.md"}) {
						t.Errorf("targets = %q, want docs/a.md", got)
					}
				})
			}
		}
	}
}

func TestAD8ForeignCopiesStaySilentAndLocalColonsRemainTargets(t *testing.T) {
	for _, tt := range []struct {
		command string
		want    []string
		written bool
	}{
		{`docker cp docs/a.md container:/tmp/a.md`, nil, false},
		{`kubectl cp docs/a.md pod:/tmp/a.md`, nil, false},
		{`scp docs/a.md remote:/tmp/a.md`, nil, false},
		{`cp docs/a.md notes:copy`, []string{"notes:copy"}, true},
		{`echo text > notes:copy`, []string{"notes:copy"}, true},
	} {
		got, written := bashWriteTargets(tt.command)
		if !reflect.DeepEqual(got, tt.want) || written != tt.written {
			t.Errorf("%s: targets=%q written=%v, want %q %v", tt.command, got, written, tt.want, tt.written)
		}
	}
}
