package main

// observatory_apply.go — SH-7 / SH-8 (docs/plans/ambient-fathom.md;
// docs/design/self-healing-fleet.md §6): the observatory's
// Preview changes → typed diff → Apply, and the law-13 guard around every
// endpoint that writes into a repository.
//
// The UI has no privileged path (law 10): preview is wiring.PlanWiring and
// apply is wiring.ApplyWiring — the same engine `fathom init` and `fathom
// fleet reconcile --apply` use — so a BLOCKED region is BLOCKED here too,
// and every apply lands one receipt that the Undo button (and `fathom
// fleet undo`) reverses.
//
// Routes (all under /observatory/wiring/, mounted by newHTTPHandler):
//
//	GET  /observatory/wiring/          every registered repository, each with a Preview button
//	POST /observatory/wiring/preview   ?service=ID          → the typed diff + plan_digest
//	POST /observatory/wiring/apply     service, plan_digest    → applies exactly the previewed plan
//	POST /observatory/wiring/undo      service, receipt_digest[, receipt] → restores a receipt
//
// Both mutations carry a token the page that offered them was given —
// plan_digest from the preview, receipt_digest from the apply result — so
// neither can be driven by a request that never saw what it is asking for.
//
// JSON when the caller asks for it (Accept: application/json or
// ?format=json), HTML otherwise. Rendering is app-kit only (cmd/fathom/
// CLAUDE.md): hitl.DiffView for the diff, hitl.RenderApprovalCard for the
// consent step, callout for BLOCKED and outcomes, form.RenderButton for the
// buttons — never hand-rolled components, never a palette colour.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/callout"
	"github.com/geoffmilleraz/app-kit/appkit/ui/dashboard"
	"github.com/geoffmilleraz/app-kit/appkit/ui/form"
	"github.com/geoffmilleraz/app-kit/appkit/ui/hitl"
	"github.com/geoffmilleraz/fathom/internal/wiring"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// ─── the registry (law 13: only repositories fathom already knows) ───────

// registeredService is one fleet entry as the daemon's own registry records
// it: the service id and the local corpus root ("" for a remote corpus other
// repositories query — nothing to wire there).
type registeredService struct {
	Service string `json:"service"`
	Root    string `json:"root,omitempty"`
}

// wiringRegistry is the ONLY source of repository paths the mutating
// endpoints will act on. A caller may name a service; it may never name a
// directory.
type wiringRegistry struct {
	list func() ([]registeredService, error)
}

// resolve looks a service id up in the registry.
func (reg wiringRegistry) resolve(service string) (registeredService, bool, error) {
	all, err := reg.list()
	if err != nil {
		return registeredService{}, false, err
	}
	for _, s := range all {
		if s.Service == service {
			return s, true, nil
		}
	}
	return registeredService{}, false, nil
}

// clientWiringRegistry reads the registry from the daemon's own store: the
// services it indexes and the corpus path each was ingested from.
func clientWiringRegistry(c *client.Client) wiringRegistry {
	return wiringRegistry{list: func() ([]registeredService, error) {
		ctx := context.Background()
		ids, err := c.Services(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]registeredService, 0, len(ids))
		for _, id := range ids {
			path, _, found, merr := c.ServiceMeta(ctx, id)
			if merr != nil {
				return nil, merr
			}
			if !found {
				path = ""
			}
			out = append(out, registeredService{Service: strings.ToLower(id), Root: path})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Service < out[j].Service })
		return out, nil
	}}
}

// mutationTarget is what requireLocalMutation resolved and handed to the
// endpoint behind it: a registered service and its registered root.
type mutationTarget struct {
	Service string
	Root    string
}

type mutationTargetKey struct{}

func mutationTargetFrom(ctx context.Context) (mutationTarget, bool) {
	t, ok := ctx.Value(mutationTargetKey{}).(mutationTarget)
	return t, ok
}

