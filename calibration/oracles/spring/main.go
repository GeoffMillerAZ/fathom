// Command spring-oracle converts a running Spring Boot application's own
// route dump, the actuator mappings endpoint (GET /actuator/mappings), into
// the route oracle shape indexbench route-oracle scores
// (calibration/oracles/routes/README.md). Tier A: the framework's own
// registry of the handler methods it dispatches to, read from the running
// application.
//
//	go run . -in mappings.json -package org.springframework.samples.petclinic \
//	    -subject spring-projects/spring-petclinic -sha <sha> -licence Apache-2.0 \
//	    -tool-version "spring-boot 4.1.0" > petclinic.json
//
// Every dispatcher-servlet mapping whose handler method is declared in a
// class under -package is a route: one per pattern and method, ANY when the
// mapping names no method. Mappings the framework or a library contributes
// (the error controller, actuator endpoints, static resources) are not the
// application's source and are left out. The handler is fathom's naming:
// the class's simple name (a nested class's "Outer$Inner" as "Outer.Inner"),
// ".", the method name. The servlet context path is not part of a pattern,
// as it is not part of the route the source declares.
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

type mappings struct {
	Contexts map[string]struct {
		Mappings struct {
			DispatcherServlets map[string][]struct {
				Handler string `json:"handler"`
				Details *struct {
					HandlerMethod *struct {
						ClassName string `json:"className"`
						Name      string `json:"name"`
					} `json:"handlerMethod"`
					RequestMappingConditions *struct {
						Methods  []string `json:"methods"`
						Patterns []string `json:"patterns"`
					} `json:"requestMappingConditions"`
				} `json:"details"`
			} `json:"dispatcherServlets"`
		} `json:"mappings"`
	} `json:"contexts"`
}

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

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("spring-oracle", flag.ContinueOnError)
	fs.SetOutput(stderr)
	in := fs.String("in", "", "actuator mappings JSON (required)")
	pkg := fs.String("package", "", "the application's root Java package (required)")
	subject := fs.String("subject", "", "subject name, owner/repo (required)")
	sha := fs.String("sha", "", "subject commit (required)")
	licence := fs.String("licence", "", "subject licence SPDX id at the pin (required)")
	toolVersion := fs.String("tool-version", "", "framework version that served the dump (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *in == "" || *pkg == "" || *subject == "" || *sha == "" || *licence == "" || *toolVersion == "" {
		fmt.Fprintln(stderr, "usage: spring-oracle -in mappings.json -package P -subject S -sha SHA -licence SPDX -tool-version V")
		return 2
	}
	b, err := os.ReadFile(*in)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	routes, err := convert(b, *pkg)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	o := oracle{Tool: "spring-boot-actuator-mappings", ToolVersion: *toolVersion, Subject: *subject, SHA: *sha, Licence: *licence, Routes: routes}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(o); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// convert reads the mappings dump's application routes.
func convert(b []byte, pkg string) ([]route, error) {
	var m mappings
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("mappings: %w", err)
	}
	seen := map[route]bool{}
	var out []route
	for _, ctx := range m.Contexts {
		for _, ds := range ctx.Mappings.DispatcherServlets {
			for _, e := range ds {
				if e.Details == nil || e.Details.HandlerMethod == nil || e.Details.RequestMappingConditions == nil {
					continue
				}
				hm := e.Details.HandlerMethod
				if !strings.HasPrefix(hm.ClassName, pkg+".") {
					continue
				}
				class := hm.ClassName[strings.LastIndex(hm.ClassName, ".")+1:]
				handler := strings.ReplaceAll(class, "$", ".") + "." + hm.Name
				methods := e.Details.RequestMappingConditions.Methods
				if len(methods) == 0 {
					methods = []string{"ANY"}
				}
				for _, p := range e.Details.RequestMappingConditions.Patterns {
					for _, me := range methods {
						r := route{Method: strings.ToUpper(me), Path: p, Handler: handler}
						if !seen[r] {
							seen[r] = true
							out = append(out, r)
						}
					}
				}
			}
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
