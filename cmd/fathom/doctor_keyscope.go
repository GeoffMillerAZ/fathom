package main

// doctor_keyscope.go — SEC-1 (ADR-042 amendment 2026-10-09, DP-40): the
// key_source_scope row. Credentials and the commands that fetch them are
// the user's (~/.fathom/config.toml), never a repository's; config.Load
// drops a repository's api_key_cmd / api_key_file / api_key_env and doctor
// names each one so the author finds out the setting does nothing.

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/config"
)

// checkKeyScope is key_source_scope: WARN naming file and field (never a
// value) for each key-source field the repository's config set; PASS says
// where key sources are read.
func checkKeyScope(root, userPath string) doctorCheck {
	const name = "key_source_scope"
	loaded, err := config.Load(userPath, filepath.Join(root, ".fathom", "config.toml"))
	if err != nil {
		return doctorCheck{Name: name, Status: doctorWarn, Reason: "config does not load: " + err.Error(), Remedy: "see config_valid"}
	}
	if len(loaded.IgnoredKeySources) == 0 {
		return doctorCheck{Name: name, Status: doctorPass, Reason: "api_key_cmd, api_key_file and api_key_env are read from the user config (" + userPath + ") only; the repository sets none"}
	}
	fields := make([]string, 0, len(loaded.IgnoredKeySources))
	for _, ig := range loaded.IgnoredKeySources {
		fields = append(fields, ig.Field)
	}
	return doctorCheck{
		Name: name, Status: doctorWarn,
		Reason: fmt.Sprintf("%s sets %s: ignored (a credential and the command that fetches it are the user's, never a repository's)", loaded.IgnoredKeySources[0].Path, strings.Join(fields, ", ")),
		Remedy: "delete the key-source fields from the repository's .fathom/config.toml and set them in " + userPath,
	}
}
