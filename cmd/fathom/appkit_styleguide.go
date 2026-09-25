package main

// The theme guide is opt-in and has no path to the live Observatory model.
// Its callbacks reuse the production renderers with fixed synthetic values.

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/app-kit/appkit/ui/tokens"
	"github.com/geoffmilleraz/app-kit/apps/styleguide"
	nethtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

//go:embed ui/guide-baseline.cue
var observatoryGuideSource []byte

const observatoryGuideRevision = "dc77e660e378727cb062fd76e2a8770fd8a4540c"

func observatoryGuideModel(state string) observatoryModel {
	m := observatoryModel{
		SchemaVersion: observatorySchemaVersion,
		FathomVersion: "synthetic-guide",
		GeneratedAt:   "2026-09-21T00:00:00Z",
		Arm:           obsArmBanner{Detail: "Synthetic fixture: no measurement arm"},
		Narrate:       obsArmBanner{Detail: "Synthetic fixture: narrative off"},
	}
	row := obsServiceRow{}
	row.Service = "fixture"
	row.Symbols = 123
	row.Freshness = "fresh"
	row.CallsTotal = 10
	row.CallsAgent = 7
	row.Grade = "A"
	known := 2
	switch state {
	case "complete":
		row.OpenObservations = &known
		m.Impact.VerifyCatchesTotal = &known
		m.Impact.VerifyCatchesNote = "Synthetic verify ledger: 2 sessions."
		m.Impact.LedgerSessions = 2
		m.Impact.DateRange = "Synthetic sessions in September 2026."
		m.Impact.ObligationCalls = 3
		m.Services = []obsServiceRow{row}
	case "partial":
		row.OpenObservations = nil
		m.Services = []obsServiceRow{row}
		m.DegradedNote = "Synthetic registry incomplete; listed services only."
		m.CallLogNote = "Synthetic fixture: call log not read; traffic is unknown."
		m.Impact.LedgerNote = "Synthetic fixture: session ledger not read; session counts are unknown."
	case "missing":
		m.DegradedNote = "Synthetic registry has no service rows; coverage is unknown, never zero."
		m.CallLogNote = "Synthetic fixture: call log not read; traffic is unknown."
		m.Impact.LedgerNote = "Synthetic fixture: session ledger not read; session counts are unknown."
	case "error":
		m.DegradedNote = "Synthetic registry read failed; service and fleet measurements are unavailable."
		m.CallLogNote = "Synthetic call-log read failed; traffic and cost are unavailable."
		m.Impact.LedgerNote = "Synthetic session-ledger read failed; session counts are unavailable."
	}
	return m
}

func newObservatoryThemeGuide(origin string) (*styleguide.Handler, error) {
	checks := make([]styleguide.ContrastCheck, 0, 8)
	for _, scheme := range []string{"light", "dark"} {
		for _, pair := range []struct {
			id, foreground, background string
			minimum                    float64
		}{
			{"body", "ink", "surface", tokens.TextFloor},
			{"secondary", "ink-muted", "surface", tokens.TextFloor},
			{"action", "accent-ink", "accent", tokens.TextFloor},
			{"focus", "focus", "surface", tokens.MeaningFloor},
		} {
			checks = append(checks, styleguide.ContrastCheck{ID: pair.id + "-" + scheme, Foreground: pair.foreground, Background: pair.background, Scheme: scheme, Minimum: pair.minimum})
		}
	}
	engine, err := styleguide.NewEngine(styleguide.EngineOptions{
		Source: observatoryGuideSource, SourceLabel: "cmd/fathom/ui/guide-baseline.cue",
		KitRevision: observatoryGuideRevision, DarkVariant: tokens.DarkVariantAttribute,
		Checks: checks, Fonts: map[string]string{
			"system-sans": "system-ui, sans-serif",
			"system-mono": "ui-monospace, monospace",
		},
	})
	if err != nil {
		return nil, err
	}
	specimens := make([]styleguide.Specimen, 0, 4)
	for _, item := range []struct{ state, label string }{
		{"complete", "Complete observation coverage"},
		{"partial", "Partial observation coverage"},
		{"missing", "Missing registry"},
		{"error", "Registry and call-log error"},
	} {
		state := item.state
		specimens = append(specimens, styleguide.Specimen{
			ID: state, Name: item.label,
			Render: func(_ context.Context, w io.Writer) error {
				model := observatoryGuideModel(state)
				fragment, err := disableObservatoryGuideLinks(renderObsFleet(model) + renderObsImpact(model))
				if err != nil {
					return err
				}
				_, err = io.WriteString(w, `<p class="mb-4 text-ink-muted">Synthetic appearance preview only; operational links are disabled.</p>`+fragment)
				return err
			},
		})
	}
	return styleguide.New(styleguide.Options{
		Engine: engine, BasePath: "/dev/theme", Origin: origin,
		Title:       "Fathom Observatory theme review",
		Stylesheets: []string{"/app.css"}, Specimens: specimens,
	})
}

