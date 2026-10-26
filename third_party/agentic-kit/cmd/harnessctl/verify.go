package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const payloadType = "application/vnd.agentic-kit.harness-layer.v1+json"

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
var hashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func validID(s string) bool { return idPattern.MatchString(s) }
func uniqueIDs(a []string, min, max int) bool {
	if len(a) < min || len(a) > max {
		return false
	}
	m := map[string]bool{}
	for _, s := range a {
		if !validID(s) || m[s] {
			return false
		}
		m[s] = true
	}
	return true
}
func parseTime(s string) (time.Time, error) {
	t, e := time.Parse("2006-01-02T15:04:05Z", s)
	if e != nil || t.Format("2006-01-02T15:04:05Z") != s {
		return time.Time{}, errClock
	}
	return t, nil
}
func interval(issued, expires string, now time.Time, max time.Duration) (time.Time, time.Time, error) {
	a, e := parseTime(issued)
	if e != nil {
		return a, a, e
	}
	b, e := parseTime(expires)
	if e != nil {
		return a, a, e
	}
	if b.Before(a) || b.Equal(a) || b.Sub(a) > max || now.Before(a) || !now.Before(b) {
		return a, b, errClock
	}
	return a, b, nil
}
func validRange(min, max int) bool { return min >= 0 && max <= 1000000 && min <= max }
func validConstraint(c constraint) bool {
	return validID(c.ID) && validRange(c.MinSamplePPM, c.MaxSamplePPM) && c.MaxWorkMillis >= 1 && c.MaxWorkMillis <= 30000 && (c.OnFault == "hold" || c.OnFault == "warn") && uniqueIDs(c.TunableBy, 0, 32)
}
func validRoot(c control) bool {
	return validID(c.ID) && oneOf(c.Event, "before-tool", "after-tool", "session-start", "session-stop", "pre-commit", "pre-push") && oneOf(c.Class, "mandatory", "advisory") && validRange(c.MinSamplePPM, c.MaxSamplePPM) && c.DefaultSamplePPM >= c.MinSamplePPM && c.DefaultSamplePPM <= c.MaxSamplePPM && c.MaxWorkMillis >= 1 && c.MaxWorkMillis <= 30000 && oneOf(c.OnFault, "hold", "warn") && uniqueIDs(c.TunableBy, 0, 32)
}
func oneOf(s string, choices ...string) bool {
	for _, x := range choices {
		if s == x {
			return true
		}
	}
	return false
}
func mandatory(min, max, def int, fault string, maint bool, tuners []string) bool {
	return min == 1000000 && max == 1000000 && def == 1000000 && fault == "hold" && !maint && len(tuners) == 0
}

