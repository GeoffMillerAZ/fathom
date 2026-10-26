// Command extractor-output exports actual fathom nodes for an independence
// control. The native oracle never consumes this file or these nodes.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

func main() {
	source := flag.String("source", "", "authored nginx source file")
	out := flag.String("out", "", "scratch node-output file")
	flag.Parse()
	if *source == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "source and out are required")
		os.Exit(2)
	}
	data, err := os.ReadFile(*source)
	if err == nil {
		nodes, extractErr := extractor.NewNginxExtractor().Extract("independence", "nginx.conf", data)
		err = extractErr
		if err == nil {
			data, err = json.Marshal(nodes)
		}
	}
	if err == nil {
		err = os.WriteFile(*out, append(data, '\n'), 0o600)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
