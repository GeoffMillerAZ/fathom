// Command gallery serves the kit's living style guide on loopback.
//
// The Gallery itself is just an http.Handler (D5); this is the thinnest
// possible shell around it. It binds 127.0.0.1 only — the style guide is a
// local development surface, not a public service.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/geoffmilleraz/app-kit/apps/gallery"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8765", "loopback address to serve on")
	flag.Parse()

	host, _, err := net.SplitHostPort(*addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gallery: bad -addr %q: %v\n", *addr, err)
		os.Exit(2)
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		fmt.Fprintf(os.Stderr, "gallery: -addr must be a loopback host (got %q); the style guide is local-only\n", host)
		os.Exit(2)
	}

	srv := &http.Server{Addr: *addr, Handler: gallery.New()}
	log.Printf("app-kit Gallery → http://%s", *addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("gallery: %v", err)
	}
}
