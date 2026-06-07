// Package proto parses Protocol Buffer (.proto) source text and validates naming
// conventions without requiring any protobuf library dependency.
//
// All exported functions are pure, deterministic, and safe for concurrent use.
package proto

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// ── Data model ───────────────────────────────────────────────────────────────

// RPC is a single remote-procedure call declaration inside a service.
type RPC struct {
	Name   string
	Input  string
	Output string
}

// Service is a gRPC service declaration.
type Service struct {
	Name string
	RPCs []RPC
}

// Field is a message field declaration.
type Field struct {
	Name   string
	Type   string
	Number int
}

// Message is a protobuf message declaration.
type Message struct {
	Name   string
	Fields []Field
}

// Contract is the parsed representation of a .proto file.
type Contract struct {
	Package  string
	Services []Service
	Messages []Message
}

// Issue is a single conformance violation.
type Issue struct {
	Location string // e.g. "Service.UserService" or "Message.user_event.user_id"
	Rule     string
	Message  string
}

// ── Parsing ──────────────────────────────────────────────────────────────────

var (
	rePkg     = regexp.MustCompile(`^\s*package\s+([\w.]+)\s*;`)
	reService = regexp.MustCompile(`^\s*service\s+(\w+)\s*\{?`)
	reRPC     = regexp.MustCompile(`^\s*rpc\s+(\w+)\s*\((\w+)\)\s+returns\s+\((\w+)\)`)
	reMessage = regexp.MustCompile(`^\s*message\s+(\w+)\s*\{?`)
	// field: [optional label] type name = number;
	reField = regexp.MustCompile(`^\s*(?:(?:repeated|optional|required)\s+)?(\w+)\s+(\w+)\s*=\s*(\d+)\s*;`)
)

// Parse parses proto source text into a Contract. It is line-oriented and uses
// only the regexp/strings standard library — no protobuf compiler needed.
func Parse(src string) Contract {
	lines := strings.Split(src, "\n")

	var c Contract
	var curService *Service
	var curMessage *Message
	depth := 0 // brace-depth relative to current top-level declaration

	for _, raw := range lines {
		line := stripComment(raw)

		// Track brace depth so we know when a top-level block ends.
		opened := strings.Count(line, "{")
		closed := strings.Count(line, "}")

		// Package
		if m := rePkg.FindStringSubmatch(line); m != nil && curService == nil && curMessage == nil {
			c.Package = m[1]
		}

		// Service start
		if m := reService.FindStringSubmatch(line); m != nil && depth == 0 {
			s := Service{Name: m[1]}
			c.Services = append(c.Services, s)
			curService = &c.Services[len(c.Services)-1]
			curMessage = nil
			depth = opened - closed
			continue
		}

		// Message start
		if m := reMessage.FindStringSubmatch(line); m != nil && depth == 0 {
			msg := Message{Name: m[1]}
			c.Messages = append(c.Messages, msg)
			curMessage = &c.Messages[len(c.Messages)-1]
			curService = nil
			depth = opened - closed
			continue
		}

		// Inside a block
		if depth > 0 {
			depth += opened - closed

			if curService != nil {
				if m := reRPC.FindStringSubmatch(line); m != nil {
					curService.RPCs = append(curService.RPCs, RPC{
						Name:   m[1],
						Input:  m[2],
						Output: m[3],
					})
				}
			}

			if curMessage != nil {
				if m := reField.FindStringSubmatch(line); m != nil {
					n, _ := strconv.Atoi(m[3])
					curMessage.Fields = append(curMessage.Fields, Field{
						Type:   m[1],
						Name:   m[2],
						Number: n,
					})
				}
			}

			if depth <= 0 {
				depth = 0
				curService = nil
				curMessage = nil
			}
			continue
		}

		depth += opened - closed
		if depth < 0 {
			depth = 0
		}
	}

	return c
}

// stripComment removes // line comments.
func stripComment(s string) string {
	if idx := strings.Index(s, "//"); idx >= 0 {
		return s[:idx]
	}
	return s
}

// ── Conformance ──────────────────────────────────────────────────────────────

// Conformance checks naming conventions on a parsed Contract and returns a
// deterministic (sorted) slice of Issues.
func Conformance(c Contract) []Issue {
	var issues []Issue

	for _, svc := range c.Services {
		if !isPascalCase(svc.Name) {
			issues = append(issues, Issue{
				Location: fmt.Sprintf("Service.%s", svc.Name),
				Rule:     "service-pascal-case",
				Message:  fmt.Sprintf("service name %q must be PascalCase", svc.Name),
			})
		}
		for _, rpc := range svc.RPCs {
			if !isPascalCase(rpc.Name) {
				issues = append(issues, Issue{
					Location: fmt.Sprintf("Service.%s.%s", svc.Name, rpc.Name),
					Rule:     "rpc-pascal-case",
					Message:  fmt.Sprintf("RPC name %q must be PascalCase", rpc.Name),
				})
			}
		}
	}

	for _, msg := range c.Messages {
		if !isPascalCase(msg.Name) {
			issues = append(issues, Issue{
				Location: fmt.Sprintf("Message.%s", msg.Name),
				Rule:     "message-pascal-case",
				Message:  fmt.Sprintf("message name %q must be PascalCase", msg.Name),
			})
		}
		for _, f := range msg.Fields {
			if !isSnakeCase(f.Name) {
				issues = append(issues, Issue{
					Location: fmt.Sprintf("Message.%s.%s", msg.Name, f.Name),
					Rule:     "field-snake-case",
					Message:  fmt.Sprintf("field name %q must be snake_case", f.Name),
				})
			}
		}
	}

	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Location != issues[j].Location {
			return issues[i].Location < issues[j].Location
		}
		return issues[i].Rule < issues[j].Rule
	})
	return issues
}

// isPascalCase returns true when s starts with an uppercase letter and contains
// no underscores (e.g. "UserService", "GetUser").
func isPascalCase(s string) bool {
	if s == "" {
		return false
	}
	runes := []rune(s)
	if !unicode.IsUpper(runes[0]) {
		return false
	}
	return !strings.Contains(s, "_")
}

// isSnakeCase returns true when s is all lowercase/digits and uses only
// underscores as separators, with no leading/trailing underscores and no
// consecutive underscores.
func isSnakeCase(s string) bool {
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, "_") || strings.HasSuffix(s, "_") {
		return false
	}
	if strings.Contains(s, "__") {
		return false
	}
	for _, r := range s {
		if !unicode.IsLower(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}
