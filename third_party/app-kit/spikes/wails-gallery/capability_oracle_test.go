//go:build darwin && !server && capabilityprobe && !workflowprobe && appkit_contained

package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func capabilitySyntheticReceipt() capabilityReceipt {
	r := capabilityReceipt{Schema: 1, Profile: "macos-http-only-media", Go: "go1.26.3", OS: "darwin", Arch: "arm64", Shutdown: true, ListenerClosed: true, WindowsClosed: 3}
	for i, id := range capabilityIDs {
		c := capabilityCase{
			ID:       id,
			Presence: capabilityPresence{Fixture: "appkit-media-denial-v1", UserAgent: "Synthetic AppleWebKit/617", Secure: true, Media: true, NotificationPermission: "unavailable", Visible: true, Focused: true},
			Before:   capabilityNativeObservation{Available: true, OriginConfigured: true, Denials: [3]uint64{1, 1, 1}, PreflightDenied: [3]bool{true, true, true}, PreflightCalls: [3]uint64{1, 1, 1}, PreflightResults: [3]int64{2, 2, 2}, WindowVisible: true, WindowKey: true, ApplicationActive: true},
			After:    capabilityNativeObservation{Available: true, OriginConfigured: true, Denials: [3]uint64{1, 1, 1}, OriginMatched: true, PreflightResults: [3]int64{-1, -1, -1}},
			Page:     capabilityPageResult{Started: true, Done: true, Result: "NotAllowedError"},
		}
		c.After.Denials[i]++
		r.Cases = append(r.Cases, c)
	}
	return r
}

func TestCapabilityReceiptRequiresActualNativeDenial(t *testing.T) {
	if err := validateCapabilityReceipt(capabilitySyntheticReceipt()); err != nil {
		t.Fatal(err)
	}
	canaries := map[string]func(*capabilityReceipt){
		"no case":                  func(r *capabilityReceipt) { r.Cases = r.Cases[:2] },
		"extra case":               func(r *capabilityReceipt) { r.Cases = append(r.Cases, r.Cases[0]) },
		"reordered":                func(r *capabilityReceipt) { r.Cases[0], r.Cases[1] = r.Cases[1], r.Cases[0] },
		"failed case":              func(r *capabilityReceipt) { r.Cases[0].Failure = "failure" },
		"no fixture":               func(r *capabilityReceipt) { r.Cases[0].Presence.Fixture = "" },
		"no media API":             func(r *capabilityReceipt) { r.Cases[0].Presence.Media = false },
		"insecure":                 func(r *capabilityReceipt) { r.Cases[0].Presence.Secure = false },
		"page hidden":              func(r *capabilityReceipt) { r.Cases[0].Presence.Visible = false },
		"page unfocused":           func(r *capabilityReceipt) { r.Cases[0].Presence.Focused = false },
		"window hidden":            func(r *capabilityReceipt) { r.Cases[0].Before.WindowVisible = false },
		"window not key":           func(r *capabilityReceipt) { r.Cases[0].Before.WindowKey = false },
		"application inactive":     func(r *capabilityReceipt) { r.Cases[0].Before.ApplicationActive = false },
		"no native callback":       func(r *capabilityReceipt) { r.Cases[0].Before.Available = false },
		"wrong origin configured":  func(r *capabilityReceipt) { r.Cases[0].Before.OriginConfigured = false },
		"preflight prompt":         func(r *capabilityReceipt) { r.Cases[0].Before.PreflightDenied[0] = false },
		"raw preflight grant":      func(r *capabilityReceipt) { r.Cases[0].Before.PreflightResults[0] = 1 },
		"raw preflight absent":     func(r *capabilityReceipt) { r.Cases[0].Before.PreflightCalls[0] = 0 },
		"preflight counter absent": func(r *capabilityReceipt) { r.Cases[0].Before.Denials[0] = 0 },
		"nil origin matched":       func(r *capabilityReceipt) { r.Cases[0].Before.OriginMatched = true },
		"rejected before delegate": func(r *capabilityReceipt) { r.Cases[0].After.Denials[0] = 1 },
		"wrong callback type":      func(r *capabilityReceipt) { r.Cases[0].After.Denials = [3]uint64{1, 2, 1} },
		"duplicate callback":       func(r *capabilityReceipt) { r.Cases[0].After.Denials[0] = 3 },
		"wrong requesting origin":  func(r *capabilityReceipt) { r.Cases[0].After.OriginMatched = false },
		"missing final observer":   func(r *capabilityReceipt) { r.Cases[0].After.Available = false },
		"final preflight forged":   func(r *capabilityReceipt) { r.Cases[0].After.PreflightDenied[0] = true },
		"never started":            func(r *capabilityReceipt) { r.Cases[0].Page.Started = false },
		"timeout":                  func(r *capabilityReceipt) { r.Cases[0].Page.Done = false },
		"missing hardware":         func(r *capabilityReceipt) { r.Cases[0].Page.Result = "NotFoundError" },
		"granted":                  func(r *capabilityReceipt) { r.Cases[0].Page.Result = "granted" },
		"returned track":           func(r *capabilityReceipt) { r.Cases[0].Page.Tracks = 1 },
		"bad presence":             func(r *capabilityReceipt) { r.Cases[0].Presence.NotificationPermission = "denied" },
		"shutdown missing":         func(r *capabilityReceipt) { r.Shutdown = false },
		"listener live":            func(r *capabilityReceipt) { r.ListenerClosed = false },
		"window remains":           func(r *capabilityReceipt) { r.WindowsClosed = 2 },
		"framework error":          func(r *capabilityReceipt) { r.FrameworkErrors = 1 },
		"framework warning":        func(r *capabilityReceipt) { r.FrameworkWarnings = 1 },
		"bridge message":           func(r *capabilityReceipt) { r.BridgeMessages = 1 },
		"failure":                  func(r *capabilityReceipt) { r.Failure = "failure" },
		"diagnostic is not full":   func(r *capabilityReceipt) { r.DiagnosticCase = "camera" },
	}
	for name, change := range canaries {
		t.Run(name, func(t *testing.T) {
			r := capabilitySyntheticReceipt()
			change(&r)
			if validateCapabilityReceipt(r) == nil {
				t.Fatal("false pass")
			}
		})
	}
}

func TestCapabilityNativeAcknowledgementUsesJSONBooleans(t *testing.T) {
	if !capabilityWireContract() {
		t.Fatal("native preflight must emit true/false booleans, not numeric 1/0")
	}
}

func TestCapabilityReceiptFraming(t *testing.T) {
	data, err := json.Marshal(capabilitySyntheticReceipt())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := readCapabilityReceipt(strings.NewReader(string(data))); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"", "{}", string(data) + "{}", string(data) + "garbage", strings.Replace(string(data), `"schema":1`, `"unknown":1,"schema":1`, 1), strings.Repeat(" ", 65<<10)} {
		if _, err := readCapabilityReceipt(strings.NewReader(input)); err == nil {
			t.Fatal("malformed receipt accepted")
		}
	}
}

func TestCapabilityScriptsDoNotRequestUnselectedAccess(t *testing.T) {
	for _, forbidden := range []string{"getCurrentPosition(", "watchPosition(", "requestPermission(", ".readText(", ".read(", "enumerateDevices(", "deviceId"} {
		if strings.Contains(capabilityPresenceScript+capabilityStartScript, forbidden) {
			t.Fatalf("unselected request: %s", forbidden)
		}
	}
	if !strings.Contains(capabilityStartScript, "tracks.forEach(t=>t.stop())") {
		t.Fatal("unexpected capture must stop immediately")
	}
}
