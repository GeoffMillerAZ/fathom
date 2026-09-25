package gallery

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

// This state belongs only to the synthetic Gallery journey. It is not an
// account schema, identity provider, persistence API or consumer permission model.
type shellDemoState struct {
	DisplayName  string
	TimeZone     string
	StartPage    string
	Digest       bool
	Version      uint64
	Access       string
	Availability string
	SignedIn     bool
	FailNextSave bool
	Activity     []string
}

type shellDemoView struct {
	Page       string
	State      shellDemoState
	CSRF       string
	Nonce      string
	Theme      string
	Fields     map[string]string
	Errors     map[string]string
	Notice     string
	NoticeRole string
	Status     int
}

func defaultShellDemoState(version uint64) shellDemoState {
	return shellDemoState{DisplayName: "Alex Morgan", TimeZone: "America/Phoenix", StartPage: "home",
		Digest: true, Version: version, Access: "editor", Availability: "ready", SignedIn: true,
		Activity: []string{"Started a synthetic session. No real account was accessed."}}
}

func copyShellDemoState(state shellDemoState) shellDemoState {
	state.Activity = append([]string{}, state.Activity...)
	return state
}

func shellDemoPage(page string) bool {
	switch page {
	case "home", "activity", "profile", "settings", "help", "about":
		return true
	}
	return false
}

func shellDemoPath(page string) string {
	if page == "home" {
		return "/shell"
	}
	return "/shell/" + page
}

type shellDemoOutcome struct {
	Page, Notice, NoticeRole string
	Status                   int
	Fields, Errors           map[string]string
}

func shellDemoGET(state shellDemoState, page string) shellDemoOutcome {
	out := shellDemoOutcome{Page: page, Status: http.StatusOK}
	if page != "profile" && page != "settings" {
		return out
	}
	switch {
	case !state.SignedIn:
		out.Status, out.Notice = http.StatusUnauthorized, "This synthetic session is signed out. Sign in to the demo to continue; no real identity provider is involved."
	case state.Availability == "error":
		out.Status, out.Notice = http.StatusServiceUnavailable, "Simulated loading failure. Retry loading to restore the saved demo values."
	case state.Availability == "loading":
		out.Notice = "Loading fixture: no background request is running. Use Retry loading to restore the ready state."
	case state.Access == "viewer":
		out.Notice = "Viewer simulation: saved values are read-only. Change the fixture to Editor to try saving."
	}
	if out.Status >= 400 {
		out.NoticeRole = "danger"
	} else if out.Notice != "" {
		out.NoticeRole = "info"
	}
	return out
}

func shellDemoApply(saved shellDemoState, form url.Values) (shellDemoState, shellDemoOutcome) {
	state := copyShellDemoState(saved)
	action, page := form.Get("action"), form.Get("return_to")
	if page == "" {
		page = "home"
	}
	out := shellDemoOutcome{Page: page, Status: http.StatusSeeOther, NoticeRole: "success"}
	if action == "save-profile" {
		out.Page = "profile"
		out.Fields = map[string]string{"display_name": form.Get("display_name")}
	}
	if action == "save-settings" {
		out.Page = "settings"
		out.Fields = map[string]string{"timezone": form.Get("timezone"), "start_page": form.Get("start_page"), "digest": form.Get("digest")}
	}
	refuse := func(status int, notice string) (shellDemoState, shellDemoOutcome) {
		out.Status, out.Notice, out.NoticeRole = status, notice, "danger"
		return state, out
	}
	if !state.SignedIn && action != "fixture" && action != "signin" && action != "reset-demo" {
		return refuse(http.StatusUnauthorized, "The demo session is signed out. Your submitted draft has not been saved.")
	}
	if action == "save-profile" || action == "save-settings" {
		if state.Access != "editor" {
			return refuse(http.StatusForbidden, "Viewer access cannot save changes. Your submitted draft is retained; saved values are unchanged.")
		}
		if state.Availability != "ready" {
			return refuse(http.StatusServiceUnavailable, "The simulated data source is unavailable. Your submitted draft is retained; retry loading before saving.")
		}
		version, err := strconv.ParseUint(form.Get("version"), 10, 64)
		if err != nil || strconv.FormatUint(version, 10) != form.Get("version") {
			return refuse(http.StatusBadRequest, "The submitted form version is invalid. Reload this page before trying again.")
		}
		if version != state.Version {
			return refuse(http.StatusConflict, "Saved values changed after this form was opened. Your draft is retained. Review the current saved values before applying the draft again.")
		}
		out.Errors = map[string]string{}
		if action == "save-profile" {
			name := strings.TrimSpace(form.Get("display_name"))
			if !utf8.ValidString(name) || utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 80 {
				out.Errors["display_name"] = "Enter a display name between 1 and 80 characters."
			}
		} else {
			if !shellDemoTimeZone(form.Get("timezone")) {
				out.Errors["timezone"] = "Choose UTC, America/Phoenix, or Europe/London."
			}
			if form.Get("start_page") != "home" && form.Get("start_page") != "activity" {
				out.Errors["start_page"] = "Choose Home or Activity."
			}
			if value := form.Get("digest"); value != "" && value != "on" {
				out.Errors["digest"] = "Use the digest checkbox to choose this preference."
			}
		}
		if len(out.Errors) != 0 {
			return refuse(http.StatusUnprocessableEntity, "Some fields need attention. Your submitted draft is retained; nothing was saved.")
		}
		if state.FailNextSave {
			state.FailNextSave = false
			return refuse(http.StatusServiceUnavailable, "The one-shot save failure was simulated. Nothing was saved; retry the same draft to complete the save.")
		}
	}
	// The fixed lifetime and bounded store make overflow unreachable in normal
	// use, but never wrap an optimistic concurrency version into an older value.
	if state.Version == ^uint64(0) {
		return refuse(http.StatusServiceUnavailable, "The demo version limit was reached. Start a new browser session.")
	}
	switch action {
	case "save-profile":
		state.DisplayName = strings.TrimSpace(form.Get("display_name"))
		out.Notice = "Profile saved in this synthetic session only."
	case "save-settings":
		state.TimeZone, state.StartPage = form.Get("timezone"), form.Get("start_page")
		state.Digest = form.Get("digest") == "on"
		out.Notice = "Settings saved in this synthetic session only."
	case "fixture":
		if form.Has("access") {
			state.Access = form.Get("access")
		}
		if form.Has("availability") {
			state.Availability = form.Get("availability")
		}
		state.FailNextSave = form.Get("fail_next_save") == "on"
		out.Notice = "Simulation controls applied. This does not change real authorization or service availability."
	case "retry-load":
		state.Availability = "ready"
		out.Notice = "Simulated loading recovered. Saved values are available again."
	case "signout":
		state.SignedIn = false
		out.Notice = "Signed out of the synthetic demo only. No real account session changed."
	case "signin":
		state.SignedIn = true
		out.Notice = "Signed in to the synthetic demo only. No credentials were requested."
	case "reset-demo":
		state = defaultShellDemoState(state.Version)
		out.Notice = "This browser's synthetic session was reset. Other sessions were not changed."
	default:
		return refuse(http.StatusBadRequest, "Unknown demo action.")
	}
	state.Version++
	state.Activity = append(state.Activity, out.Notice)
	if len(state.Activity) > 10 {
		state.Activity = append([]string{}, state.Activity[len(state.Activity)-10:]...)
	}
	out.Fields, out.Errors = nil, nil
	return state, out
}

func shellDemoTimeZone(value string) bool {
	return value == "UTC" || value == "America/Phoenix" || value == "Europe/London"
}
