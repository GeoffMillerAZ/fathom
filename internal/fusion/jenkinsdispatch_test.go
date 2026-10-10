package fusion

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// jenkinsCorpus is a repository with a declarative Jenkinsfile, a second
// pipeline by extension, a shared library's vars/ and the build entries the
// pipeline's shell steps run.
func jenkinsCorpus() map[string]string {
	return map[string]string{
		"Jenkinsfile": `@Library('ops@main') _

pipeline {
    agent any
    stages {
        stage('Build') {
            steps {
                sh 'make test'
                sh """
                    task lint
                    npm run build
                """
                sh(script: './scripts/ci.sh')
                dir('web') {
                    sh 'npm run bundle'
                }
                sh "make ${TARGET}"
                sh "task ${TASK_NAME}"
                dir("${SUBDIR}") {
                    sh 'make test'
                }
                sh 'make ghost'
                buildApp(target: 'all')
                unknownStep()
                build job: 'deploy-prod'
                build job: "deploy-${ENV}"
            }
        }
        stage('Package') {
            steps {
                powershell(script: 'make dist')
            }
        }
    }
}
`,
		"ci/release.jenkinsfile":   "pipeline {\n  agent any\n  stages {\n    stage('Release') {\n      steps {\n        bat 'make dist'\n      }\n    }\n  }\n}\n",
		"vars/buildApp.groovy":     "def call(Map config = [:]) {\n    sh 'make ghost-from-library'\n    deployAll()\n}\n",
		"vars/deployAll.groovy":    "def call() {\n    echo 'deploying'\n}\n\ndef rollback() {\n    echo 'rolling back'\n}\n",
		"vars/buildApp.txt":        "Builds the app.\n",
		"src/org/acme/Util.groovy": "package org.acme\nclass Util {\n  def call() { 'x' }\n}\n",
		"Makefile":                 "test:\n\t@echo t\ndist:\n\t@echo d\nghost-from-library:\n\t@echo g\n",
		"Taskfile.yml":             "version: '3'\ntasks:\n  lint:\n    cmds:\n      - echo\n",
		"package.json":             "{\"name\": \"app\", \"scripts\": {\"build\": \"tsc\"}}\n",
		"web/package.json":         "{\"name\": \"web\", \"scripts\": {\"bundle\": \"vite build\"}}\n",
		"scripts/ci.sh":            "#!/bin/bash\nsetup() { echo a; }\nmain() { setup; }\nmain\n",
	}
}

// TestJenkinsfileIsTheDialect (LB-F373): a Jenkinsfile by basename, a
// .jenkinsfile by extension and a shared library's vars/<name>.groovy by its
// path read through the jenkinsfile dialect; a Groovy source anywhere else is
// claimed by nothing (LB-Q-groovy-rest keeps .groovy unsupported).
func TestJenkinsfileIsTheDialect(t *testing.T) {
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	want := p.exts["Jenkinsfile"]
	if want == nil {
		t.Fatal("no jenkinsfile dialect registered for the Jenkinsfile basename")
	}
	for _, rel := range []string{"Jenkinsfile", "ci/Jenkinsfile", "ci/release.jenkinsfile", "vars/buildApp.groovy", "libs/ops/vars/deploy.groovy"} {
		e, _, ok, _ := p.resolveExtractor(rel, []byte("def call() { echo 'x' }\n"), nil)
		if !ok || e != want {
			t.Errorf("%s: dispatched to %T (ok=%v), want the jenkinsfile dialect", rel, e, ok)
		}
	}
	for _, rel := range []string{"src/org/acme/Util.groovy", "Util.groovy", "vars/notes.txt", "vars/sub/deep.groovy", "build.gradle", "Jenkinsfile.groovy"} {
		e, _, ok, _ := p.resolveExtractor(rel, []byte("class Util { def call() { 'x' } }\n"), nil)
		if ok && e == want {
			t.Errorf("%s: claimed by the jenkinsfile dialect, want it left alone", rel)
		}
	}
}

func jenkinsIngest(t *testing.T) (*Pipeline, store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	writeCorpus(t, dir, jenkinsCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: "jk", Path: dir}); err != nil {
		t.Fatal(err)
	}
	return p, st, dir
}

// TestIngest_JenkinsfileDialect (LB-F373): ingest stores the declarative
// sections of a Jenkinsfile, a shared-library variable's call, and nothing for
// a plain Groovy source.
func TestIngest_JenkinsfileDialect(t *testing.T) {
	_, st, _ := jenkinsIngest(t)
	got := crSourceKinds(t, st, "jk")
	for rel, kind := range map[string]string{
		"Jenkinsfile":            "JenkinsStage",
		"ci/release.jenkinsfile": "JenkinsSteps",
		"vars/buildApp.groovy":   "JenkinsFunction",
	} {
		if !crHasKind(got[rel], kind) {
			t.Errorf("%s read as %q, want %s", rel, got[rel], kind)
		}
	}
	for _, kind := range []string{"JenkinsPipeline", "JenkinsAgent", "JenkinsStages", "JenkinsSteps", "JenkinsLibrary"} {
		if !crHasKind(got["Jenkinsfile"], kind) {
			t.Errorf("Jenkinsfile lacks %s: %q", kind, got["Jenkinsfile"])
		}
	}
	if k := got["src/org/acme/Util.groovy"]; k != "" {
		t.Errorf("a plain Groovy source was read as %q, want nothing (LB-Q-groovy-rest)", k)
	}
}

// TestReingestFile_JenkinsfileParity: a one-file refresh of each Jenkins file
// reads it as the ingest did.
func TestReingestFile_JenkinsfileParity(t *testing.T) {
	p, st, dir := jenkinsIngest(t)
	ctx := context.Background()
	before := crSourceKinds(t, st, "jk")
	for _, rel := range []string{"Jenkinsfile", "ci/release.jenkinsfile", "vars/buildApp.groovy", "vars/deployAll.groovy"} {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(b, []byte("// edited\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := p.ReingestFile(ctx, st, "jk", dir, rel); err != nil {
			t.Fatal(err)
		}
	}
	if after := crSourceKinds(t, st, "jk"); !reflect.DeepEqual(before, after) {
		t.Errorf("ReingestFile changed what a file reads as\nbefore: %v\n after: %v", before, after)
	}
}
