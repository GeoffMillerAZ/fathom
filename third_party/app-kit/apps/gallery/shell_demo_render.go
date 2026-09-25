package gallery

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/appearance"
	"github.com/geoffmilleraz/app-kit/appkit/ui/dashboard"
	"github.com/geoffmilleraz/app-kit/appkit/ui/form"
)

//go:embed shell_demo.js
var shellDemoJS string

func registerShellDemoAssets(mux *http.ServeMux) {
	registerShellLiveAssets(mux)
	mux.HandleFunc("/static/shell-demo.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
		w.Header().Set("ETag", fmt.Sprintf(`"%x"`, sha256.Sum256([]byte(shellDemoJS))))
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", 405)
			return
		}
		http.ServeContent(w, r, "shell-demo.js", time.Time{}, strings.NewReader(shellDemoJS))
	})
}

type shellDemoRenderData struct {
	shellDemoView
	Title, Action, SubmitLabel, CurrentURL, Baseline, Diagnostics, DraftSummary string
	CanEdit, CanView, LiveObserver                                              bool
	DisplayName, TimeZone, StartPage, Digest, Appearance                        template.HTML
	Account                                                                     dashboard.AccountDestinations
	AccountNav, Feedback                                                        template.HTML
	FormSections                                                                template.HTML
	Shell                                                                       template.HTML
	Bootstrap                                                                   template.JS
}

func shellDemoURL(page string) string {
	if page == "home" || page == "" {
		return "/shell"
	}
	return "/shell/" + page
}

