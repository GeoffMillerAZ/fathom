// Command theme-cancellation serves the real Gallery theme lab behind a
// deliberately narrow, one-request cancellation probe. It is assessment
// machinery, not an application or reusable runtime API.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/geoffmilleraz/app-kit/apps/gallery"
)

const maxFormBytes = 64 << 10

type probeRecord struct {
	Armed       bool   `json:"armed"`
	Live        bool   `json:"live"`
	Started     bool   `json:"started"`
	Canceled    bool   `json:"canceled"`
	TimedOut    bool   `json:"timed_out"`
	Delegated   bool   `json:"delegated"`
	BodySHA256  string `json:"body_sha256,omitempty"`
	StartedAt   string `json:"started_at,omitempty"`
	CanceledAt  string `json:"canceled_at,omitempty"`
	TimedOutAt  string `json:"timed_out_at,omitempty"`
	DelegatedAt string `json:"delegated_at,omitempty"`
}

type probeHandler struct {
	next   http.Handler
	origin string
	host   string
	token  string
	delay  time.Duration
	now    func() time.Time

	mu     sync.Mutex
	record probeRecord
}

func newProbeHandler(next http.Handler, origin, token string, delay time.Duration) (*probeHandler, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" || u.Host == "" || u.Path != "" {
		return nil, errors.New("probe origin must be an HTTP origin")
	}
	if delay <= 0 {
		delay = 15 * time.Second
	}
	return &probeHandler{next: next, origin: origin, host: u.Host, token: token, delay: delay, now: time.Now}, nil
}

func (p *probeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawQuery == "" && r.URL.EscapedPath() == r.URL.Path && (r.URL.Path == "/__probe/arm" || r.URL.Path == "/__probe/state") {
		p.control(w, r)
		return
	}
	body, candidate := p.candidate(r)
	if !candidate {
		p.next.ServeHTTP(w, r)
		return
	}

	p.mu.Lock()
	if !p.record.Armed || p.record.Live {
		p.mu.Unlock()
		p.next.ServeHTTP(w, r)
		return
	}
	p.record.Armed = false
	p.record.Live = true
	p.record.Started = true
	p.record.BodySHA256 = hashBytes(body)
	p.record.StartedAt = p.timestamp()
	p.mu.Unlock()

	timer := time.NewTimer(p.delay)
	defer timer.Stop()
	select {
	case <-r.Context().Done():
		p.recordCancellation()
		return
	case <-timer.C:
		// If both signals are ready, cancellation wins: never invoke the real
		// handler for a request whose client has already gone away.
		select {
		case <-r.Context().Done():
			p.recordCancellation()
			return
		default:
		}
		p.mu.Lock()
		p.record.Live = false
		p.record.TimedOut = true
		p.record.TimedOutAt = p.timestamp()
		p.record.Delegated = true
		p.record.DelegatedAt = p.timestamp()
		p.mu.Unlock()
		p.next.ServeHTTP(w, r)
	}
}

func (p *probeHandler) recordCancellation() {
	p.mu.Lock()
	p.record.Live = false
	p.record.Canceled = true
	p.record.CanceledAt = p.timestamp()
	p.mu.Unlock()
}

// candidate recognizes the valid preview transport envelope, not the full
// candidate semantics owned by the real handler. It restores every byte it
// examines, so ineligible and malformed requests reach that handler with their
// original body and without r.Form or r.PostForm populated by the probe.
func (p *probeHandler) candidate(r *http.Request) ([]byte, bool) {
	origins := r.Header.Values("Origin")
	fetchSites := r.Header.Values("Sec-Fetch-Site")
	if r.Method != http.MethodPost || r.URL.Path != "/theme-lab" || r.URL.EscapedPath() != r.URL.Path || r.URL.RawQuery != "" ||
		r.Host != p.host || !loopbackPeer(r.RemoteAddr) || len(origins) != 1 || origins[0] != p.origin ||
		len(fetchSites) != 1 || (fetchSites[0] != "same-origin" && fetchSites[0] != "none") ||
		!acceptedFormMediaType(r.Header.Get("Content-Type")) || r.Body == nil {
		return nil, false
	}
	prefix, err := io.ReadAll(io.LimitReader(r.Body, maxFormBytes+1))
	r.Body = struct {
		io.Reader
		io.Closer
	}{Reader: io.MultiReader(bytes.NewReader(prefix), r.Body), Closer: r.Body}
	if err != nil || len(prefix) > maxFormBytes {
		return nil, false
	}
	values, err := url.ParseQuery(string(prefix))
	if err != nil || len(values["action"]) != 1 || values["action"][0] != "preview" {
		return nil, false
	}
	return prefix, true
}

