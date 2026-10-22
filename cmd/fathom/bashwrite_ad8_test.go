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
		{"perl include directory", `perl -Ilib -e 'print' data.txt`, nil},
		{"perl module and inline expression", `perl -Mstrict -ne 'print' data.txt`, nil},
		{"ruby", `ruby -i -pe 'gsub(/p/, "q")' docs/a.md`, []string{"docs/a.md"}},
		{"ruby include directory", `ruby -Ilib script.rb data.txt`, nil},
		{"nested quotes", `echo "$(grep ">" "$F" | wc -l)"`, nil},
		{"nested quotes with write", `echo "$(printf "%s" "$(grep ">" "$F")")" > docs/a.md`, []string{"docs/a.md"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := bashWriteTargets(tt.command)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("targets = %q, want %q", got, tt.want)
			}
		})
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
