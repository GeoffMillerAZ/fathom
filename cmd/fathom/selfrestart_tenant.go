package main

// selfrestart_tenant.go — the responsible-tenant half of self-restart
// (docs/design/responsible-tenant.md §8.2, RT-11/RT-12): the stat gate that
// keeps the watcher from hashing an 89 MB binary every 20 s, the
// cross-process cooldown kept in the lifecycle ledger, the managed-only
// gate, and the plist reading that proves the adoption exit code relaunches.

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
)

// binaryStat is the (size, mtime, inode) triple the watcher compares
// between polls. An in-place rewrite changes size or mtime; a rename-over
// deploy (`task deploy`'s install) changes the inode.
type binaryStat struct {
	size    int64
	mtimeNs int64
	ino     uint64
}

func statBinary(path string) (binaryStat, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return binaryStat{}, err
	}
	st := binaryStat{size: fi.Size(), mtimeNs: fi.ModTime().UnixNano()}
	if sys, ok := fi.Sys().(*syscall.Stat_t); ok {
		st.ino = uint64(sys.Ino)
	}
	return st, nil
}

// binaryStatChanged reports whether the executable may have new bytes: the
// only condition under which the watcher hashes it (RT-12).
func binaryStatChanged(prev, cur binaryStat) bool {
	return prev != cur
}

// selfRestartManaged reports whether this serve-http is the managed daemon,
// the only one that watches its own binary (design §8.2, §10.1): started
// with --managed (plist schema 2), or launched by launchd under the label of
// the fathom agent installed on this machine (XPC_SERVICE_NAME, which
// launchd sets to the job's label; a shell-started daemon inherits its
// terminal's value, which is never a fathom agent's label). It returns the
// label the lifecycle ledger is kept under.
func selfRestartManaged(managedFlag bool, xpcServiceName string, installedLabel func() string) (bool, string) {
	label := ""
	if xpcServiceName != "" && xpcServiceName != "0" {
		if inst := installedLabel(); inst != "" && inst == xpcServiceName {
			label = inst
		}
	}
	if managedFlag {
		if label == "" {
			label = firstNonEmpty(installedLabel(), defaultDaemonLabel)
		}
		return true, label
	}
	return label != "", label
}

// lifecycleLedgerPath is ~/.fathom/daemon/<label>/lifecycle.jsonl (design
// §2.3), or "" when HOME is unresolvable.
func lifecycleLedgerPath(label string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" || label == "" {
		return ""
	}
	return filepath.Join(home, ".fathom", "daemon", label, "lifecycle.jsonl")
}

// lifecycleSchemaVersion versions the ledger rows (I32's pattern).
const lifecycleSchemaVersion = 1

// lifecycleRow is one ledger line. Only the `adopt` row is written here
// (wave A); wave B's ledger (RT-37) adds start/exit/mode/breaker rows in
// the same file and shape.
type lifecycleRow struct {
	SchemaVersion int    `json:"schema_version"`
	FathomVersion string `json:"fathom_version,omitempty"`
	Row           string `json:"row"`
	At            string `json:"at"`
	PID           int    `json:"pid,omitempty"`
	OldBinary     string `json:"old_binary,omitempty"`
	NewBinary     string `json:"new_binary,omitempty"`
	OldCommit     string `json:"old_binary_commit,omitempty"`
}

// ledgerTailBytes bounds how much of the ledger lastAdoption reads: the
// ledger rotates at 1 MiB (design §2.3), so the tail holds every row that
// can still matter to a 5-minute cooldown.
const ledgerTailBytes = 1 << 20