// The real Observatory fragment contains links to live routes that this
// isolated guide intentionally does not mount. Parse the trusted fragment and
// make only those anchors non-operational; retain their text and classes for
// visual comparison without brittle string replacement or invented routes.
func disableObservatoryGuideLinks(fragment string) (string, error) {
	context := &nethtml.Node{Type: nethtml.ElementNode, DataAtom: atom.Div, Data: "div"}
	nodes, err := nethtml.ParseFragment(strings.NewReader(fragment), context)
	if err != nil {
		return "", err
	}
	var render func(*nethtml.Node)
	render = func(node *nethtml.Node) {
		if node.Type == nethtml.ElementNode && node.DataAtom == atom.A {
			node.Data, node.DataAtom = "span", atom.Span
			attrs := node.Attr[:0]
			for _, attr := range node.Attr {
				if attr.Key != "href" && attr.Key != "target" && attr.Key != "rel" {
					attrs = append(attrs, attr)
				}
			}
			node.Attr = append(attrs, nethtml.Attribute{Key: "aria-disabled", Val: "true"})
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			render(child)
		}
	}
	var out strings.Builder
	for _, node := range nodes {
		render(node)
		if err := nethtml.Render(&out, node); err != nil {
			return "", err
		}
	}
	return out.String(), nil
}

func observatoryGuideMux(origin string) (http.Handler, error) {
	guide, err := newObservatoryThemeGuide(origin)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle("/dev/theme", guide)
	mux.Handle("/dev/theme/", guide)
	mux.HandleFunc("/app.css", func(w http.ResponseWriter, r *http.Request) {
		peerHost, _, peerErr := net.SplitHostPort(r.RemoteAddr)
		peerIP := net.ParseIP(peerHost)
		if peerErr != nil || peerIP == nil || !peerIP.IsLoopback() ||
			r.Host != strings.TrimPrefix(origin, "http://") ||
			(r.Header.Get("Origin") != "" && r.Header.Get("Origin") != origin) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(obsAppCSS)
	})
	return mux, nil
}

func cmdThemeGuide(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("theme-guide", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:18767", "exact loopback address with port")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		return 1
	}
	host, port, err := net.SplitHostPort(*addr)
	portNum, portErr := strconv.Atoi(port)
	if err != nil || portErr != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() || portNum < 1 || portNum > 65535 {
		fmt.Fprintln(stderr, "error: --addr requires an exact loopback IP and port")
		return 1
	}
	origin := "http://" + net.JoinHostPort(host, port)
	handler, err := observatoryGuideMux(origin)
	if err != nil {
		fmt.Fprintf(stderr, "error: theme guide: %v\n", err)
		return 1
	}
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintf(stderr, "error: theme guide listen: %v\n", err)
		return 1
	}
	defer listener.Close()
	fmt.Fprintf(stdout, "Opt-in synthetic Observatory theme guide: %s/dev/theme\n", origin)
	server := &http.Server{Handler: handler}
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(stderr, "error: theme guide serve: %v\n", err)
		return 1
	}
	return 0
}