func acceptedFormMediaType(value string) bool {
	media, params, err := mime.ParseMediaType(value)
	return err == nil && media == "application/x-www-form-urlencoded" && (len(params) == 0 || (len(params) == 1 && strings.EqualFold(params["charset"], "utf-8")))
}

func (p *probeHandler) control(w http.ResponseWriter, r *http.Request) {
	if r.Host != p.host || !loopbackPeer(r.RemoteAddr) {
		http.Error(w, "Probe controls require the exact loopback endpoint", http.StatusForbidden)
		return
	}
	want := []byte("Bearer " + p.token)
	got := []byte(r.Header.Get("Authorization"))
	if len(got) != len(want) || subtle.ConstantTimeCompare(got, want) != 1 {
		w.Header().Set("WWW-Authenticate", "Bearer")
		http.Error(w, "Probe authorization required", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	switch {
	case r.URL.Path == "/__probe/arm" && r.Method == http.MethodPost:
		p.mu.Lock()
		if p.record.Live {
			p.mu.Unlock()
			http.Error(w, "A delayed preview is already live", http.StatusConflict)
			return
		}
		p.record = probeRecord{Armed: true}
		state := p.record
		p.mu.Unlock()
		writeJSON(w, http.StatusOK, state)
	case r.URL.Path == "/__probe/state" && r.Method == http.MethodGet:
		p.mu.Lock()
		state := p.record
		p.mu.Unlock()
		writeJSON(w, http.StatusOK, state)
	default:
		w.Header().Set("Allow", map[string]string{"/__probe/arm": http.MethodPost, "/__probe/state": http.MethodGet}[r.URL.Path])
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (p *probeHandler) timestamp() string { return p.now().UTC().Format(time.RFC3339Nano) }

func loopbackPeer(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func hashBytes(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func readRegular(path string) ([]byte, string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, "", err
	}
	info, err := os.Stat(abs)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxFormBytes {
		return nil, "", errors.New("source must be a non-empty regular file no larger than 64KiB")
	}
	body, err := os.ReadFile(abs)
	if err != nil || len(body) == 0 || len(body) > maxFormBytes {
		return nil, "", errors.New("source is empty, too large, or unreadable")
	}
	return bytes.Clone(body), abs, nil
}

func executableHash() (path, digest string, err error) {
	path, err = os.Executable()
	if err != nil {
		return "", "", err
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	return path, hashBytes(body), nil
}

func randomToken() (string, error) {
	body := make([]byte, 32)
	if _, err := rand.Read(body); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(body), nil
}

func run() error {
	sourceFlag := flag.String("source", "", "absolute or relative path to the selected theme CUE source")
	flag.Parse()
	if *sourceFlag == "" {
		return errors.New("-source is required")
	}
	source, sourcePath, err := readRegular(*sourceFlag)
	if err != nil {
		return err
	}
	binaryPath, binarySHA, err := executableHash()
	if err != nil {
		return fmt.Errorf("executable identity: %w", err)
	}
	token, err := randomToken()
	if err != nil {
		return fmt.Errorf("control token: %w", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	origin := "http://" + listener.Addr().String()
	kitIdentity := "local-gallery-binary:sha256:" + binarySHA
	lab, err := gallery.NewThemeLab(origin, bytes.Clone(source), sourcePath, kitIdentity)
	if err != nil {
		_ = listener.Close()
		return err
	}
	realGallery := gallery.NewWithOptions(gallery.Options{ThemeLab: lab})
	probe, err := newProbeHandler(realGallery, origin, token, 15*time.Second)
	if err != nil {
		_ = listener.Close()
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	srv := &http.Server{
		Handler:           probe,
		ReadHeaderTimeout: 5 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	ready := struct {
		Schema       string `json:"schema"`
		Origin       string `json:"origin"`
		BinaryPath   string `json:"binary_path"`
		BinarySHA256 string `json:"binary_sha256"`
		SourcePath   string `json:"source_path"`
		SourceSHA256 string `json:"source_sha256"`
		KitIdentity  string `json:"kit_identity"`
		ControlToken string `json:"control_token"`
	}{"theme-cancellation-ready-v1", origin, binaryPath, binarySHA, sourcePath, hashBytes(source), kitIdentity, token}
	if err := json.NewEncoder(os.Stdout).Encode(ready); err != nil {
		_ = srv.Close()
		return err
	}
	served := make(chan error, 1)
	go func() { served <- srv.Serve(listener) }()

	select {
	case err := <-served:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdownErr := srv.Shutdown(shutdownCtx)
		if shutdownErr != nil {
			_ = srv.Close()
		}
		serveErr := <-served
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		return errors.Join(shutdownErr, serveErr)
	}
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "theme-cancellation:", err)
		os.Exit(1)
	}
}
