// Command releasecandidate prepares a private, unpublished working-tree snapshot.
// It is a maintainer tool, not an appkit consumer command or a release operation.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("releasecandidate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "explicit maintained app-kit checkout")
	out := flags.String("out", "", "fresh private output directory outside the checkout")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *root == "" || *out == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: releasecandidate -root /path/to/app-kit -out /fresh/private/candidate")
		return 2
	}
	receipt, err := prepare(ctx, *root, *out)
	if err != nil {
		fmt.Fprintln(stderr, "releasecandidate:", err)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(receipt); err != nil {
		fmt.Fprintln(stderr, "releasecandidate: writing receipt output:", err)
		return 1
	}
	return 0
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