// hostIsLoopback reports whether a Host or Origin host names this machine:
// localhost, or an IP the kernel routes to loopback. Anything else — a LAN
// address, a tunnel hostname, a DNS name that happens to resolve here — is
// refused, because the check must not depend on a resolver's answer.
func hostIsLoopback(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// sameOrigin reports whether an Origin header names the origin this request
// was served from: the same scheme, the same port, and a host that names
// this machine. The loopback spellings — localhost, 127.0.0.1, ::1 — are ONE
// host here, because they are one machine and the page could have been
// opened under any of them. A page served by another local process on a
// different port is a different origin and is refused: loopback is where
// the daemon listens, not a trust boundary between the processes behind it,
// and an undo needs no plan digest a stranger would have to guess.
func sameOrigin(r *http.Request, origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || !hostIsLoopback(u.Host) {
		return false
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if !strings.EqualFold(u.Scheme, scheme) {
		return false
	}
	return portOf(u.Host, scheme) == portOf(r.Host, scheme)
}

// portOf is the explicit port of a host[:port], or the scheme's default.
func portOf(hostport, scheme string) string {
	if _, p, err := net.SplitHostPort(hostport); err == nil {
		return p
	}
	if scheme == "https" {
		return "443"
	}
	return "80"
}

// samePath reports whether two paths name the same directory once cleaned
// and made absolute. Symlinks are deliberately NOT resolved: the registry
// recorded a spelling, and that spelling is the one a caller must match.
func samePath(a, b string) bool {
	aa, err1 := filepath.Abs(filepath.Clean(a))
	bb, err2 := filepath.Abs(filepath.Clean(b))
	return err1 == nil && err2 == nil && aa == bb
}

// requireLocalMutation is the law-13 gate every mutating endpoint sits
// behind (SH-8). It refuses, before the handler runs:
//
//	403  a Host that is not a loopback name — nothing here is reachable off
//	     this machine, and a request that arrived under another name did not
//	     come from a page the daemon served;
//	403  an Origin that is not THIS origin (a cross-site page, or another
//	     local server on a different port — sameOrigin), or a Sec-Fetch-Site
//	     other than same-origin / none — a page in another tab cannot drive
//	     these buttons, and there are no cookies, so there is no ambient
//	     authority to borrow;
//	400  a service id the registry does not know;
//	400  any path that is not the registered root of that service — a
//	     caller may name a service, never a directory; a service with no
//	     local root (a remote corpus) is refused the same way.
//
// On success the resolved target rides the request context
// (mutationTargetFrom) so the handler never re-derives it from input.
func requireLocalMutation(reg wiringRegistry, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed — the mutating endpoints are POST only", http.StatusMethodNotAllowed)
			return
		}
		if !hostIsLoopback(r.Host) {
			http.Error(w, fmt.Sprintf("refused: Host %q is not loopback — the mutating endpoints serve this machine only (law 13)", r.Host), http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && !sameOrigin(r, origin) {
			http.Error(w, fmt.Sprintf("refused: Origin %q is not a page this daemon served (Host %q) — only a same-origin page may drive a mutation", origin, r.Host), http.StatusForbidden)
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
			http.Error(w, fmt.Sprintf("refused: Sec-Fetch-Site %q — only a same-origin page may drive a mutation", site), http.StatusForbidden)
			return
		}

		service := strings.ToLower(strings.TrimSpace(r.FormValue("service")))
		if service == "" {
			http.Error(w, "a service id is required (service=<id>) — the registry, not the caller, decides which directory that is", http.StatusBadRequest)
			return
		}
		svc, found, err := reg.resolve(service)
		if err != nil {
			http.Error(w, "the registry could not be read: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		if !found {
			http.Error(w, fmt.Sprintf("refused: service %q is not in the registry", service), http.StatusBadRequest)
			return
		}
		if svc.Root == "" {
			http.Error(w, fmt.Sprintf("refused: service %q has no local path in the registry — not a repository fathom can wire", service), http.StatusBadRequest)
			return
		}
		if p := r.FormValue("path"); p != "" && !samePath(p, svc.Root) {
			http.Error(w, fmt.Sprintf("refused: path %q is not the registered root of service %q — a caller may name a service, never a directory", p, service), http.StatusBadRequest)
			return
		}
		if fi, serr := os.Stat(svc.Root); serr != nil || !fi.IsDir() {
			http.Error(w, fmt.Sprintf("refused: the registered root of %q (%s) is not a directory on this machine", service, svc.Root), http.StatusBadRequest)
			return
		}
		ctx := context.WithValue(r.Context(), mutationTargetKey{}, mutationTarget{Service: service, Root: svc.Root})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ─── preview → apply (SH-7) ──────────────────────────────────────────────

// serviceWiringSpec is what the observatory's Apply wires into one
// repository: the stanza regions (created or appended when absent, adopted
// and upgraded when pre-v3, BLOCKED when tracked and hand-edited) and the
// hook entries with the shim spelling — reconcile's safe subset plus the
// initial wiring, since the button exists precisely for repositories the
// remedy line has been nagging about.
func serviceWiringSpec(root, service string) (wiring.Spec, []string) {
	spec := wiring.Spec{}
	for _, name := range stanzaFiles {
		spec.Regions = append(spec.Regions, stanzaRegion(name, service))
	}
	keys, notes := shimHookKeys(root)
	spec.Keys = keys
	return spec, notes
}

// wiringPreview is one repository's typed diff plus the freshness token the
// apply must hand back.
type wiringPreview struct {
	Service string      `json:"service"`
	Root    string      `json:"root"`
	Plan    wiring.Plan `json:"plan"`
	// PlanDigest covers the sorted plan items AND the current bytes of
	// every file the plan names, so a change anywhere in a target file —
	// inside fathom's region or not — between preview and apply changes
	// the digest and the apply is refused.
	PlanDigest string   `json:"plan_digest"`
	Notes      []string `json:"notes,omitempty"`
}

// planDigest is sha256 over the plan's own digest (its sorted items) and,
// per target file in sorted order, the digest of the file's current bytes
// ("absent" for a file that does not exist).
func planDigest(root string, plan wiring.Plan) string {
	files := map[string]bool{}
	for _, it := range plan.Items {
		files[it.File] = true
	}
	names := make([]string, 0, len(files))
	for f := range files {
		names = append(names, f)
	}
	sort.Strings(names)

	h := sha256.New()
	h.Write([]byte(plan.Digest))
	for _, f := range names {
		h.Write([]byte{0})
		h.Write([]byte(f))
		h.Write([]byte{0})
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil {
			h.Write([]byte("absent"))
			continue
		}
		sum := sha256.Sum256(data)
		h.Write(sum[:])
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// previewFor computes the typed diff for a target without touching it.
func previewFor(t mutationTarget) (wiringPreview, wiring.Spec, error) {
	spec, notes := serviceWiringSpec(t.Root, t.Service)
	plan, err := wiring.PlanWiring(t.Root, spec)
	if err != nil {
		return wiringPreview{}, spec, err
	}
	return wiringPreview{Service: t.Service, Root: t.Root, Plan: plan, PlanDigest: planDigest(t.Root, plan), Notes: notes}, spec, nil
}

// wiringPreviewHandler answers POST /observatory/wiring/preview.
func wiringPreviewHandler(baseURL string, now func() time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t, ok := mutationTargetFrom(r.Context())
		if !ok {
			http.Error(w, "internal: preview reached without a resolved target", http.StatusInternalServerError)
			return
		}
		pv, _, err := previewFor(t)
		if err != nil {
			http.Error(w, "preview failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if wantsJSON(r) {
			writeObsJSON(w, pv)
			return
		}
		m := cachedObservatoryModel(baseURL, now())
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(obsPage("fathom · wiring · "+t.Service, "wiring", renderWiringPreview(pv), m)))
	})
}

// wiringApplyResult is what an apply did — or refused to do.
type wiringApplyResult struct {
	Service string        `json:"service"`
	Root    string        `json:"root"`
	Result  wiring.Result `json:"result"`
	// ReceiptDigest is the token an undo of this apply must hand back
	// (receiptDigest of Result.Receipt); empty when nothing was written.
	ReceiptDigest string `json:"receipt_digest,omitempty"`
}

// receiptDigest is the undo token for one receipt: sha256 over its id, the
// plan it recorded, what it undid, and every file it touched with the
// digest fathom left that file at. It is derived from the receipt alone —
// the engine's Undo already refuses a file that moved since — so it proves
// one thing: the caller has seen this receipt, from the apply result that
// carried it. An undo that could be requested with `receipt=last` and no
// knowledge at all was the one mutation a page on another local port could
// drive blind.
func receiptDigest(r wiring.Receipt) string {
	h := sha256.New()
	for _, part := range []string{r.ID, r.Time, r.PlanDigest, r.UndoOf} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	for _, f := range r.Files {
		h.Write([]byte(f.Path))
		h.Write([]byte{0})
		h.Write([]byte(f.AfterDigest))
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// wiringApplyHandler answers POST /observatory/wiring/apply (SH-7). It
// requires the plan_digest a preview produced and recomputes it first: if
// the repository changed in between — any byte of any target file — the
// apply is refused with 409 and the diff the human approved is never
// silently replaced by a different one. The engine's own ExpectDigest is
// passed too, so the check holds even across a race between the
// recomputation and the write.
func wiringApplyHandler(baseURL string, now func() time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t, ok := mutationTargetFrom(r.Context())
		if !ok {
			http.Error(w, "internal: apply reached without a resolved target", http.StatusInternalServerError)
			return
		}
		want := strings.TrimSpace(r.FormValue("plan_digest"))
		if want == "" {
			http.Error(w, "apply requires the plan_digest from a preview — nothing is applied unseen", http.StatusBadRequest)
			return
		}
		pv, spec, err := previewFor(t)
		if err != nil {
			http.Error(w, "apply failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if pv.PlanDigest != want {
			msg := fmt.Sprintf("refused: %s changed since the plan was previewed (previewed %s, now %s) — re-run the preview and look at the new diff",
				t.Service, shortDigest(want), shortDigest(pv.PlanDigest))
			if wantsJSON(r) {
				w.WriteHeader(http.StatusConflict)
				writeObsJSON(w, map[string]any{"error": msg, "previewed": want, "now": pv.PlanDigest, "plan": pv.Plan})
				return
			}
			m := cachedObservatoryModel(baseURL, now())
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(obsPage("fathom · wiring · "+t.Service, "wiring", renderWiringRefused(t, msg), m)))
			return
		}
		res, err := wiring.ApplyWiring(t.Root, spec, wiring.ApplyOptions{Now: now, ExpectDigest: pv.Plan.Digest})
		if err != nil {
			status := http.StatusInternalServerError
			if strings.Contains(err.Error(), "changed since the plan was previewed") {
				status = http.StatusConflict
			}
			http.Error(w, "apply: "+err.Error(), status)
			return
		}
		out := wiringApplyResult{Service: t.Service, Root: t.Root, Result: res}
		if res.Receipt != nil {
			out.ReceiptDigest = receiptDigest(*res.Receipt)
		}
		if wantsJSON(r) {
			writeObsJSON(w, out)
			return
		}
		m := cachedObservatoryModel(baseURL, now())
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(obsPage("fathom · wiring · "+t.Service, "wiring", renderWiringApplied(out), m)))
	})
}

// wiringUndoHandler answers POST /observatory/wiring/undo: restore a receipt
// (the newest when none is named). It requires the receipt_digest the apply
// result carried, exactly as apply requires the preview's plan_digest:
// missing is 400, and a digest that is not the named receipt's is 409 —
// nothing is undone unseen. A refusal — any file changed since the receipt
// — is 409 with every refused file named, and nothing is written.
func wiringUndoHandler(baseURL string, now func() time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t, ok := mutationTargetFrom(r.Context())
		if !ok {
			http.Error(w, "internal: undo reached without a resolved target", http.StatusInternalServerError)
			return
		}
		want := strings.TrimSpace(r.FormValue("receipt_digest"))
		if want == "" {
			http.Error(w, "undo requires the receipt_digest from the apply result — nothing is undone unseen", http.StatusBadRequest)
			return
		}
		id := strings.TrimSpace(r.FormValue("receipt"))
		if id == "last" {
			id = ""
		}
		target, ferr := wiring.FindReceipt(t.Root, id)
		if ferr != nil {
			http.Error(w, "undo: "+ferr.Error(), http.StatusBadRequest)
			return
		}
		if got := receiptDigest(target); got != want {
			msg := fmt.Sprintf("refused: receipt_digest %s is not receipt %s's (%s) — undo only what the apply result showed you", shortDigest(want), target.ID, shortDigest(got))
			if wantsJSON(r) {
				w.WriteHeader(http.StatusConflict)
				writeObsJSON(w, map[string]any{"error": msg, "service": t.Service, "receipt": target.ID})
				return
			}
			m := cachedObservatoryModel(baseURL, now())
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(obsPage("fathom · wiring · "+t.Service, "wiring",
				callout.Render(callout.Callout{Kind: callout.Caution, Title: "undo refused — nothing was written", Body: msg})+wiringBackLink(), m)))
			return
		}
		res, err := wiring.Undo(t.Root, target.ID, wiring.ApplyOptions{Now: now})
		switch {
		case errors.Is(err, wiring.ErrUndoRefused):
			if wantsJSON(r) {
				w.WriteHeader(http.StatusConflict)
				writeObsJSON(w, map[string]any{"error": err.Error(), "service": t.Service, "result": res})
				return
			}
			var b strings.Builder
			for _, rf := range res.Refused {
				b.WriteString(callout.Render(callout.Callout{Kind: callout.Danger, Title: rf.Path, Body: rf.Reason}))
			}
			m := cachedObservatoryModel(baseURL, now())
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(obsPage("fathom · wiring · "+t.Service, "wiring",
				callout.Render(callout.Callout{Kind: callout.Caution, Title: "undo refused — nothing was written", Body: "fathom will not restore over a change it did not make. Resolve the file by hand, or leave it."})+b.String()+wiringBackLink(), m)))
			return
		case err != nil:
			http.Error(w, "undo: "+err.Error(), http.StatusBadRequest)
			return
		}
		if wantsJSON(r) {
			out := map[string]any{"service": t.Service, "root": t.Root, "result": res}
			if res.Receipted != nil {
				// The undo is itself receipted; this is the token to undo it.
				out["receipt_digest"] = receiptDigest(*res.Receipted)
			}
			writeObsJSON(w, out)
			return
		}
		var b strings.Builder
		b.WriteString(callout.Render(callout.Callout{
			Kind: callout.Success, Title: "undid receipt " + res.Receipt.ID,
			Body: fmt.Sprintf("%s restored, %s removed; the undo is itself receipted as %s", plural(len(res.Restored), "file"), plural(len(res.Removed), "file"), receiptIDOr(res.Receipted)),
		}))
		b.WriteString(wiringBackLink())
		m := cachedObservatoryModel(baseURL, now())
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(obsPage("fathom · wiring · "+t.Service, "wiring", b.String(), m)))
	})
}

