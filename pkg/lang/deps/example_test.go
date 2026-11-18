package deps_test

import (
	"context"
	"fmt"
	"testing/fstest"

	"github.com/geoffmilleraz/fathom/pkg/lang"
	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
)

func ExampleWithPack() {
	parser, err := lang.New(lang.WithDialects(packFS()))
	if err != nil {
		panic(err)
	}
	defer parser.Close()

	reader, err := deps.New(deps.WithPack(parser, "task-spec", map[string]deps.Kind{"includes": deps.KindInclude}))
	if err != nil {
		panic(err)
	}
	repo := fstest.MapFS{
		"build.task.json": {Data: []byte(`{"includes":["settings.txt"]}`)},
		"settings.txt":    {},
	}
	inputs := []deps.Input{{Path: "build.task.json", Dialect: "task-spec"}}
	files, err := reader.ReadFiles(context.Background(), repo, inputs)
	if err != nil {
		panic(err)
	}
	file := files[0]
	for _, edge := range file.Edges {
		fmt.Println(edge.Kind, edge.To, edge.Provenance)
	}
	fmt.Println("measured", file.Measured, "complete", file.Complete)
	// Output:
	// include settings.txt pack
	// measured false complete false
}
