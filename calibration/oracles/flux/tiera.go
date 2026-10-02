package main

// tiera.go — the tier-A definitions (wave ox, LB-F156).
//
//   - Every document of a Flux file is decoded strictly (sigs.k8s.io/yaml
//     UnmarshalStrict: an unknown or duplicated field fails) into its kind's
//     type from the Flux controllers' own API modules (kustomize-controller
//     api v1, helm-controller api v2, source-controller api v1,
//     notification-controller api v1 and v1beta3; Apache-2.0), the types the
//     controllers and the flux CLI read objects with. A document of a Flux
//     group and version the oracle carries no type for, or one the types
//     refuse, fails its file, which contributes no definition and is counted.
//   - A Kustomization is also loaded by the flux CLI itself: `flux build
//     kustomization <name> -n <namespace> --kustomization-file <file> --path
//     <an empty kustomization> --dry-run` finds it by name and namespace in its
//     file and builds offline (the path is a scratch kustomization, since
//     spec.path is relative to a source the oracle never fetches).
//   - The entities are the decoded objects' (Kind/[namespace/]name); the walk
//     only places them at their line; one the types carry and the walk cannot
//     place is unlocated, one only the walk has is disagreement, both counted.
//
// References and cross_references stay the walk's (tier C) in -refs-out: no
// offline tool resolves a sourceRef, chartRef, dependsOn or valuesFrom (flux
// build reads no source, and the controllers resolve them in a cluster).

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	notificationv1 "github.com/fluxcd/notification-controller/api/v1"
	notificationv1beta3 "github.com/fluxcd/notification-controller/api/v1beta3"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	sigsyaml "sigs.k8s.io/yaml"
)

// fluxTypes are the kinds the oracle decodes, by apiVersion and kind.
var fluxTypes = map[string]func() metav1.Object{
	"kustomize.toolkit.fluxcd.io/v1/Kustomization":    func() metav1.Object { return &kustomizev1.Kustomization{} },
	"helm.toolkit.fluxcd.io/v2/HelmRelease":           func() metav1.Object { return &helmv2.HelmRelease{} },
	"source.toolkit.fluxcd.io/v1/GitRepository":       func() metav1.Object { return &sourcev1.GitRepository{} },
	"source.toolkit.fluxcd.io/v1/HelmRepository":      func() metav1.Object { return &sourcev1.HelmRepository{} },
	"source.toolkit.fluxcd.io/v1/OCIRepository":       func() metav1.Object { return &sourcev1.OCIRepository{} },
	"source.toolkit.fluxcd.io/v1/Bucket":              func() metav1.Object { return &sourcev1.Bucket{} },
	"source.toolkit.fluxcd.io/v1/HelmChart":           func() metav1.Object { return &sourcev1.HelmChart{} },
	"source.toolkit.fluxcd.io/v1/ExternalArtifact":    func() metav1.Object { return &sourcev1.ExternalArtifact{} },
	"notification.toolkit.fluxcd.io/v1/Receiver":      func() metav1.Object { return &notificationv1.Receiver{} },
	"notification.toolkit.fluxcd.io/v1beta3/Alert":    func() metav1.Object { return &notificationv1beta3.Alert{} },
	"notification.toolkit.fluxcd.io/v1beta3/Provider": func() metav1.Object { return &notificationv1beta3.Provider{} },
}

// fobj is one object of the typed decode.
type fobj struct{ kind, ns, name string }

// typedObjects strictly decodes every document of a Flux file.
func typedObjects(b []byte) ([]fobj, error) {
	var out []fobj
	r := utilyaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(b)))
	for {
		doc, err := r.Read()
		if errors.Is(err, io.EOF) {
			return out, nil
		} else if err != nil {
			return nil, err
		}
		var probe map[string]any
		if err := sigsyaml.Unmarshal(doc, &probe); err != nil {
			return nil, err
		}
		if len(probe) == 0 {
			continue
		}
		av, _ := probe["apiVersion"].(string)
		kind, _ := probe["kind"].(string)
		mk, ok := fluxTypes[av+"/"+kind]
		if !ok {
			return nil, fmt.Errorf("no Flux type for %s %s", av, kind)
		}
		obj := mk()
		if err := sigsyaml.UnmarshalStrict(doc, obj); err != nil {
			return nil, fmt.Errorf("%s %s: %w", av, kind, err)
		}
		out = append(out, fobj{kind, obj.GetNamespace(), obj.GetName()})
	}
}

// emptyKustomizeDir writes an empty kustomization for flux build's --path.
func emptyKustomizeDir(dir string) (string, error) {
	p := filepath.Join(dir, "ox-empty")
	if err := os.MkdirAll(p, 0o755); err != nil {
		return "", err
	}
	return p, os.WriteFile(filepath.Join(p, "kustomization.yaml"), []byte("apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources: []\n"), 0o644)
}

