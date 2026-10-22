package main

// doctor_providers.go — SR-32 (surface reckoning F-71, design §4): the
// `providers` row folds `fathom providers`'s own zero/near-zero-token
// ProbeAll auth-health pass (CF-5, providers.go) into `fathom doctor` so
// CF-5 is exercised without a second top-level command. `providers` and
// `smoke` become hidden `fathom help` aliases (verbs.go) — hidden, never
// deleted, so a hook or skill calling either directly keeps working.
//
// `fathom smoke` (CF-6, smoke.go) is deliberately NEVER run here: it is
// "opt-in real completion" by design (docs/plans/config-experience.md
// CF-6) — a doctor run must never spend real money against a real
// account just because someone ran a wiring diagnosis.
import (
	"context"
	"fmt"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// doctorProvidersRow probes every configured profile (or the bare
// env-overlay lane when [profiles] is empty) the same way probeProfiles
// does for `fathom providers`, then condenses the per-profile rows into
// one doctor row:
//
//   - no [profiles] section AND the sole env-overlay lane cannot even be
//     constructed/pinged: skipped, naming why — an adopter who has never
//     touched an LLM profile must not see a wiring diagnosis go red for a
//     feature they never configured (config_valid's fail-loud discipline
//     is for a BROKEN config, not an ABSENT one).
//   - at least one [profiles] entry exists and any probed lane failed:
//     fail, naming the first failure — a configured credential that does
//     not work is exactly what this row exists to catch.
//   - otherwise: pass, counting ok vs not-probeable lanes.
func doctorProvidersRow(root string, build profileClientBuilder, parents ...context.Context) doctorCheck {
	ctx, cancel := doctorContextArg(parents)
	defer cancel()
	const name = "providers"

	loaded, err := config.Load(config.DefaultUserPath(), projectdir.Config.Path(root))
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("config did not load: %v", err)}
	}
	names, err := selectProfileNames(loaded.Config, "")
	if err != nil {
		return doctorCheck{
			Name: name, Status: doctorFail,
			Reason: err.Error(),
			Remedy: "fix [profiles] in .fathom/config.toml",
		}
	}

	rows := probeProfilesContext(ctx, loaded, names, build)

	var okN, notProbeableN, errN int
	var firstErr string
	for _, r := range rows {
		switch r.Status {
		case providerStatusOK:
			okN++
		case providerStatusNotProbeable:
			notProbeableN++
		case providerStatusError:
			errN++
			if firstErr == "" {
				firstErr = fmt.Sprintf("%s: %s", r.Name, r.Detail)
			}
		}
	}

	if errN > 0 && len(loaded.Config.Profiles) == 0 {
		return doctorCheck{
			Name: name, Status: doctorSkipped,
			Reason: fmt.Sprintf("no [profiles] configured and the env-overlay lane is not usable (%s) — fathom providers/smoke are optional", firstErr),
		}
	}
	if errN > 0 {
		return doctorCheck{
			Name: name, Status: doctorFail,
			Reason: fmt.Sprintf("%d/%d profile(s) failed auth, e.g. %s", errN, len(rows), firstErr),
			Remedy: "fathom providers  (full per-profile detail)",
		}
	}
	reason := fmt.Sprintf("%d/%d profile(s) ok", okN, len(rows))
	if notProbeableN > 0 {
		reason += fmt.Sprintf(", %d not probeable (no Ping support)", notProbeableN)
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
}
