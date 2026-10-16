package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	lessontransfer "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/lesson-transfer"
)

const lessonInputVersion = "kit-workflow/lesson/v1"

const (
	lessonValidateUsage = "usage: kit-workflow lesson validate --lesson lesson.json --reproducer reproducer.json --attestations attestations.json"
	lessonAdoptionUsage = "usage: kit-workflow lesson adoption-check --lesson lesson.json --reproducer reproducer.json --attestations attestations.json --receiver receiver.json --assertion outcome.json"
	lessonUsage         = lessonValidateUsage + "\n" + lessonAdoptionUsage
)

type lessonOutput struct {
	Version          string                           `json:"version"`
	Operation        string                           `json:"operation"`
	PayloadDigest    string                           `json:"payload_digest"`
	ReproducerDigest string                           `json:"reproducer_digest"`
	Receiver         *lessontransfer.ReceiverSnapshot `json:"receiver,omitempty"`
	Integrity        string                           `json:"integrity"`
	Reproduction     string                           `json:"reproduction"`
	Fresh            bool                             `json:"fresh"`
	CheckedAt        time.Time                        `json:"checked_at"`
	Boundary         string                           `json:"boundary"`
}

// runLesson deliberately does not receive a reproduction, review, revocation,
// or host-current callback. It can validate local bytes, but never turns a
// caller assertion into an adoption or a passing reproduction.
func runLesson(ctx context.Context, args []string, stdout, stderr io.Writer, now func() time.Time) int {
	if len(args) == 1 && isHelp(args[0]) {
		return writeHelp(stdout, stderr, lessonUsage)
	}
	if len(args) > 1 && (args[0] == "validate" || args[0] == "adoption-check") {
		// Inspect flag positions only: a file literally named --help is input,
		// not a request for help. Do not open any inputs on the help path.
		for i := 1; i < len(args); i += 2 {
			if isHelp(args[i]) {
				usage := lessonValidateUsage
				if args[0] == "adoption-check" {
					usage = lessonAdoptionUsage
				}
				return writeHelp(stdout, stderr, usage)
			}
		}
	}
	if ctx == nil || now == nil {
		return writeLessonFailure(stdout, stderr, 1, "invalid", "context and clock are required")
	}
	if err := ctx.Err(); err != nil {
		return writeLessonFailure(stdout, stderr, 1, errorKind(err), err.Error())
	}
	if len(args) < 1 || (args[0] != "validate" && args[0] != "adoption-check") {
		return writeLessonFailure(stdout, stderr, 2, "usage", lessonUsage)
	}
	paths, err := lessonPaths(args[1:], args[0] == "adoption-check")
	if err != nil {
		return writeLessonFailure(stdout, stderr, 2, "usage", err.Error())
	}
	lesson, err := readLessonFile(paths.lesson, 64<<10)
	if err != nil {
		return writeLessonFailure(stdout, stderr, 1, "invalid", err.Error())
	}
	reproducer, err := readLessonFile(paths.reproducer, 256<<10)
	if err != nil {
		return writeLessonFailure(stdout, stderr, 1, "invalid", err.Error())
	}
	attestations, err := readLessonFile(paths.attestations, 64<<10)
	if err != nil {
		return writeLessonFailure(stdout, stderr, 1, "invalid", err.Error())
	}
	bundle, err := lessontransfer.ValidateBundle(lesson, reproducer, attestations)
	if err != nil {
		return writeLessonFailure(stdout, stderr, 1, "invalid", err.Error())
	}
	checkedAt := now().UTC()
	if checkedAt.IsZero() {
		return writeLessonFailure(stdout, stderr, 1, "invalid", "clock returned zero time")
	}
	if err := ctx.Err(); err != nil {
		return writeLessonFailure(stdout, stderr, 1, errorKind(err), err.Error())
	}
	result := lessonOutput{
		Version: lessonInputVersion, Operation: "lesson/validate", PayloadDigest: bundle.PayloadDigest, ReproducerDigest: bundle.ReproducerDigest,
		Integrity: "bound", Reproduction: "not-run", Fresh: false, CheckedAt: checkedAt,
		Boundary: "local byte validation only; no review acceptance, current-host check, revocation check, reproduction, persistence, execution, publication, or authorization occurred",
	}
	if args[0] == "adoption-check" {
		receiverBytes, err := readLessonFile(paths.receiver, 64<<10)
		if err != nil {
			return writeLessonFailure(stdout, stderr, 1, "invalid", err.Error())
		}
		assertionBytes, err := readLessonFile(paths.assertion, 64<<10)
		if err != nil {
			return writeLessonFailure(stdout, stderr, 1, "invalid", err.Error())
		}
		receiver, err := lessontransfer.ParseReceiverSnapshot(receiverBytes)
		if err != nil {
			return writeLessonFailure(stdout, stderr, 1, "invalid", err.Error())
		}
		outcome, err := lessontransfer.ParseReproductionOutcome(assertionBytes)
		if err != nil || outcome.Classification != bundle.Payload.Reproducer.Expected || receiver.Workload == "" {
			if err == nil {
				err = fmt.Errorf("receiver assertion does not match declared expected outcome")
			}
			return writeLessonFailure(stdout, stderr, 1, "invalid", err.Error())
		}
		if err := lessontransfer.ValidateApplicability(bundle, receiver); err != nil {
			return writeLessonFailure(stdout, stderr, 1, "invalid", err.Error())
		}
		result.Operation, result.Receiver = "lesson/adoption-check", &receiver
		result.Reproduction = "asserted-unverified"
		result.Boundary = "receiver and outcome are caller-supplied assertions; this command did not invoke receiver-owned review, current-host or revocation callbacks, or a reproducer, so adoption remains pending"
	}
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		fmt.Fprintf(stderr, "kit-workflow: write result: %v\n", err)
		return 1
	}
	return 0
}