// lastAdoption returns the time of the newest `adopt` row in the ledger.
// A missing ledger is (zero, false, nil): no adoption on record. Any other
// read failure is returned, never folded into "none" (I38).
func lastAdoption(ledger string) (time.Time, bool, error) {
	f, err := os.Open(ledger)
	if errors.Is(err, os.ErrNotExist) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return time.Time{}, false, err
	}
	if off := fi.Size() - ledgerTailBytes; off > 0 {
		if _, err := f.Seek(off, io.SeekStart); err != nil {
			return time.Time{}, false, err
		}
	}
	var (
		last  time.Time
		found bool
	)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 64*1024)
	for sc.Scan() {
		var r lifecycleRow
		if json.Unmarshal(sc.Bytes(), &r) != nil || r.Row != "adopt" {
			continue // a torn first line after the seek, or another row kind
		}
		t, perr := time.Parse(time.RFC3339Nano, r.At)
		if perr != nil {
			continue
		}
		if !found || t.After(last) {
			last, found = t, true
		}
	}
	if err := sc.Err(); err != nil {
		return time.Time{}, false, err
	}
	return last, found, nil
}

// appendAdoptRow appends an `adopt` row (old and new binary) to the ledger.
func appendAdoptRow(ledger string, at time.Time, oldHash, newHash string) error {
	if err := os.MkdirAll(filepath.Dir(ledger), 0o700); err != nil {
		return err
	}
	row := lifecycleRow{
		SchemaVersion: lifecycleSchemaVersion,
		FathomVersion: buildinfo.Version(),
		Row:           "adopt",
		At:            at.UTC().Format(time.RFC3339Nano),
		PID:           os.Getpid(),
		OldBinary:     shortHash(oldHash),
		NewBinary:     shortHash(newHash),
		OldCommit:     buildinfo.Read().Revision,
	}
	b, err := json.Marshal(row)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(ledger, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// keepAliveRelaunches reads a launchd plist's KeepAlive and reports whether
// it is a dict and whether launchd relaunches the job after it exits with
// code: KeepAlive true relaunches on any exit; the dict
// {SuccessfulExit: false} relaunches only on a non-zero exit.
func keepAliveRelaunches(plist string, code int) (dict, relaunch bool, err error) {
	dec := xml.NewDecoder(strings.NewReader(plist))
	dec.Strict = false
	var (
		depth       int
		lastKey     string
		inKeepAlive bool
		kaDepth     int
		dictKey     string
		successful  = true // launchd's default when SuccessfulExit is absent
	)
	for {
		tok, terr := dec.Token()
		if terr == io.EOF {
			break
		}
		if terr != nil {
			return false, false, terr
		}
		switch el := tok.(type) {
		case xml.StartElement:
			depth++
			name := el.Name.Local
			switch {
			case !inKeepAlive && lastKey == "KeepAlive" && depth == 3:
				switch name {
				case "true":
					return false, true, nil
				case "false":
					return false, false, nil
				case "dict":
					inKeepAlive, kaDepth, dict = true, depth, true
				}
			case inKeepAlive && depth == kaDepth+1 && dictKey == "SuccessfulExit" && (name == "true" || name == "false"):
				successful = name == "true"
			case !inKeepAlive && name == "key" && depth == 3:
				var k string
				if derr := dec.DecodeElement(&k, &el); derr != nil {
					return false, false, derr
				}
				depth--
				lastKey = k
				continue
			case inKeepAlive && name == "key" && depth == kaDepth+1:
				var k string
				if derr := dec.DecodeElement(&k, &el); derr != nil {
					return false, false, derr
				}
				depth--
				dictKey = k
				continue
			}
			if !inKeepAlive && depth == 3 && name != "key" {
				lastKey = ""
			}
		case xml.EndElement:
			if inKeepAlive && depth == kaDepth {
				// {SuccessfulExit: false}: relaunch on a non-zero exit only;
				// {SuccessfulExit: true}: relaunch on a zero exit only.
				if successful {
					return true, code == 0, nil
				}
				return true, code != 0, nil
			}
			depth--
		}
	}
	if dict {
		return true, false, fmt.Errorf("unterminated KeepAlive dict")
	}
	return false, false, fmt.Errorf("plist has no KeepAlive")
}
