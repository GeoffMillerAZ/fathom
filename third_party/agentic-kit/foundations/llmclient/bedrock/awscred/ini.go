package awscred

import (
	"bufio"
	"bytes"
	"os"
	"strings"
)

// parseINI parses AWS-style INI content into section -> key -> value. Keys
// are lowercased (AWS profile keys are conventionally lowercase; a mixed-case
// file is normalized so lookups are case-insensitive on keys, matching AWS
// CLI/SDK behavior). Section names are NOT lowercased — profile names are
// case-sensitive. Lines beginning with '#' or ';' are comments; blank lines
// are ignored; keys appearing before any [section] header are ignored
// (malformed input, not a parse error — this is a best-effort reader over a
// file this package does not own).
func parseINI(data []byte) map[string]map[string]string {
	sections := map[string]map[string]string{}
	var cur string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			cur = strings.TrimSpace(line[1 : len(line)-1])
			if _, ok := sections[cur]; !ok {
				sections[cur] = map[string]string{}
			}
			continue
		}
		if cur == "" {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		sections[cur][strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}
	return sections
}

// readINIFile reads and parses path. A missing file is not an error — it
// returns an empty section map, letting callers treat "no shared config on
// this host" the same as "an empty shared config." Every OTHER read failure
// (permission denied, I/O error) is deliberately a hard error, not forgiven:
// when the chain has genuinely reached the file legs, an unreadable config
// is a real configuration problem the caller must hear about — and callers
// with explicit/env credentials never reach this function at all (Resolve
// short-circuits before any file access), so the hard failure can never
// break them.
func readINIFile(path string) (map[string]map[string]string, error) {
	if path == "" {
		return map[string]map[string]string{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]map[string]string{}, nil
		}
		return nil, err
	}
	return parseINI(data), nil
}

// configProfileSection returns the [section] name a profile uses in the
// shared CONFIG file: "default" is special-cased to the bare "default"
// section; every other profile is prefixed "profile " per the AWS
// config-file convention. This prefix does NOT apply to the credentials
// file, where every profile (including "default") is its own bare section.
func configProfileSection(profile string) string {
	if profile == "default" {
		return "default"
	}
	return "profile " + profile
}
