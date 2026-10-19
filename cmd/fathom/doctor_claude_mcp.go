package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type claudeMCPEntry struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type claudeMCPDocument struct {
	MCPServers map[string]claudeMCPEntry `json:"mcpServers"`
	Projects   map[string]struct {
		MCPServers map[string]claudeMCPEntry `json:"mcpServers"`
	} `json:"projects"`
}

// This is a read-only configuration check for the CLI's local > project > user
// precedence. A matching entry proves neither workspace trust nor consumption.
// Managed policy, command-line settings sources and runtime enablement remain
// host checks: doctor does not approve, connect to or modify any server here.
func checkClaudeMCPRegistration(root, baseURL string) doctorCheck {
	const name = "mcp_registration"
	remedy := "inspect the project's .mcp.json fathom HTTP entry and `claude mcp get fathom`; review workspace trust and server approval in the actual session"
	unknown := func(reason string) doctorCheck {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: reason, Remedy: remedy}
	}
	read := func(path string) (claudeMCPDocument, bool, *doctorCheck) {
		var doc claudeMCPDocument
		data, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			return doc, false, nil
		}
		if err != nil {
			c := unknown(fmt.Sprintf("%s is not readable (%s)", path, errnoOf(err)))
			return doc, false, &c
		}
		var object map[string]json.RawMessage
		if json.Unmarshal(data, &object) != nil || object == nil || json.Unmarshal(data, &doc) != nil {
			c := unknown(fmt.Sprintf("%s is not a valid MCP configuration object; scope precedence cannot be checked", path))
			return doc, false, &c
		}
		return doc, true, nil
	}
	check := func(entry claudeMCPEntry, path, scope string) doctorCheck {
		if strings.Contains(entry.URL, "${") {
			return unknown(fmt.Sprintf("fathom in %s (%s scope) has an unexpanded URL variable; inspect the host's resolved entry", path, scope))
		}
		endpoint, ok := claudeMCPEndpoint(entry.URL)
		matched := false
		for _, surface := range []string{"/rpc/agent", "/rpc/companion"} {
			expected, valid := claudeMCPEndpoint(strings.TrimRight(baseURL, "/") + surface)
			matched = matched || (ok && valid && endpoint == expected)
		}
		if entry.Type != "http" || !matched {
			// Do not echo arbitrary configured URLs: userinfo and query values
			// may carry credentials, even when the entry is misconfigured.
			return doctorCheck{Name: name, Status: doctorFail, Reason: fmt.Sprintf("fathom in %s (%s scope) does not configure HTTP at the requested daemon's /rpc/agent or /rpc/companion endpoint", path, scope), Remedy: remedy}
		}
		return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("fathom configured in %s (%s scope); workspace trust, enablement, connection and tool consumption not checked", path, scope)}
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return unknown("cannot resolve home directory to inspect Claude's local/user MCP scopes")
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return unknown("cannot resolve the project root to inspect Claude's MCP scopes")
	}
	userPath := filepath.Join(home, ".claude.json")
	user, userRead, problem := read(userPath)
	if problem != nil {
		return *problem
	}
	if entry, ok := user.Projects[root].MCPServers["fathom"]; ok {
		return check(entry, userPath, "local")
	}
	projectPath := filepath.Join(root, ".mcp.json")
	project, projectRead, problem := read(projectPath)
	if problem != nil {
		return *problem
	}
	if entry, ok := project.MCPServers["fathom"]; ok {
		return check(entry, projectPath, "project")
	}
	if entry, ok := user.MCPServers["fathom"]; ok {
		return check(entry, userPath, "user")
	}
	if !userRead && !projectRead {
		return unknown("no readable project .mcp.json or user .claude.json; inspect the actual host's MCP configuration")
	}
	return doctorCheck{Name: name, Status: doctorFail, Reason: "no fathom entry in the project's local, project or user MCP scopes", Remedy: remedy}
}

// Normalize only endpoint spelling, never DNS or environment variables. Query,
// fragment and userinfo differ from the requested plain daemon endpoint.
func claudeMCPEndpoint(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", false
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", false
	}
	host := strings.ToLower(u.Host)
	if (scheme == "http" && u.Port() == "80") || (scheme == "https" && u.Port() == "443") {
		host = strings.TrimSuffix(host, ":"+u.Port())
	}
	return scheme + "://" + host + strings.TrimRight(u.EscapedPath(), "/"), true
}