func receiptIDOr(r *wiring.Receipt) string {
	if r == nil {
		return "(none)"
	}
	return r.ID
}

func shortDigest(d string) string {
	d = strings.TrimPrefix(d, "sha256:")
	if len(d) > 8 {
		d = d[:8]
	}
	return "sha256:" + d
}

// wiringRoutes mounts the wiring page and the three guarded mutations.
func wiringRoutes(reg wiringRegistry, baseURL string, now func() time.Time) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/observatory/wiring/", wiringPageHandler(reg, baseURL, now))
	mux.Handle("/observatory/wiring/preview", requireLocalMutation(reg, wiringPreviewHandler(baseURL, now)))
	mux.Handle("/observatory/wiring/apply", requireLocalMutation(reg, wiringApplyHandler(baseURL, now)))
	mux.Handle("/observatory/wiring/undo", requireLocalMutation(reg, wiringUndoHandler(baseURL, now)))
	return mux
}

// ─── rendering (app-kit only) ────────────────────────────────────────────

// wiringPageHandler answers GET /observatory/wiring/: every registered
// repository with a Preview button — the consent step starts here, and
// nothing on this page writes.
func wiringPageHandler(reg wiringRegistry, baseURL string, now func() time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		all, err := reg.list()
		if wantsJSON(r) {
			if err != nil {
				http.Error(w, "the registry could not be read: "+err.Error(), http.StatusServiceUnavailable)
				return
			}
			writeObsJSON(w, map[string]any{"services": all})
			return
		}
		var b strings.Builder
		b.WriteString(`<p class="mb-2 max-w-3xl text-ink-muted">Preview shows exactly what fathom would write into a repository — file by file, before and after — and only then offers Apply. A region fathom is tracking that was edited by hand is BLOCKED, never merged (law 15). Every apply is receipted and undoable from here or with <code class="font-mono">fathom fleet undo</code>.</p>`)
		if err != nil {
			b.WriteString(callout.Render(callout.Callout{Kind: callout.Caution, Title: "registry unavailable", Body: err.Error()}))
		}
		var widgets []string
		for _, s := range all {
			var body strings.Builder
			if s.Root == "" {
				body.WriteString(kv("root", `<span class="italic text-ink-faint">no local path — a remote corpus; nothing to wire</span>`))
			} else {
				body.WriteString(kv("root", `<span class="font-mono text-xs break-all">`+html.EscapeString(s.Root)+`</span>`))
				body.WriteString(`<form method="post" action="/observatory/wiring/preview" class="mt-2"><input type="hidden" name="service" value="` + html.EscapeString(s.Service) + `">` +
					form.RenderButton(form.Button{Label: "Preview changes", Variant: form.ButtonGhost}) + `</form>`)
			}
			widgets = append(widgets, dashboard.RenderWidget(dashboard.Widget{Title: s.Service, Body: template.HTML(body.String())}))
		}
		b.WriteString(sectionHeading("wiring (" + plural(len(all), "service") + ")"))
		b.WriteString(dashboard.RenderWidgetGrid(widgets, dashboard.WidgetGridOptions{Cols: 3, AriaLabel: "wiring per service"}))
		m := cachedObservatoryModel(baseURL, now())
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(obsPage("fathom · wiring", "wiring", b.String(), m)))
	})
}

