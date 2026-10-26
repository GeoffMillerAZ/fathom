// Fake peer for synthetic-only command-guard trials. It answers one request.
// Usage: go run ./cmd/command-guard/testdata/fake-concern-peer.go concern|no-concern|unknown
// Pin expectedResponseModel to fake-response. This peer performs no inference.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] != "concern" && os.Args[1] != "no-concern" && os.Args[1] != "unknown" {
		fmt.Fprintln(os.Stderr, "choice: concern|no-concern|unknown")
		os.Exit(2)
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		os.Exit(2)
	}
	done := make(chan struct{})
	var once sync.Once
	var accepted atomic.Bool
	srv := &http.Server{ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: time.Second, MaxHeaderBytes: 8 << 10}
	srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !accepted.CompareAndSwap(false, true) {
			http.Error(w, "one request only", 503)
			return
		}
		defer once.Do(func() { close(done) })
		if r.Method != "POST" || r.URL.Path != "/v1/chat/completions" {
			http.Error(w, "unsupported", 404)
			return
		}
		defer r.Body.Close()
		var request struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10))
		if decoder.Decode(&request) != nil {
			http.Error(w, "invalid", 400)
			return
		}
		var trailing any
		if _, err := decoder.Token(); err != io.EOF {
			http.Error(w, "invalid", 400)
			return
		}
		_ = trailing
		questionID := ""
		for _, m := range request.Messages {
			var prompt struct {
				Questions []struct {
					ID string `json:"id"`
				} `json:"questions"`
			}
			if json.Unmarshal([]byte(m.Content), &prompt) == nil && len(prompt.Questions) == 1 {
				questionID = prompt.Questions[0].ID
				break
			}
		}
		if questionID == "" {
			http.Error(w, "invalid", 400)
			return
		}
		probabilities := map[string]float64{"concern": 0, "no-concern": 0, "unknown": 0}
		probabilities[os.Args[1]] = 1
		inner, _ := json.Marshal(map[string]any{"answers": map[string]any{questionID: map[string]any{"choice": os.Args[1], "probabilities": probabilities}}})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "fake-response", "choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": string(inner)}}}})
	})
	fmt.Println("http://" + listener.Addr().String() + "/v1")
	go func() { _ = srv.Serve(listener) }()
	lifetime := time.NewTimer(5 * time.Minute)
	defer lifetime.Stop()
	select {
	case <-done:
	case <-lifetime.C:
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if srv.Shutdown(ctx) != nil {
		_ = srv.Close()
	}
}
