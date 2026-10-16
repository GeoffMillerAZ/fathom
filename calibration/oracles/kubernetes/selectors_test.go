package main

import (
	"reflect"
	"testing"
)

// TestSelectorJoins (LB-F456): a core v1 Service selects the objects of its
// own namespace whose pod labels its spec.selector matches, by
// apimachinery's ValidatedSelectorFromSet and Matches; an absent namespace
// is `default`; a Service with no or an empty selector, or one the label
// validation refuses, selects nothing; a non-string label map is no label
// set; a document after a malformed one is still read.
func TestSelectorJoins(t *testing.T) {
	w := &selWalk{}
	w.file("a.yaml", []byte(`apiVersion: v1
kind: Service
metadata: {name: web}
spec:
  selector: {app: web}
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: web, namespace: default}
spec:
  template:
    metadata: {labels: {app: web, tier: front}}
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: web, namespace: other}
spec:
  template:
    metadata: {labels: {app: web}}
---
apiVersion: v1
kind: Service
metadata: {name: headless}
spec: {}
---
apiVersion: v1
kind: Service
metadata: {name: empty}
spec: {selector: {}}
---
apiVersion: v1
kind: Service
metadata: {name: bad}
spec: {selector: {app: "not a valid value!"}}
---
apiVersion: v1
kind: Pod
metadata: {name: numeric, labels: {app: web, version: 1}}
spec: {}
---
key: [unclosed
---
apiVersion: batch/v1
kind: CronJob
metadata: {name: nightly}
spec:
  jobTemplate:
    spec:
      template:
        metadata: {labels: {app: web}}
`))
	w.file("b.yaml", []byte(`apiVersion: v1
kind: Pod
metadata: {name: p, labels: {app: web}}
`))
	got := w.pairs()
	want := []xref{
		{"a.yaml", "Service/web", "a.yaml", "CronJob/nightly"},
		{"a.yaml", "Service/web", "a.yaml", "Deployment/default/web"},
		{"a.yaml", "Service/web", "b.yaml", "Pod/p"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pairs\n got %+v\nwant %+v", got, want)
	}
	if w.st.services != 4 || w.st.noSelector != 2 || w.st.refused != 1 || w.st.badLabels != 1 || w.st.undecodable != 1 {
		t.Errorf("stats %+v", w.st)
	}
}
