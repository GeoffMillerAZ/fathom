package fusion

import (
	"reflect"
	"testing"
)

// TestResolveDocXrefs_ImageRef (LB-X-docker-image-ref): an image reference
// resolves through the image-tag index to the root entity (the last stage) of
// the in-repo Dockerfile that builds it — a compose service's image:, a
// Kubernetes container's image:, a Dockerfile's FROM and its COPY --from — and
// a compose build: (a context, or a context and a Dockerfile relative to it)
// resolves to that Dockerfile. A FROM or COPY --from naming an earlier stage,
// a stage index or an image nothing in the repository builds resolves to
// nothing, as does a templated reference.
func TestResolveDocXrefs_ImageRef(t *testing.T) {
	files := docFiles(t, "docker-compose", map[string]string{
		"compose.yaml": "services:\n" +
			"  base:\n    image: acme/base:1.0\n    build: ./base\n" +
			"  web:\n    image: docker.io/acme/base:1.0\n" +
			"  api:\n    build:\n      context: ./api\n      dockerfile: Dockerfile\n" +
			"  db:\n    image: postgres:16\n" +
			"  tmpl:\n    image: ${REGISTRY}/acme/base:1.0\n",
	})
	files = append(files, docFiles(t, "kubernetes", map[string]string{
		"deploy/web.yaml": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\nspec:\n  template:\n    spec:\n      containers:\n        - name: web\n          image: acme/base:1.0\n        - name: sidecar\n          image: envoyproxy/envoy:v1.28.0\n",
	})...)
	files = append(files, dockerfileDocs(t, map[string]string{
		"base/Dockerfile": "FROM alpine:3.19 AS base\nRUN apk add --no-cache curl\n",
		"api/Dockerfile": "# syntax=docker/dockerfile:1\n" +
			"FROM acme/base:1.0 AS build\n" +
			"RUN make\n" +
			"FROM build AS test\n" +
			"FROM golang:1.22\n" +
			"COPY --from=build /out/app /app\n" +
			"COPY --from=0 /etc/x /etc/x\n" +
			"COPY --link \\\n  --from=acme/base:1.0 /etc/ssl /etc/ssl\n" +
			"COPY --from=nginx:1.27 /etc/nginx /etc/nginx\n",
	})...)
	got := xrefBrief(ResolveDocXrefs(files))
	want := []string{
		"api/Dockerfile:build -from-> base/Dockerfile:base",
		"api/Dockerfile:golang:1.22 -copy_from-> base/Dockerfile:base",
		"compose.yaml:api -build-> api/Dockerfile:golang:1.22",
		"compose.yaml:base -build-> base/Dockerfile:base",
		"compose.yaml:base -runs-> base/Dockerfile:base",
		"compose.yaml:web -runs-> base/Dockerfile:base",
		"deploy/web.yaml:acme/base:1.0 -runs-> base/Dockerfile:base",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

// TestResolveDocXrefs_ImageRefWithoutABuildSite: the same references with no
// build site in the set (the harness's view of a compose subject: compose
// files only) resolve to nothing, and a COPY --from of an external image
// gives nothing whatever builds exist.
func TestResolveDocXrefs_ImageRefWithoutABuildSite(t *testing.T) {
	files := docFiles(t, "docker-compose", map[string]string{
		"compose.yaml": "services:\n  web:\n    image: acme/base:1.0\n  base:\n    image: acme/base:1.0\n    build: ./base\n",
	})
	if got := xrefBrief(ResolveDocXrefs(files)); len(got) != 0 {
		t.Fatalf("no Dockerfile in the set, want no edge: got %q", got)
	}
	files = dockerfileDocs(t, map[string]string{
		"a/Dockerfile": "FROM alpine:3.19 AS a\nCOPY --from=busybox:1.36 /bin/busybox /bin/busybox\n",
		"b/Dockerfile": "FROM busybox:1.36 AS b\n",
	})
	if got := xrefBrief(ResolveDocXrefs(files)); len(got) != 0 {
		t.Fatalf("an external image nothing builds, want no edge: got %q", got)
	}
}