type lessonFilePaths struct {
	lesson, reproducer, attestations, receiver, assertion string
}

func lessonPaths(args []string, requireAssertions bool) (lessonFilePaths, error) {
	var paths lessonFilePaths
	seen := make(map[string]bool, 5)
	if len(args)%2 != 0 {
		return paths, fmt.Errorf("lesson flags require values")
	}
	for len(args) != 0 {
		name, value := args[0], strings.TrimSpace(args[1])
		if value == "" {
			return paths, fmt.Errorf("%s is required", name)
		}
		if seen[name] {
			return paths, fmt.Errorf("duplicate lesson flag %q", name)
		}
		seen[name] = true
		switch name {
		case "--lesson":
			paths.lesson = value
		case "--reproducer":
			paths.reproducer = value
		case "--attestations":
			paths.attestations = value
		case "--receiver":
			paths.receiver = value
		case "--assertion":
			paths.assertion = value
		default:
			return paths, fmt.Errorf("unknown lesson flag %q", name)
		}
		args = args[2:]
	}
	if paths.lesson == "" || paths.reproducer == "" || paths.attestations == "" || (requireAssertions && (paths.receiver == "" || paths.assertion == "")) {
		return paths, fmt.Errorf("lesson, reproducer, and attestations are required; adoption-check also requires receiver and assertion")
	}
	if !requireAssertions && (paths.receiver != "" || paths.assertion != "") {
		return paths, fmt.Errorf("receiver and assertion are only valid for adoption-check")
	}
	return paths, nil
}

func readLessonFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect lesson input: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("lesson input must be a readable regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open lesson input: %w", err)
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read lesson input: %w", err)
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("lesson input exceeds byte limit")
	}
	return b, nil
}

func writeLessonFailure(stdout, stderr io.Writer, code int, kind, message string) int {
	result := errorOutput{Version: lessonInputVersion}
	result.Error.Kind, result.Error.Message = kind, message
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		fmt.Fprintf(stderr, "kit-workflow: write error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stderr, "kit-workflow: %s\n", message)
	return code
}
