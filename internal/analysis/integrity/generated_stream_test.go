package integrity

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestGeneratedHandEditsStreamingSkipsUnmarkedBodies(t *testing.T) {
	var paths []string
	for i := 0; i < 1000; i++ {
		paths = append(paths, fmt.Sprintf("assets/image-%04d.png", i))
	}
	reads := 0
	findings, missing := GeneratedHandEdits(GeneratedInput{
		ChangedFiles:  paths,
		HeadGenerated: func(string) (bool, bool, error) { return false, true, nil },
		HeadFile: func(string) ([]byte, bool, error) {
			reads++
			return nil, false, errors.New("unexpected full read")
		},
	})
	if len(findings) != 0 || len(missing) != 0 || reads != 0 {
		t.Fatalf("findings=%v missing=%v full reads=%d", findings, missing, reads)
	}
}

func TestGeneratedHandEditsStreamingReadsMarkedBodyAndReportsErrors(t *testing.T) {
	const output = "api/thing.pb.go"
	base := map[string][]byte{output: []byte(goGenHeader + "package api\nvar X = 1\n"), "api/thing.proto": []byte("message Thing {}\n")}
	head := map[string][]byte{output: []byte(goGenHeader + "package api\nvar X = 2\n"), "api/thing.proto": base["api/thing.proto"]}
	fullReads := 0
	in := GeneratedInput{
		ChangedFiles:  []string{output},
		HeadGenerated: func(string) (bool, bool, error) { return true, true, nil },
		BaseFile:      fileReader(base),
		HeadFile: func(p string) ([]byte, bool, error) {
			fullReads++
			return fileReader(head)(p)
		},
		SiblingFiles: map[string][]string{"api": {output, "api/thing.proto"}},
	}
	findings, missing := GeneratedHandEdits(in)
	if len(findings) != 1 || findings[0].Key != output || len(missing) != 0 || fullReads != 2 {
		t.Fatalf("findings=%v missing=%v full reads=%d", findings, missing, fullReads)
	}
	in.HeadGenerated = func(string) (bool, bool, error) { return false, false, errors.New("header read failed") }
	findings, missing = GeneratedHandEdits(in)
	if len(findings) != 0 || len(missing) != 1 || !strings.Contains(missing[0], "header read failed") || fullReads != 2 {
		t.Fatalf("header failure: findings=%v missing=%v full reads=%d", findings, missing, fullReads)
	}
	in.HeadGenerated = func(string) (bool, bool, error) { return true, true, nil }
	in.HeadFile = func(string) ([]byte, bool, error) { return nil, false, errors.New("body read failed") }
	findings, missing = GeneratedHandEdits(in)
	if len(findings) != 0 || len(missing) != 1 || !strings.Contains(missing[0], "body read failed") {
		t.Fatalf("body failure: findings=%v missing=%v", findings, missing)
	}
	in.HeadFile = fileReader(head)
	in.BaseFile = func(p string) ([]byte, bool, error) {
		if p == "api/thing.proto" {
			return nil, false, errors.New("source read failed")
		}
		return fileReader(base)(p)
	}
	findings, missing = GeneratedHandEdits(in)
	if len(findings) != 0 || len(missing) != 1 || !strings.Contains(missing[0], "source read failed") {
		t.Fatalf("source failure: findings=%v missing=%v", findings, missing)
	}
}

func TestGeneratedHandEditsDeclaredOutputUsesFullBody(t *testing.T) {
	root := t.TempDir()
	writeDeclFile(t, root, ".fathom/pack.cue", "generated: [{output: \"ui/app.css\", source: [\"ui/input.css\"], regenerate: \"task ui:css\"}]\n")
	base := map[string][]byte{"ui/app.css": []byte(".a{}\n"), "ui/input.css": []byte("a\n")}
	head := map[string][]byte{"ui/app.css": []byte(".a{color:red}\n"), "ui/input.css": []byte("a\n")}
	in := GeneratedInput{
		Root:         root,
		ChangedFiles: []string{"ui/app.css"},
		BaseFile:     fileReader(base),
		HeadFile:     fileReader(head),
		HeadGenerated: func(string) (bool, bool, error) {
			t.Fatal("declared output must bypass classifier")
			return false, false, nil
		},
	}
	findings, missing := GeneratedHandEdits(in)
	if len(findings) != 1 || findings[0].Key != "ui/app.css" || len(missing) != 0 {
		t.Fatalf("findings=%v missing=%v", findings, missing)
	}
}
