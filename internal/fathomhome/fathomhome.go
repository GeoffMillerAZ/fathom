// Package fathomhome resolves the per-user fathom directory, ~/.fathom.
//
// PE-26 (docs/plans/progress-and-economics.md, docs/design/progress-and-
// economics.md section 11, observation O121): a scratch-clone or test run
// of `fathom verify` or `fathom intent audit` appended rows keyed by its
// scratch path to the operator's real ledgers, and a worker running against
// the shared daemon wrote into the operator's daemon records and logs. One
// environment variable, FATHOM_HOME, now relocates the whole directory
// (ledgers, logs, stamps, caches, daemon records) so a scratch run names its
// own and leaves the real one untouched.
//
// Precedence, for a file that has its own variable (FATHOM_VERIFY_LEDGER,
// FATHOM_INTENT_LEDGER, FATHOM_SESSION_LEDGER, FATHOM_HOOKS_LOG, ...): the
// per-file variable, then FATHOM_HOME, then $HOME/.fathom. Nothing here
// touches the disk.
//
// One thing deliberately stays machine-wide: the gate lease's lock
// (scripts/gate-lease.sh), which must be one file for every worker on the
// machine or concurrent gates stop queueing. It is a shell script and does
// not read FATHOM_HOME.
package fathomhome

import (
	"os"
	"path/filepath"
)

// Env is the variable that relocates the whole per-user fathom directory.
const Env = "FATHOM_HOME"

// dirName is the directory's name under the user's home.
const dirName = ".fathom"

// FathomHome is the per-user fathom directory: $FATHOM_HOME when set (made
// absolute against the working directory once, so a child that changes
// directory reads the same place), else $HOME/.fathom, else "" when no home
// can be resolved. Callers treat "" as "no per-user state", exactly as they
// did when os.UserHomeDir failed.
func FathomHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return Under(home)
}

// Under is FathomHome for a caller that already holds a home directory (an
// injected one in a test, or the one it resolved for other reasons): the
// override is honored when set, else home/.fathom, and "" when neither
// exists. Callers that take a home parameter (the daemon records, gc, the
// compilation cache) use it, so the one variable moves them too.
func Under(home string) string {
	if v := os.Getenv(Env); v != "" {
		if abs, err := filepath.Abs(v); err == nil {
			return abs
		}
		return filepath.Clean(v)
	}
	if home == "" {
		return ""
	}
	return filepath.Join(home, dirName)
}

// At joins elem under home's own fathom directory, home/.fathom, literally:
// it never reads FATHOM_HOME. It is for a caller that must see one home's
// directory whatever the override says (the guard that keeps a scratch run
// off the operator's daemon) or that addresses a fixture home it built.
func At(home string, elem ...string) string {
	return filepath.Join(append([]string{home, dirName}, elem...)...)
}

// Path joins elem under FathomHome, or returns "" when FathomHome is.
func Path(elem ...string) string {
	h := FathomHome()
	if h == "" {
		return ""
	}
	return filepath.Join(append([]string{h}, elem...)...)
}

// PerFile resolves a file that has its own override: the value of envVar
// verbatim when set (it wins over FATHOM_HOME), else elem under FathomHome
// ("" when neither resolves).
func PerFile(envVar string, elem ...string) string {
	if p := os.Getenv(envVar); p != "" {
		return p
	}
	return Path(elem...)
}
