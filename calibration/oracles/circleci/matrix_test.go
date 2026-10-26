package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMatrixUnionKeepsDisabledWorkflowsAndDistinctRequirements(t *testing.T) {
	dst := &compiled{jobs: map[string]bool{}, workflows: map[string][]cwjob{}}
	mergeCompiled(dst, &compiled{jobs: map[string]bool{"a": true}, workflows: map[string][]cwjob{"ordinary": {{name: "a", requires: []string{"z"}}}}})
	mergeCompiled(dst, &compiled{jobs: map[string]bool{"b": true}, workflows: map[string][]cwjob{"scheduled": {{name: "b"}}, "ordinary": {{name: "a", requires: []string{"y", "z"}}}}})
	want := map[string][]cwjob{"ordinary": {{name: "a", requires: []string{"y", "z"}}}, "scheduled": {{name: "b"}}}
	if !reflect.DeepEqual(dst.workflows, want) || len(dst.jobs) != 2 {
		t.Fatalf("union: %#v", dst)
	}
}

func TestParameterProtocolRejectsIdentityAndInvalidCases(t *testing.T) {
	for _, tc := range []struct{ body, name, sha, want string }{
		{`{"subjects":[{"name":"example/project","sha":"abc","configs":{".circleci/config.yml":[{}]}}]}`, "example/project", "def", "identity"},
		{`{"subjects":[{"name":"example/project","sha":"abc","configs":{".circleci/config.yml":[]}}]}`, "example/project", "abc", "cases"},
		{`{"subjects":[{"name":"example/project","sha":"abc","configs":{".circleci/config.yml":[{"x":{}}]}}]}`, "example/project", "abc", "scalar"},
		{`{"subjects":[]}`, "missing", "abc", "subject"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "matrix.json")
			if err := os.WriteFile(p, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := readParameterProtocol(p, tc.name, tc.sha)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %s", err, tc.want)
			}
		})
	}
}