func wiringBackLink() string {
	return `<p class="mt-4"><a class="text-accent" href="/observatory/wiring/">← wiring</a> · <a class="text-accent" href="/observatory">fleet</a></p>`
}

// renderWiringPreview is the typed diff, then the consent card. BLOCKED
// items render first and loudest; apply, skip, and the reason for each
// follow; the Apply button carries the plan digest back.
func renderWiringPreview(pv wiringPreview) string {
	var b strings.Builder
	b.WriteString(sectionHeading("preview · " + pv.Service))
	b.WriteString(kv("root", `<span class="font-mono text-xs break-all">`+html.EscapeString(pv.Root)+`</span>`))
	if pv.Plan.LedgerNote != "" {
		b.WriteString(callout.Render(callout.Callout{Kind: callout.Caution, Title: "ownership ledger", Body: pv.Plan.LedgerNote}))
	}
	for _, n := range pv.Notes {
		b.WriteString(callout.Render(callout.Callout{Kind: callout.Info, Body: strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(n), "note:"))}))
	}
	for _, it := range pv.Plan.Blocked() {
		b.WriteString(callout.Render(callout.Callout{
			Kind:  callout.Danger,
			Title: "BLOCKED · " + it.File + " — fathom will not merge (law 15)",
			Body:  it.Reason,
			Details: template.HTML(hitl.DiffView([]hitl.Change{{
				Label:  it.File + " · " + it.Target + " · yours (left) vs fathom's (right)",
				Before: it.Before,
				After:  it.After,
			}})),
			DetailsLabel: "Show both versions",
			Open:         true,
		}))
	}
	applicable := pv.Plan.Applicable()
	var changes []hitl.Change
	for _, it := range applicable {
		changes = append(changes, hitl.Change{Label: it.File + " · " + it.Target + " — " + it.Reason, Before: it.Before, After: it.After})
	}
	if len(changes) > 0 {
		b.WriteString(sectionHeading("changes fathom would write (" + plural(len(changes), "item") + ")"))
		b.WriteString(hitl.DiffView(changes))
	}
	if skipped := pv.Plan.Skipped(); len(skipped) > 0 {
		var lines []string
		for _, it := range skipped {
			lines = append(lines, html.EscapeString(it.File+" · "+it.Target+" — "+it.Reason))
		}
		b.WriteString(callout.Render(callout.Callout{Kind: callout.Info, Title: plural(len(skipped), "item") + " already current", BodyHTML: template.HTML(strings.Join(lines, "<br>"))}))
	}
	if len(applicable) == 0 {
		b.WriteString(callout.Render(callout.Callout{Kind: callout.Success, Title: "nothing to apply", Body: "every region and hook fathom owns here is current" + blockedSuffix(len(pv.Plan.Blocked()))}))
		b.WriteString(wiringBackLink())
		return b.String()
	}
	files := map[string]bool{}
	for _, it := range applicable {
		files[it.File] = true
	}
	actions := `<form method="post" action="/observatory/wiring/apply"><input type="hidden" name="service" value="` + html.EscapeString(pv.Service) + `"><input type="hidden" name="plan_digest" value="` + html.EscapeString(pv.PlanDigest) + `">` +
		form.RenderButton(form.Button{Label: "Apply exactly this", Variant: form.ButtonPrimary}) + `</form>` +
		form.RenderButton(form.Button{Label: "Cancel", Variant: form.ButtonGhost, Href: "/observatory/wiring/"})
	b.WriteString(hitl.RenderApprovalCard(hitl.ApprovalCard{
		Intent:  "Apply wiring",
		Title:   "Write " + plural(len(applicable), "item") + " into " + plural(len(files), "file") + " of " + pv.Service,
		Summary: "Only the items shown above are written; every other byte of every file survives. The write is refused if the repository changes before you click (plan " + shortDigest(pv.PlanDigest) + "), and it is receipted so it can be undone." + blockedSuffix(len(pv.Plan.Blocked())),
		Risk:    hitl.RiskLow,
		Changes: []hitl.Metric{{Label: "files touched", From: "0", To: fmt.Sprint(len(files))}},
		Actions: template.HTML(actions),
	}))
	return b.String()
}

