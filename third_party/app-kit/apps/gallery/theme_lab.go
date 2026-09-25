package gallery

import (
	"context"
	"html/template"
	"io"

	"github.com/geoffmilleraz/app-kit/appkit/ui/tokens"
	"github.com/geoffmilleraz/app-kit/apps/styleguide"
)

// NewThemeLab mounts the experimental app-owned guide using Gallery's existing
// renderers. Inputs are explicit trusted source and exact local build identity;
// no licensed corpus or website content enters this tool.
func NewThemeLab(origin string, source []byte, sourceLabel, kitRevision string) (*styleguide.Handler, error) {
	checks := []styleguide.ContrastCheck{}
	for _, scheme := range []string{"light", "dark"} {
		for _, pair := range []struct {
			id, fg, bg string
			minimum    float64
		}{
			{"body", "ink", "surface", tokens.TextFloor},
			{"secondary", "ink-muted", "surface", tokens.TextFloor},
			{"action", "accent-ink", "accent", tokens.TextFloor},
			{"focus", "focus", "surface", tokens.MeaningFloor},
		} {
			checks = append(checks, styleguide.ContrastCheck{ID: pair.id + "-" + scheme, Foreground: pair.fg, Background: pair.bg, Scheme: scheme, Minimum: pair.minimum})
		}
	}
	engine, err := styleguide.NewEngine(styleguide.EngineOptions{Source: source, SourceLabel: sourceLabel, KitRevision: kitRevision, DarkVariant: tokens.DarkVariantAttribute, Checks: checks, Fonts: map[string]string{
		"system-sans": "system-ui, sans-serif", "system-mono": "ui-monospace, monospace", "system-serif": "ui-serif, Georgia, serif",
	}})
	if err != nil {
		return nil, err
	}
	specimens := []styleguide.Specimen{}
	for _, s := range []struct {
		id, name string
		build    func() (template.HTML, error)
	}{
		{"dashboard", "Dashboard", buildDashboard}, {"components", "Charts and components", buildComponents}, {"forms", "Form controls", buildForms},
	} {
		specimens = append(specimens, styleguide.Specimen{ID: s.id, Name: s.name, Render: func(_ context.Context, w io.Writer) error {
			body, err := s.build()
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, string(body))
			return err
		}})
	}
	return styleguide.New(styleguide.Options{Engine: engine, BasePath: "/theme-lab", Origin: origin, Title: "Gallery theme lab", Stylesheets: []string{"/static/app.css"}, Specimens: specimens})
}