func validateRegistration(r registration, projectRoot, registrationPath string, now time.Time) (project, map[string]key, map[string]control, error) {
	if r.Version != "harnessctl.registration/1" || r.Epoch < 1 || r.Epoch > 2147483647 || !validID(r.HostID) || len(r.Keys) < 1 || len(r.Keys) > 512 || len(r.Controls) < 1 || len(r.Controls) > 256 || len(r.Projects) < 1 || len(r.Projects) > 256 {
		return project{}, nil, nil, errRegistration
	}
	if _, _, e := interval(r.IssuedAt, r.ExpiresAt, now, 366*24*time.Hour); e != nil {
		return project{}, nil, nil, e
	}
	keys := map[string]key{}
	for _, k := range r.Keys {
		if !validID(k.ID) || keys[k.ID].ID != "" {
			return project{}, nil, nil, errRegistration
		}
		b, e := base64.StdEncoding.DecodeString(k.PublicKey)
		if e != nil || len(b) != ed25519.PublicKeySize || base64.StdEncoding.EncodeToString(b) != k.PublicKey {
			return project{}, nil, nil, errRegistration
		}
		keys[k.ID] = k
	}
	controls := map[string]control{}
	for _, c := range r.Controls {
		if !validRoot(c) || controls[c.ID].ID != "" {
			return project{}, nil, nil, errRegistration
		}
		if c.Class == "mandatory" && !mandatory(c.MinSamplePPM, c.MaxSamplePPM, c.DefaultSamplePPM, c.OnFault, c.AllowMaintenance, c.TunableBy) {
			return project{}, nil, nil, errPolicy
		}
		controls[c.ID] = c
	}
	roots := []os.FileInfo{}
	rootNames := []string{}
	pids := map[string]bool{}
	var selected project
	found := false
	for _, p := range r.Projects {
		if !validID(p.ID) || pids[p.ID] || len(p.Root) > 4096 || !filepath.IsAbs(p.Root) || filepath.Clean(p.Root) != p.Root || !validID(p.PrincipalID) || !uniqueIDs(p.TeamIDs, 0, 16) || !uniqueIDs(p.ControlIDs, 1, 64) || len(p.Layers) < 1 || len(p.Layers) > 32 {
			return project{}, nil, nil, errRegistration
		}
		for _, name := range rootNames {
			if strings.EqualFold(name, p.Root) {
				return project{}, nil, nil, errRegistration
			}
		}
		rootNames = append(rootNames, p.Root)
		rootInfo, e := os.Lstat(p.Root)
		if e == nil {
			if !canonical(p.Root) || !rootInfo.IsDir() {
				return project{}, nil, nil, errRegistration
			}
			for _, other := range roots {
				if os.SameFile(rootInfo, other) {
					return project{}, nil, nil, errRegistration
				}
			}
			roots = append(roots, rootInfo)
		} else if !os.IsNotExist(e) || p.Root == projectRoot {
			return project{}, nil, nil, errRegistration
		}
		if p.OrganizationID != nil && !validID(*p.OrganizationID) {
			return project{}, nil, nil, errRegistration
		}
		for _, id := range p.ControlIDs {
			if controls[id].ID == "" {
				return project{}, nil, nil, errRegistration
			}
		}
		seenLayers := map[string]bool{}
		seenPriority := map[int]bool{}
		for _, ref := range p.Layers {
			switch ref.Kind {
			case "installation", "organization", "team", "project", "personal":
			default:
				return project{}, nil, nil, errRegistration
			}
			if !validID(ref.ID) || seenLayers[ref.ID] || !validID(ref.TargetID) || len(ref.Path) > 4096 || !filepath.IsAbs(ref.Path) || filepath.Clean(ref.Path) != ref.Path || !validID(ref.KeyID) || keys[ref.KeyID].ID == "" || ref.Revision < 1 || ref.Revision > 2147483647 || !hashPattern.MatchString(ref.PayloadSHA256) || ref.MaxAgeSeconds < 1 || ref.MaxAgeSeconds > 2592000 || ref.Priority < 0 || ref.Priority > 31 || seenPriority[ref.Priority] || !uniqueIDs(ref.ControlIDs, 1, 64) {
				return project{}, nil, nil, errRegistration
			}
			seenLayers[ref.ID] = true
			seenPriority[ref.Priority] = true
			for _, id := range ref.ControlIDs {
				if controls[id].ID == "" || !contains(p.ControlIDs, id) {
					return project{}, nil, nil, errRegistration
				}
			}
		}
		pids[p.ID] = true
		if p.Root == projectRoot {
			selected = p
			found = true
		}
	}
	if !found {
		return project{}, nil, nil, errProject
	}
	if within(registrationPath, projectRoot) {
		return project{}, nil, nil, errRegistration
	}
	for _, id := range selected.ControlIDs {
		if controls[id].ID == "" {
			return project{}, nil, nil, errRegistration
		}
	}
	ids := map[string]bool{}
	priorities := map[int]bool{}
	coverage := map[string]bool{}
	installation := 0
	organizations := map[string]bool{}
	teams := map[string]bool{}
	for _, t := range selected.TeamIDs {
		teams[t] = false
	}
	for _, ref := range selected.Layers {
		if !validID(ref.ID) || ids[ref.ID] || !validID(ref.TargetID) || len(ref.Path) > 4096 || !filepath.IsAbs(ref.Path) || filepath.Clean(ref.Path) != ref.Path || !validID(ref.KeyID) || keys[ref.KeyID].ID == "" || ref.Revision < 1 || ref.Revision > 2147483647 || !hashPattern.MatchString(ref.PayloadSHA256) || ref.MaxAgeSeconds < 1 || ref.MaxAgeSeconds > 2592000 || ref.Priority < 0 || ref.Priority > 31 || priorities[ref.Priority] || !uniqueIDs(ref.ControlIDs, 1, 64) {
			return project{}, nil, nil, errRegistration
		}
		ids[ref.ID] = true
		priorities[ref.Priority] = true
		switch ref.Kind {
		case "installation":
			if ref.TargetID != r.HostID {
				return project{}, nil, nil, errRegistration
			}
			installation++
		case "organization":
			if selected.OrganizationID == nil || ref.TargetID != *selected.OrganizationID || organizations[ref.TargetID] {
				return project{}, nil, nil, errRegistration
			}
			organizations[ref.TargetID] = true
		case "team":
			if _, ok := teams[ref.TargetID]; !ok || teams[ref.TargetID] {
				return project{}, nil, nil, errRegistration
			}
			teams[ref.TargetID] = true
		case "project":
			if ref.TargetID != selected.ID {
				return project{}, nil, nil, errRegistration
			}
		case "personal":
			if ref.TargetID != selected.PrincipalID {
				return project{}, nil, nil, errRegistration
			}
		default:
			return project{}, nil, nil, errRegistration
		}
		for _, id := range ref.ControlIDs {
			if controls[id].ID == "" || !contains(selected.ControlIDs, id) {
				return project{}, nil, nil, errRegistration
			}
			coverage[id] = true
		}
	}
	if installation != 1 || (selected.OrganizationID != nil && !organizations[*selected.OrganizationID]) {
		return project{}, nil, nil, errRegistration
	}
	for _, v := range teams {
		if !v {
			return project{}, nil, nil, errRegistration
		}
	}
	for _, id := range selected.ControlIDs {
		if !coverage[id] {
			return project{}, nil, nil, errRegistration
		}
		for _, t := range controls[id].TunableBy {
			if !ids[t] {
				return project{}, nil, nil, errRegistration
			}
		}
	}
	return selected, keys, controls, nil
}
func contains(a []string, s string) bool {
	for _, x := range a {
		if x == s {
			return true
		}
	}
	return false
}