func blockedSuffix(n int) string {
	if n == 0 {
		return ""
	}
	return " " + plural(n, "BLOCKED file") + " will not be written."
}

// renderWiringRefused is the 409: the repository moved between preview and
// apply, so the approved diff is not the diff that would be written.
func renderWiringRefused(t mutationTarget, msg string) string {
	var b strings.Builder
	b.WriteString(callout.Render(callout.Callout{Kind: callout.Caution, Title: "apply refused — the repository changed since the preview", Body: msg}))
	b.WriteString(`<form method="post" action="/observatory/wiring/preview" class="mt-2"><input type="hidden" name="service" value="` + html.EscapeString(t.Service) + `">` +
		form.RenderButton(form.Button{Label: "Preview again", Variant: form.ButtonPrimary}) + `</form>`)
	b.WriteString(wiringBackLink())
	return b.String()
}

// renderWiringApplied is the outcome: what was written (receipted, with an
// Undo button), what was skipped, and what was BLOCKED.
func renderWiringApplied(out wiringApplyResult) string {
	var b strings.Builder
	res := out.Result
	b.WriteString(sectionHeading("applied · " + out.Service))
	for _, it := range res.Blocked {
		b.WriteString(callout.Render(callout.Callout{Kind: callout.Danger, Title: "BLOCKED · " + it.File + " — not written (law 15)", Body: it.Reason}))
	}
	if res.Receipt != nil {
		var names []string
		for _, f := range res.Receipt.Files {
			names = append(names, f.Path)
		}
		undo := `<form method="post" action="/observatory/wiring/undo"><input type="hidden" name="service" value="` + html.EscapeString(out.Service) + `"><input type="hidden" name="receipt" value="` + html.EscapeString(res.Receipt.ID) + `"><input type="hidden" name="receipt_digest" value="` + html.EscapeString(out.ReceiptDigest) + `">` +
			form.RenderButton(form.Button{Label: "Undo this apply", Variant: form.ButtonDanger}) + `</form>`
		b.WriteString(callout.Render(callout.Callout{
			Kind:     callout.Success,
			Title:    "wrote " + plural(len(res.Applied), "item") + " — receipt " + res.Receipt.ID,
			BodyHTML: template.HTML(html.EscapeString(strings.Join(names, ", ")) + `<div class="mt-2">` + undo + `</div>` + `<div class="mt-2 font-mono text-xs text-ink-muted">$ fathom fleet undo ` + html.EscapeString(res.Receipt.ID) + ` --repo ` + html.EscapeString(out.Root) + `</div>`),
		}))
	} else if len(res.Applied) == 0 {
		b.WriteString(callout.Render(callout.Callout{Kind: callout.Info, Title: "nothing written", Body: "every item was already current" + blockedSuffix(len(res.Blocked))}))
	}
	if len(res.Applied) > 0 {
		var changes []hitl.Change
		for _, it := range res.Applied {
			changes = append(changes, hitl.Change{Label: it.File + " · " + it.Target, Before: it.Before, After: it.After})
		}
		b.WriteString(hitl.DiffView(changes))
	}
	b.WriteString(wiringBackLink())
	return b.String()
}
