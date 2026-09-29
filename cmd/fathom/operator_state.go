package main

// operator_state.go — where the managed daemon's store, call log and
// address come from (RT-22, RT-23; docs/design/responsible-tenant.md
// §10.1/§10.2, I52). Three sources, because each alone was bypassed: the
// managed daemon's record, the installed launchd plist, and both of those
// under the account's own home when HOME is swapped (stranger-check and
// scenariobench run a scratch daemon under a throwaway HOME that holds no
// record, from inside a repository whose default call log IS the
// operator's).

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/testisolate"
)

// recordReadError is an unreadable daemon record, with its path.
type recordReadError struct {
	Path string
	Err  error
}

func (e *recordReadError) Error() string {
	if strings.Contains(e.Err.Error(), e.Path) {
		return e.Err.Error()
	}
	return e.Path + ": " + e.Err.Error()
}

func (e *recordReadError) Unwrap() error { return e.Err }

// accountHome is the account's home from the user database, which HOME
// does not override; "" when it cannot be read.
func accountHome() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}
	return u.HomeDir
}

// operatorHomes are the homes whose records and plists name the managed
// daemon: HOME, and the account's home when it differs. Under FATHOM_TEST=1
// (testisolate) only the isolated HOME: a test never reads the operator's.
func operatorHomes(home, account, envTest string) []string {
	var out []string
	if home != "" {
		out = append(out, home)
	}
	if envTest != "1" && account != "" && !sameResolvedPath(account, home) {
		out = append(out, account)
	}
	return out
}

// plistProgramArgs returns a launchd plist's ProgramArguments, unescaped.
func plistProgramArgs(body []byte) []string {
	dec := xml.NewDecoder(strings.NewReader(string(body)))
	var (
		args            []string
		lastKey, cur    string
		inKey, inString bool
		inArgs          bool
		depth, argDepth int
	)
	for {
		tok, err := dec.Token()
		if err != nil {
			return args
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			cur = ""
			switch t.Name.Local {
			case "key":
				inKey = true
			case "string":
				inString = true
			case "array":
				if lastKey == "ProgramArguments" && !inArgs {
					inArgs, argDepth = true, depth
				}
			}
		case xml.CharData:
			if inKey || inString {
				cur += string(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "key":
				inKey, lastKey = false, strings.TrimSpace(cur)
			case "string":
				inString = false
				if inArgs && depth == argDepth+1 {
					args = append(args, cur)
				}
			case "array":
				if inArgs && depth == argDepth {
					return args
				}
			}
			depth--
		}
	}
}

// installedDaemonArgs is the ProgramArguments of the fathom serve-http
// agent installed under home/Library/LaunchAgents (the first by name), or
// nil.
func installedDaemonArgs(home string) []string {
	dir := filepath.Join(home, "Library", "LaunchAgents")
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".plist") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, n := range names {
		body, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			continue
		}
		args := plistProgramArgs(body)
		if len(args) > 1 && strings.Contains(filepath.Base(args[0]), "fathom") && args[1] == "serve-http" {
			return args
		}
	}
	return nil
}

// cmdArgValue returns the value of --name/-name in argv.
func cmdArgValue(argv []string, name string) (string, bool) {
	for i, a := range argv {
		for _, p := range []string{"--" + name, "-" + name} {
			if a == p && i+1 < len(argv) {
				return argv[i+1], true
			}
			if v, ok := strings.CutPrefix(a, p+"="); ok {
				return v, true
			}
		}
	}
	return "", false
}

// managedDaemonAddrIn is the managed daemon's address: the installed
// plist's --addr, else configured ([daemon].addr), else 127.0.0.1:7575.
func managedDaemonAddrIn(home, configured string) string {
	if v, ok := cmdArgValue(installedDaemonArgs(home), "addr"); ok && v != "" {
		return v
	}
	return firstNonEmpty(configured, "127.0.0.1:7575")
}

// checkOperatorState is the RT-22 check over every source: it returns
// errOperatorState when --store or --call-log is the managed daemon's, or
// when a record that may be the managed daemon's cannot be read (it may
// hold exactly those paths — fail closed, naming it, I38). An unreadable
// scratch record is returned as a warning.
func checkOperatorState(storePath, callLog string, homes []string, managedAddr string) (warnings []string, err error) {
	var stores, logs []string
	for _, home := range homes {
		// The operator's own records, literally under home/.fathom and never
		// FATHOM_HOME: this guard exists to keep a scratch run off the
		// operator's daemon, so a scratch environment must not blind it.
		recs, errs := listDaemonRecordsAt(filepath.Join(home, ".fathom"), nil)
		for _, e := range errs {
			var re *recordReadError
			if !errors.As(e, &re) {
				warnings = append(warnings, fmt.Sprintf("daemon records under %s unreadable: %v", home, e))
				continue
			}
			if mayBeManagedRecord(home, re.Path, managedAddr) {
				return warnings, fmt.Errorf("%w: the managed daemon's record %s is unreadable (%v), so --store/--call-log cannot be checked against it; pass --allow-operator-state if they are scratch paths, or fix or remove the record", errOperatorState, re.Path, re.Err)
			}
			warnings = append(warnings, fmt.Sprintf("unreadable daemon record %s (%v) — not the managed daemon's; ignored for the operator-state check", re.Path, re.Err))
		}
		s, l := operatorPaths(recs, managedAddr)
		stores, logs = append(stores, s...), append(logs, l...)
		args := installedDaemonArgs(home)
		if v, ok := cmdArgValue(args, "store"); ok && v != "" {
			stores = append(stores, v)
		}
		if v, ok := cmdArgValue(args, "call-log"); ok && v != "" {
			logs = append(logs, v)
		}
	}
	return warnings, refuseOperatorState(storePath, callLog, stores, logs)
}

// mayBeManagedRecord: a record directory that is not a scratch id
// (addr-<port>-<pid>) is a managed label's; a legacy file for the managed
// address was the managed daemon's.
func mayBeManagedRecord(home, path, managedAddr string) bool {
	if filepath.Base(path) == "record.json" {
		_, _, scratch := scratchRecordID(filepath.Base(filepath.Dir(path)))
		return !scratch
	}
	legacy, err := console.DaemonRecordPath(home, managedAddr)
	return err == nil && legacy == path
}

// operatorStateHomes is operatorHomes for this process.
func operatorStateHomes() []string {
	home, _ := os.UserHomeDir()
	return operatorHomes(home, accountHome(), os.Getenv(testisolate.EnvTest))
}