func renderShellDemo(v shellDemoView) ([]byte, error) {
	title := map[string]string{"home": "Overview", "activity": "Activity", "profile": "Profile", "settings": "Settings", "help": "Help", "about": "About"}[v.Page]
	if title == "" {
		title = "Application shell"
	}
	d := shellDemoRenderData{shellDemoView: v, Title: title, CurrentURL: shellDemoURL(v.Page), SubmitLabel: "Save changes", CanEdit: v.State.SignedIn && v.State.Access == "editor" && v.State.Availability == "ready", CanView: v.State.SignedIn && v.State.Availability == "ready"}
	d.Account = dashboard.AccountDestinations{
		Profile:  dashboard.NavItem{Href: "/shell/profile", Label: "Profile"},
		Settings: dashboard.NavItem{Href: "/shell/settings", Label: "Settings"},
		Help:     dashboard.NavItem{Href: "/shell/help", Label: "Help"},
		About:    dashboard.NavItem{Href: "/shell/about", Label: "About"},
	}
	d.AccountNav = template.HTML(dashboard.RenderNav([]dashboard.NavGroup{d.Account.Group("")}, d.CurrentURL))
	// The handler determines the outcome. The kit only presents the explicit kind;
	// it cannot infer authorization, persistence or retry policy from HTTP codes.
	feedback := dashboard.Notice{Body: v.Notice}
	if v.Status >= 400 {
		feedback.Kind = dashboard.NoticeError
	}
	if v.Status == 409 {
		feedback.Kind, feedback.Title = dashboard.NoticeConflict, "Review a newer saved version"
	}
	d.Feedback = template.HTML(dashboard.RenderNotice(feedback))
	d.LiveObserver = v.Page == "activity" && v.State.SignedIn
	if v.Status == 409 {
		d.SubmitLabel = "Apply reviewed draft"
	}
	value := func(key, fallback string) string {
		if v.Fields != nil {
			return v.Fields[key]
		}
		return fallback
	}
	textField := func(key, label, saved, help string) template.HTML {
		id := "shell-" + key
		err := v.Errors[key]
		// A history reload must not combine restored edits with a fresh hidden
		// version. This native hint also applies without the draft enhancement;
		// actual browser history behavior is covered by the opt-in history probe.
		control := form.RenderInput(form.Input{ID: id, Name: key, Value: value(key, saved), Autocomplete: "off", Required: true, Disabled: !d.CanEdit, Invalid: err != "", HasHelp: help != ""})
		return template.HTML(form.RenderField(form.Field{Label: label, For: id, Help: help, Error: err, Required: true, Control: template.HTML(control)}))
	}
	selectField := func(key, label, saved, help string, options []form.Option) template.HTML {
		id, selected, valid := "shell-"+key, value(key, saved), false
		for _, o := range options {
			if selected == o.Value {
				valid = true
			}
		}
		if !valid {
			options = append([]form.Option{{Value: selected, Label: "Choose a valid value", Disabled: true}}, options...)
		}
		err := v.Errors[key]
		control := form.RenderSelect(form.Select{ID: id, Name: key, Value: selected, Options: options, Required: true, Disabled: !d.CanEdit, Invalid: err != "", HasHelp: true})
		return template.HTML(form.RenderField(form.Field{Label: label, For: id, Help: help, Error: err, Required: true, Control: template.HTML(control)}))
	}
	d.DisplayName = textField("display_name", "Display name", v.State.DisplayName, "Use 1 to 80 characters. This changes only the synthetic profile.")
	d.TimeZone = selectField("timezone", "Time zone", v.State.TimeZone, "Used for the sample account preference, not your device clock.", []form.Option{{Value: "America/Phoenix", Label: "Phoenix (America/Phoenix)"}, {Value: "UTC", Label: "Coordinated Universal Time (UTC)"}, {Value: "Europe/London", Label: "London (Europe/London)"}})
	d.StartPage = selectField("start_page", "Starting page", v.State.StartPage, "The Open workspace action on Overview uses this saved choice.", []form.Option{{Value: "home", Label: "Overview"}, {Value: "activity", Label: "Activity"}})
	digest := v.State.Digest
	if v.Fields != nil {
		digest = v.Fields["digest"] == "on"
	}
	d.Digest = template.HTML(form.RenderCheckbox(form.Checkbox{ID: "shell-digest", Name: "digest", Value: "on", Label: "Include me in the sample weekly digest", Checked: digest, Disabled: !d.CanEdit, Invalid: v.Errors["digest"] != ""}))
	baseline := map[string]any{"display_name": v.State.DisplayName}
	d.Action = "save-profile"
	if v.Page == "settings" {
		d.Action = "save-settings"
		baseline = map[string]any{"timezone": v.State.TimeZone, "start_page": v.State.StartPage, "digest": v.State.Digest}
	}
	b, _ := json.Marshal(baseline)
	d.Baseline = string(b)
	if v.Fields != nil {
		if v.Page == "profile" {
			d.DraftSummary = "Display name: " + v.Fields["display_name"]
		} else if v.Page == "settings" {
			d.DraftSummary = fmt.Sprintf("Time zone: %s\nStarting page: %s\nDigest submitted value: %s", v.Fields["timezone"], v.Fields["start_page"], v.Fields["digest"])
		}
	}
	d.Appearance = template.HTML(appearance.RenderControl(appearance.Options{StorageKey: "appkit-gallery-shell-theme"}))
	bootstrap := appearance.BootstrapScript(appearance.Options{StorageKey: "appkit-gallery-shell-theme"})
	if v.Theme == "light" || v.Theme == "dark" {
		b, _ := json.Marshal(v.Theme)
		bootstrap += "\nwindow.AppKitAppearance.set(" + string(b) + ");\n"
	}
	d.Bootstrap = template.JS(bootstrap) // Kit-authored script, JSON-escaped configuration only.
	d.Diagnostics = fmt.Sprintf("App-kit shell laboratory\nBuild: development working tree\nGo: %s\nRuntime: Go/HTTP synthetic fixture\nPersistence: bounded process memory; not a real account\nBrowser/desktop conformance: not established\n", runtime.Version())
	var sectionErr error
	d.FormSections, sectionErr = renderShellDemoSections(d)
	if sectionErr != nil {
		return nil, sectionErr
	}
	var content, actions bytes.Buffer
	if err := shellDemoTemplates.ExecuteTemplate(&content, "content", d); err != nil {
		return nil, err
	}
	if err := shellDemoTemplates.ExecuteTemplate(&actions, "actions", d); err != nil {
		return nil, err
	}
	groups := []dashboard.NavGroup{{Heading: "Workspace", Items: []dashboard.NavItem{{Href: "/shell", Label: "Overview"}, {Href: "/shell/activity", Label: "Activity"}}}, d.Account.Group("Your account")}
	d.Shell = template.HTML(dashboard.RenderShell(dashboard.Shell{Title: title, Brand: `<a href="/catalog" class="font-semibold text-accent underline-offset-4 hover:underline">app-kit laboratory</a>`, Nav: template.HTML(dashboard.RenderNav(groups, d.CurrentURL)), Actions: template.HTML(actions.String()), Content: template.HTML(content.String()), ContentID: "shell-content", SkipLink: true}))
	var out bytes.Buffer
	if err := shellDemoTemplates.ExecuteTemplate(&out, "document", d); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// The page chooses content and state. The kit owns only the repeated section
// hierarchy, responsive columns and separators; it does not wrap or submit forms.
func renderShellDemoSections(d shellDemoRenderData) (template.HTML, error) {
	var sections []form.SettingsSection
	var content bytes.Buffer
	switch d.Page {
	case "profile":
		if err := shellDemoTemplates.ExecuteTemplate(&content, "identity-content", d); err != nil {
			return "", err
		}
		sections = []form.SettingsSection{
			{HeadingID: "shell-personal-title", Title: "Personal details", Description: "A readable name helps people recognize your work. No photo upload or external avatar service is needed for this task.", Content: d.DisplayName},
			{HeadingID: "shell-identity-title", Title: "Sign-in identity", Description: "Managed by the app's identity provider. This sample address cannot receive mail.", Content: template.HTML(content.String())},
		}
	case "settings":
		if err := shellDemoTemplates.ExecuteTemplate(&content, "digest-content", d); err != nil {
			return "", err
		}
		sections = []form.SettingsSection{
			{HeadingID: "shell-locale-title", Title: "Region", Description: "Save an explicit preference. A real app must apply it consistently to its own dates and data.", Content: d.TimeZone},
			{HeadingID: "shell-start-title", Title: "Starting point", Description: "Choose where Open workspace takes you. All navigation destinations in this specimen work.", Content: d.StartPage},
			{HeadingID: "shell-digest-title", Title: "Digest preference", Description: "Demonstrates a persisted opt-in. This laboratory never sends email or schedules a delivery.", Content: template.HTML(content.String())},
		}
	default:
		return "", nil
	}
	markup, err := form.RenderSettingsSections(sections)
	return template.HTML(markup), err // Reviewed kit output; template fragments escaped above.
}

var shellDemoTemplates = template.Must(template.New("shell-demo").Funcs(template.FuncMap{"shellURL": shellDemoURL}).Parse(shellDemoHTML))