// fluxBuild loads one Kustomization with the flux CLI, offline. An
// "operation not permitted" is retried after ORACLE_TCC_RESET.
func fluxBuild(bin, file, name, ns, path string) error {
	var last error
	for i := 0; i < 6; i++ {
		cmd := exec.Command(bin, "build", "kustomization", name, "-n", ns, "--kustomization-file", file, "--path", path, "--dry-run")
		cmd.Env = append(os.Environ(), "KUBECONFIG=/dev/null")
		b, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}
		last = fmt.Errorf("%v: %s", err, strings.Join(strings.Fields(string(b)), " "))
		if !strings.Contains(strings.ToLower(string(b)), "operation not permitted") {
			return last
		}
		if c := os.Getenv("ORACLE_TCC_RESET"); c != "" {
			_ = exec.Command("sh", "-c", c).Run()
		}
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	return last
}

// writeTierA writes the tier-A definitions to stdout and the walk's tier-C
// references and cross_references to refsOut.
func writeTierA(o out, objs []object, files []string, subject, bin, scratch, refsOut, filesOut string) error {
	ver, err := exec.Command(bin, "version", "--client").Output()
	if err != nil {
		return fmt.Errorf("flux version: %w", err)
	}
	empty, err := emptyKustomizeDir(scratch)
	if err != nil {
		return err
	}
	byFile := map[string][]object{}
	for _, ob := range objs {
		byFile[ob.path] = append(byFile[ob.path], ob)
	}
	var defs []def
	var accepted []string
	rejected, buildFailed, unlocated, walkOnly, built, loadedOnly := 0, 0, 0, 0, 0, 0
	for _, rel := range files {
		abs := filepath.Join(subject, filepath.FromSlash(rel))
		b, err := os.ReadFile(abs)
		if err != nil {
			return err
		}
		typed, err := typedObjects(b)
		if err != nil {
			rejected++
			fmt.Fprintf(os.Stderr, "flux-oracle: the Flux types refuse %s: %.240s\n", rel, err)
			continue
		}
		accepted = append(accepted, rel)
		want := map[fobj]int{}
		for _, t := range typed {
			if t.kind == "Kustomization" {
				ns := t.ns
				if ns == "" {
					ns = "default"
				}
				if err := fluxBuild(bin, abs, t.name, ns, empty); err != nil {
					// The CLI loads the object first and builds its path
					// second: "kustomize build failed" means it read the
					// object (the stand-in path cannot hold the components
					// spec.components names relative to a source the oracle
					// never fetches); anything else refuses the object.
					if !strings.Contains(err.Error(), "kustomize build failed") {
						buildFailed++
						fmt.Fprintf(os.Stderr, "flux-oracle: flux build kustomization %s in %s: %.240s\n", t.name, rel, err)
						continue
					}
					loadedOnly++
				}
				built++
			}
			want[t]++
		}
		for _, ob := range byFile[rel] {
			k := fobj{ob.kind, ob.ns, scalar(at(ob.node, "metadata", "name"))}
			if want[k] > 0 {
				want[k]--
				defs = append(defs, def{ob.path, ob.name, ob.line, "resource"})
			} else {
				walkOnly++
			}
		}
		for k, n := range want {
			if n > 0 {
				unlocated += n
				fmt.Fprintf(os.Stderr, "flux-oracle: unlocated %s %s/%s/%s\n", rel, k.kind, k.ns, k.name)
			}
		}
	}
	fmt.Fprintf(os.Stderr, "flux-oracle: tier A (%s): %d flux files, %d refused by the Flux types; %d Kustomizations loaded by flux build (%d of them loaded but their components not built on the stand-in path), %d refused; %d definitions (%d unlocated, %d walk-only)\n",
		strings.TrimSpace(string(ver)), len(files), rejected, built, loadedOnly, buildFailed, len(defs), unlocated, walkOnly)
	if defs == nil {
		defs = []def{}
	}
	type defsOut struct {
		Tool        string `json:"tool"`
		Version     string `json:"version"`
		Tier        string `json:"tier"`
		Definitions []def  `json:"definitions"`
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(defsOut{
		Tool:        "Flux API types, strict (kustomize-controller api v1.9.6, helm-controller api v1.6.5, source-controller api v1.9.6, notification-controller api v1.9.4) and flux build kustomization --dry-run; lines: gopkg.in/yaml.v3",
		Version:     strings.TrimSpace(string(ver)),
		Tier:        "A",
		Definitions: defs,
	}); err != nil {
		return err
	}
	o.Definitions = []def{}
	var buf bytes.Buffer
	renc := json.NewEncoder(&buf)
	renc.SetIndent("", " ")
	if err := renc.Encode(o); err != nil {
		return err
	}
	if err := os.WriteFile(refsOut, buf.Bytes(), 0o644); err != nil {
		return err
	}
	if filesOut != "" {
		return os.WriteFile(filesOut, []byte(strings.Join(accepted, "\n")+"\n"), 0o644)
	}
	return nil
}
