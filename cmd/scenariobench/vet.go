package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/benchprep"
)

func init() { offlineCommands["vet"] = cmdVet }

// suiteImages are run_suites.py's images, keyed by C2's primary language.
var suiteImages = map[string]string{"go": "golang:1.27-bookworm", "python": "python:3.12-bookworm", "node": "node:22-bookworm"}

// dockerSuite runs C3's offline trials under the preparation mount contract
// (docs/design/bench-prep.md section 5): the clone read-only at /subject, the
// subject's prepared work directory at /work and bench.d at /bench-inputs.
type dockerSuite struct {
	WorkRoot    string // <WorkRoot>/<name> holds the operator's prepared /work
	BenchInputs string
	UID, GID    int
	Timeout     time.Duration
}

func (d dockerSuite) argv(req benchprep.SuiteRequest) ([]string, error) {
	if d.UID == 0 {
		return nil, errors.New("C3 refuses to run trials as uid 0")
	}
	image, ok := suiteImages[req.Language]
	if !ok {
		return nil, fmt.Errorf("no suite image for language %q", req.Language)
	}
	clone, err := filepath.Abs(req.Clone)
	if err != nil {
		return nil, err
	}
	work, err := filepath.Abs(filepath.Join(d.WorkRoot, req.Subject))
	if err != nil {
		return nil, err
	}
	inputs, err := filepath.Abs(d.BenchInputs)
	if err != nil {
		return nil, err
	}
	return []string{"docker", "run", "--rm", "--network", "none", "--cpus", "4",
		"--user", fmt.Sprintf("%d:%d", d.UID, d.GID),
		"-v", clone + ":/subject:ro", "-v", work + ":/work", "-v", inputs + ":/bench-inputs:ro",
		"-w", "/subject", image, "sh", "-c", req.Command}, nil
}

func (d dockerSuite) run(ctx context.Context, req benchprep.SuiteRequest) (benchprep.SuiteRun, error) {
	argv, err := d.argv(req)
	if err != nil {
		return benchprep.SuiteRun{}, err
	}
	if info, err := os.Stat(filepath.Join(d.WorkRoot, req.Subject)); err != nil || !info.IsDir() {
		return benchprep.SuiteRun{}, fmt.Errorf("no prepared work directory for %s under %s", req.Subject, d.WorkRoot)
	}
	ctx, cancel := context.WithTimeout(ctx, d.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	start := time.Now()
	err = cmd.Run()
	run := benchprep.SuiteRun{Seconds: float64(time.Since(start).Milliseconds()) / 1000, Network: "none"}
	var exit *exec.ExitError
	switch {
	case ctx.Err() != nil:
		run.ExitCode = 124
	case errors.As(err, &exit):
		run.ExitCode = exit.ExitCode()
	case err != nil:
		return benchprep.SuiteRun{}, err
	}
	return run, nil
}

// Tests replace these so no container, miner or git remote is needed.
var (
	newVetSuite = func(d dockerSuite) benchprep.SuiteRunner { return d.run }
	vetMiner    func(ctx context.Context, clone string) ([]benchprep.FixCommit, error)
	readSelfURL = func(ctx context.Context) (string, error) {
		out, err := exec.CommandContext(ctx, "git", "config", "remote.origin.url").Output()
		return strings.TrimSpace(string(out)), err
	}
)

// cmdVet writes one C1 to C7 receipt per manifest from operator-prepared
// clones. It never clones or touches the network.
func cmdVet(args []string) int {
	flags := newFlagSet("vet")
	manifests := flags.String("manifests", filepath.Join("calibration", "bench.d"), "directory of subject manifests")
	corpusDir := flags.String("corpus-dir", "calibration", "directory holding corpus.yaml and corpus.d/ (C7)")
	subjectsDir := flags.String("subjects-dir", filepath.Join("workspace", "scenariobench", "subjects"), "operator-prepared full clones, one per manifest name")
	workDir := flags.String("work-dir", filepath.Join("workspace", "bench", "work"), "prepared /work directories, one per manifest name (C3)")
	c6Dir := flags.String("c6", "", "directory of recount.py outputs <name>.json (C6; missing leaves C6 pending)")
	selfURL := flags.String("self-url", "", "this checkout's repository url (C7; default: git config remote.origin.url)")
	outDir := flags.String("out", filepath.Join("workspace", "bench", "vet"), "receipt directory")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	fail := func(err error) int {
		fmt.Fprintf(os.Stderr, "scenariobench vet: %v\n", err)
		return 2
	}
	subjects, err := benchprep.LoadSubjectManifests(*manifests)
	if err != nil {
		return fail(err)
	}
	selected := map[string]bool{}
	for _, name := range flags.Args() {
		selected[name] = true
	}
	corpus, err := benchprep.LoadCorpusURLs(filepath.Join(*corpusDir, "corpus.yaml"), filepath.Join(*corpusDir, "corpus.d"))
	if err != nil {
		return fail(err)
	}
	if *selfURL == "" {
		if *selfURL, err = readSelfURL(context.Background()); err != nil || *selfURL == "" {
			return fail(fmt.Errorf("no -self-url and no remote.origin.url: %v", err))
		}
	}
	deps := benchprep.VetDeps{
		SubjectsDir: *subjectsDir,
		Suite:       newVetSuite(dockerSuite{WorkRoot: *workDir, BenchInputs: *manifests, UID: os.Getuid(), GID: os.Getgid(), Timeout: 20 * time.Minute}),
		Miner:       vetMiner,
		SelfURL:     *selfURL,
		CorpusURLs:  corpus,
		Peers:       subjects,
	}
	if *c6Dir != "" {
		deps.C6Rows = func(name string) ([]benchprep.C6Row, error) {
			raw, err := os.ReadFile(filepath.Join(*c6Dir, name+".json"))
			if err != nil {
				return nil, err
			}
			var rows []benchprep.C6Row
			return rows, json.Unmarshal(raw, &rows)
		}
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return fail(err)
	}
	vetted := 0
	for _, s := range subjects {
		if len(selected) > 0 && !selected[s.Name] {
			continue
		}
		delete(selected, s.Name)
		receipt, err := benchprep.VetSubject(context.Background(), s, deps)
		if err != nil {
			return fail(err)
		}
		data, err := json.MarshalIndent(receipt, "", "  ")
		if err != nil {
			return fail(err)
		}
		if err := os.WriteFile(filepath.Join(*outDir, s.Name+".json"), append(data, '\n'), 0o644); err != nil {
			return fail(err)
		}
		fmt.Printf("%s: %s\n", s.Name, receipt.Verdict)
		vetted++
	}
	if len(selected) > 0 {
		var missing []string
		for name := range selected {
			missing = append(missing, name)
		}
		sort.Strings(missing)
		return fail(fmt.Errorf("no manifest named %s", strings.Join(missing, ", ")))
	}
	if vetted == 0 {
		return fail(errors.New("no manifests to vet"))
	}
	return 0
}