func decodeB64(s string) ([]byte, error) {
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		b, e := enc.DecodeString(s)
		if e == nil && enc.EncodeToString(b) == s {
			return b, nil
		}
	}
	return nil, errSource
}
func verifyLayer(raw []byte, ref layerRef, k key, now time.Time) (layer, [32]byte, error) {
	var env envelope
	var zero [32]byte
	if strictJSON(raw, &env) != nil || env.PayloadType != payloadType || len(env.Signatures) != 1 {
		return layer{}, zero, errEnvelope
	}
	sig := env.Signatures[0]
	if len(sig.KeyID) > 128 {
		return layer{}, zero, errEnvelope
	}
	for _, ch := range sig.KeyID {
		if ch < 0x20 || ch > 0x7e {
			return layer{}, zero, errEnvelope
		}
	}
	payload, e := decodeB64(env.Payload)
	if e != nil || len(payload) > 32768 {
		return layer{}, zero, errEnvelope
	}
	signature, e := decodeB64(sig.Sig)
	if e != nil || len(signature) != ed25519.SignatureSize {
		return layer{}, zero, errEnvelope
	}
	h := sha256.Sum256(payload)
	if hex.EncodeToString(h[:]) != ref.PayloadSHA256 {
		return layer{}, zero, errBinding
	}
	if k.Revoked {
		return layer{}, zero, errSignature
	}
	pk, e := base64.StdEncoding.DecodeString(k.PublicKey)
	if e != nil {
		return layer{}, zero, errEnvelope
	}
	pae := []byte("DSSEv1 " + strconv.Itoa(len(payloadType)) + " " + payloadType + " " + strconv.Itoa(len(payload)) + " ")
	pae = append(pae, payload...)
	if !ed25519.Verify(ed25519.PublicKey(pk), pae, signature) {
		return layer{}, zero, errSignature
	}
	var l layer
	if strictJSON(payload, &l) != nil {
		return layer{}, zero, errEnvelope
	}
	if l.Version != "harnessctl.layer/1" || l.ID != ref.ID || l.Kind != ref.Kind || l.TargetID != ref.TargetID || l.Revision != ref.Revision || len(l.Controls) != len(ref.ControlIDs) || len(l.Preferences) > 64 {
		return layer{}, zero, errBinding
	}
	issued, _, e := interval(l.IssuedAt, l.ExpiresAt, now, 366*24*time.Hour)
	if e != nil || now.Sub(issued) >= time.Duration(ref.MaxAgeSeconds)*time.Second {
		return layer{}, zero, errClock
	}
	seen := map[string]bool{}
	for _, c := range l.Controls {
		if !validConstraint(c) || seen[c.ID] {
			return layer{}, zero, errEnvelope
		}
		if !contains(ref.ControlIDs, c.ID) {
			return layer{}, zero, errBinding
		}
		seen[c.ID] = true
	}
	seen = map[string]bool{}
	for _, p := range l.Preferences {
		if seen[p.ControlID] || p.SamplePPM < 0 || p.SamplePPM > 1000000 {
			return layer{}, zero, errEnvelope
		}
		if !contains(ref.ControlIDs, p.ControlID) {
			return layer{}, zero, errBinding
		}
		seen[p.ControlID] = true
	}
	return l, h, nil
}

func snapshotDigest(regHash [32]byte, projectID string, refs []layerRef) (string, error) {
	if len(projectID) > 64 || len(refs) > 32 {
		return "", errSource
	}
	b := []byte("agentic-kit/harnessctl/snapshot/1\x00")
	b = append(b, regHash[:]...)
	b = appendU32(b, len(projectID))
	b = append(b, projectID...)
	b = appendU32(b, len(refs))
	sort.Slice(refs, func(i, j int) bool { return refs[i].ID < refs[j].ID })
	for _, r := range refs {
		h, e := hex.DecodeString(r.PayloadSHA256)
		if e != nil || len(h) != 32 {
			return "", errSource
		}
		b = appendU32(b, len(r.ID))
		b = append(b, r.ID...)
		b = append(b, h...)
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}
func appendU32(b []byte, n int) []byte {
	return append(b, byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
}
