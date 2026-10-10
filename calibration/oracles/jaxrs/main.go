// Command jaxrs-oracle converts a running JAX-RS application's own OpenAPI
// document (SmallRye OpenAPI's GET /q/openapi?format=json on Quarkus, or
// any OpenAPI 3 document the runtime generates from the resource classes it
// deployed) into the route oracle shape indexbench route-oracle scores
// (calibration/oracles/routes/README.md). Tier A: the runtime's own model of
// the resources it serves, read from the running application.
//
//	go run . -in openapi.json -strip-prefix /api -subject owner/repo -sha <sha> \
//	    -licence MIT -tool smallrye-openapi -tool-version "quarkus 3.40.1" > app.json
//
// Every operation is a route: its method upper-cased, its path as the
// document writes it. The handler comes from the operationId, which the
// build must generate with SmallRye's CLASS_METHOD strategy
// (quarkus.smallrye-openapi.operation-id-strategy=CLASS_METHOD): "Class_method"
// becomes fathom's "Class.method" (split at the first "_"); an operation
// with no operationId has handler "". -strip-prefix removes a root path the
// deployment configures (quarkus.rest.path, a servlet mapping) rather than
// the source declares, as the Spring converter leaves out the servlet
// context path; the README of a subject that needs it says so.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type route struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Handler string `json:"handler"`
}

type oracle struct {
	Tool        string  `json:"tool"`
	ToolVersion string  `json:"tool_version"`
	Subject     string  `json:"subject"`
	SHA         string  `json:"sha"`
	Licence     string  `json:"licence"`
	Routes      []route `json:"routes"`
}

// httpMethods are the OpenAPI path-item keys that are operations.
var httpMethods = map[string]bool{"get": true, "put": true, "post": true, "delete": true, "options": true, "head": true, "patch": true, "trace": true}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("jaxrs-oracle", flag.ContinueOnError)
	fs.SetOutput(stderr)
	in := fs.String("in", "", "OpenAPI 3 JSON the running application served (required)")
	strip := fs.String("strip-prefix", "", "a configured root path to remove from every path")
	subject := fs.String("subject", "", "subject name, owner/repo (required)")
	sha := fs.String("sha", "", "subject commit (required)")
	licence := fs.String("licence", "", "subject licence SPDX id at the pin (required)")
	tool := fs.String("tool", "smallrye-openapi", "the runtime's OpenAPI generator")
	toolVersion := fs.String("tool-version", "", "runtime version that served the document (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *in == "" || *subject == "" || *sha == "" || *licence == "" || *toolVersion == "" {
		fmt.Fprintln(stderr, "usage: jaxrs-oracle -in openapi.json [-strip-prefix /api] -subject S -sha SHA -licence SPDX -tool-version V")
		return 2
	}
	b, err := os.ReadFile(*in)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	routes, err := convert(b, *strip)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	o := oracle{Tool: *tool, ToolVersion: *toolVersion, Subject: *subject, SHA: *sha, Licence: *licence, Routes: routes}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(o); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// convert reads an OpenAPI document's operations.
func convert(b []byte, strip string) ([]route, error) {
	var doc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("openapi: %w", err)
	}
	strip = strings.TrimRight(strip, "/")
	var out []route
	for p, item := range doc.Paths {
		path := p
		if strip != "" && (path == strip || strings.HasPrefix(path, strip+"/")) {
			path = strings.TrimPrefix(path, strip)
			if path == "" {
				path = "/"
			}
		}
		for m, raw := range item {
			if !httpMethods[m] {
				continue
			}
			var op struct {
				OperationID string `json:"operationId"`
			}
			if err := json.Unmarshal(raw, &op); err != nil {
				return nil, fmt.Errorf("openapi %s %s: %w", m, p, err)
			}
			out = append(out, route{Method: strings.ToUpper(m), Path: path, Handler: handlerOf(op.OperationID)})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Method != b.Method {
			return a.Method < b.Method
		}
		return a.Handler < b.Handler
	})
	return out, nil
}

// handlerOf turns a CLASS_METHOD operationId into Class.method.
func handlerOf(id string) string {
	class, method, ok := strings.Cut(id, "_")
	if !ok {
		return ""
	}
	return class + "." + method
}
