package main

// doctor_keyscope.go — SEC-1 and SEC-2 (ADR-042 amendments 2026-10-09,
// DP-40, CF-15): the key_source_scope row. A repository's
// .fathom/config.toml chooses behaviour only: config.Load drops every
// user-only field it sets (an executable, a credential, a key host, the
// daemon's own paths and switches, [tenancy]; internal/config/scope.go),
// and every use site refuses a repo-path value that leaves the repository.
// Doctor names both so the author finds out the setting does nothing. The
// row keeps its SEC-1 name.

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/config"
)

// checkKeyScope is key_source_scope: WARN naming file and field (never a
// value) for each user-only field the repository's config set, and each
// repo-path field whose value leaves the repository; PASS says where the
// user-only fields are read.
func checkKeyScope(root, userPath string) doctorCheck {
	const name = "key_source_scope"
	projPath := filepath.Join(root, ".fathom", "config.toml")
	loaded, err := config.Load(userPath, projPath)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorWarn, Reason: "config does not load: " + err.Error(), Remedy: "see config_valid"}
	}
	refused := config.RefusedRepoPaths(root, loaded.Config)
	if len(loaded.IgnoredKeySources) == 0 && len(refused) == 0 {
		return doctorCheck{Name: name, Status: doctorPass, Reason: "credentials, executables, key hosts and the daemon's own paths are read from the user config (" + userPath + ") only; the repository sets none, and its paths stay inside it"}
	}
	var parts, remedies []string
	if n := len(loaded.IgnoredKeySources); n > 0 {
		fields := make([]string, 0, n)
		for _, ig := range loaded.IgnoredKeySources {
			fields = append(fields, ig.Field)
		}
		parts = append(parts, fmt.Sprintf("%s sets %s: ignored (user-only: a repository chooses behaviour, never an executable, a credential, a path outside it or automatic execution)", loaded.IgnoredKeySources[0].Path, strings.Join(fields, ", ")))
		remedies = append(remedies, "delete the user-only fields from the repository's .fathom/config.toml and set them in "+userPath)
	}
	if len(refused) > 0 {
		fields := make([]string, 0, len(refused))
		for _, r := range refused {
			fields = append(fields, r.Field+" "+r.Err.Error())
		}
		parts = append(parts, fmt.Sprintf("%s: refused at use (%s)", projPath, strings.Join(fields, "; ")))
		remedies = append(remedies, "name repository-relative paths inside the repository")
	}
	return doctorCheck{Name: name, Status: doctorWarn, Reason: strings.Join(parts, "; "), Remedy: strings.Join(remedies, "; ")}
}
