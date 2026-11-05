// Command consumer is a module outside fathom that imports the language
// library. TestOutsideModuleBuilds builds and runs it offline.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

func main() {
	r, err := lang.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer r.Close()
	src := "resource \"aws_s3_bucket\" \"b\" {\n  bucket = var.name\n}\nvariable \"name\" {}\n"
	res, err := r.Extract(context.Background(), lang.File{Path: "main.tf", Src: []byte(src)})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(res); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
