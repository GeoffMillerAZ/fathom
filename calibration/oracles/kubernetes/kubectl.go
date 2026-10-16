package main

// kubectl.go — the tier-A definitions (wave ox, LB-F153): kubectl's own
// decode of each manifest file names the objects and their container images.
//
//	KUBECONFIG=/dev/null kubectl annotate --local --dry-run=client -o json -f <file> ox.example.com/m=1
//
// --local never contacts an API server (no discovery: `kubectl create
// --dry-run=client` needs it and fails offline with "unable to recognize"),
// it decodes built-in kinds and custom resources alike, and a document it
// cannot decode (no kind, malformed YAML) fails the whole file, which then
// contributes nothing and is counted. kubectl prints no source line, so each
// object is placed by an independent gopkg.in/yaml.v3 reader: the document
// root whose kind, namespace and name equal the object's, in document order;
// an image at the container item of the same list and index whose image
// equals kubectl's. An object no document root carries (a List's item) or an
// image the reader cannot place is unlocated, counted on stderr, never
// guessed into the truth.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// kimage is one container image kubectl decoded, by the dialect's list path.
type kimage struct {
	List  string
	Index int
	Image string
}

// kobj is one object of kubectl's decode.
type kobj struct {
	Kind, Namespace, Name string
	Images                []kimage
}

// containerLists are the pod-spec container lists whose images the dialect
// claims as data_source definitions (doc.json): its definition, mirrored.
var containerLists = [][]string{
	{"spec", "containers"},
	{"spec", "initContainers"},
	{"spec", "template", "spec", "containers"},
	{"spec", "template", "spec", "initContainers"},
}

func jget(v any, keys ...string) any {
	for _, k := range keys {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[k]
	}
	return v
}

func jstr(v any) string {
	s, _ := v.(string)
	return s
}

// kubectlObjects reads kubectl's -o json output: one JSON object per
// decoded document, concatenated.
func kubectlObjects(b []byte) ([]kobj, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	var out []kobj
	for {
		var v map[string]any
		if err := dec.Decode(&v); errors.Is(err, io.EOF) {
			return out, nil
		} else if err != nil {
			return nil, err
		}
		o := kobj{Kind: jstr(v["kind"]), Namespace: jstr(jget(v, "metadata", "namespace")), Name: jstr(jget(v, "metadata", "name"))}
		for _, l := range containerLists {
			cs, _ := jget(v, l...).([]any)
			for i, c := range cs {
				if img := jstr(jget(c, "image")); img != "" {
					o.Images = append(o.Images, kimage{List: strings.Join(l, "."), Index: i, Image: img})
				}
			}
		}
		out = append(out, o)
	}
}

// kubectlDecode runs kubectl's offline decode on one file; an error is the
// tool's rejection of the file (its first stderr line).
func kubectlDecode(bin, file string) ([]kobj, error) {
	var stdout, stderr bytes.Buffer
	var err error
	for i := 0; i < 6; i++ {
		cmd := exec.Command(bin, "annotate", "--local", "--dry-run=client", "-o", "json", "-f", file, "ox.example.com/m=1")
		cmd.Env = append(os.Environ(), "KUBECONFIG=/dev/null")
		stdout.Reset()
		stderr.Reset()
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err = cmd.Run()
		// A revoked volume grant reads as "operation not permitted": retried
		// after ORACLE_TCC_RESET, never taken for the tool's rejection.
		if !strings.Contains(strings.ToLower(stderr.String()), "operation not permitted") {
			break
		}
		if c := os.Getenv("ORACLE_TCC_RESET"); c != "" {
			_ = exec.Command("sh", "-c", c).Run()
		}
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	if err != nil {
		line, _, _ := strings.Cut(strings.TrimSpace(stderr.String()), "\n")
		return nil, fmt.Errorf("%v: %s", err, line)
	}
	if s := strings.TrimSpace(stderr.String()); s != "" {
		line, _, _ := strings.Cut(s, "\n")
		return nil, fmt.Errorf("stderr: %s", line)
	}
	return kubectlObjects(stdout.Bytes())
}

// locateStats counts what kubectl decoded and the reader could not place.
type locateStats struct {
	objects, nameless, unlocated, images, imagesUnlocated int
}

func (s *locateStats) add(o locateStats) {
	s.objects += o.objects
	s.nameless += o.nameless
	s.unlocated += o.unlocated
	s.images += o.images
	s.imagesUnlocated += o.imagesUnlocated
}

// ydoc is one document root as the independent reader sees it.
type ydoc struct {
	root           *yaml.Node
	kind, ns, name string
	used           bool
}

// locate places kubectl's objects of one file by an independent yaml.v3 read
// of its source and returns the definitions in the dialect's shape.
func locate(rel string, src []byte, objs []kobj) ([]def, locateStats) {
	var docs []*ydoc
	dec := yaml.NewDecoder(bytes.NewReader(src))
	for {
		var doc yaml.Node
		if err := dec.Decode(&doc); err != nil {
			break
		}
		if len(doc.Content) == 0 {
			continue
		}
		r := doc.Content[0]
		docs = append(docs, &ydoc{root: r, kind: scalar(get(r, "kind")), ns: scalar(path(r, "metadata", "namespace")), name: scalar(path(r, "metadata", "name"))})
	}
	var defs []def
	var st locateStats
	for _, o := range objs {
		st.objects++
		if o.Kind == "" || o.Name == "" {
			st.nameless++
			continue
		}
		var d *ydoc
		for _, c := range docs {
			if !c.used && c.kind == o.Kind && c.ns == o.Namespace && c.name == o.Name {
				d = c
				break
			}
		}
		if d == nil {
			st.unlocated++
			continue
		}
		d.used = true
		qual := o.Kind + "/" + o.Name
		if o.Namespace != "" {
			qual = o.Kind + "/" + o.Namespace + "/" + o.Name
		}
		defs = append(defs, def{rel, qual, d.root.Line, "resource"})
		for _, im := range o.Images {
			st.images++
			cs := seq(path(d.root, strings.Split(im.List, ".")...))
			if im.Index >= len(cs) || scalar(get(cs[im.Index], "image")) != im.Image {
				st.imagesUnlocated++
				continue
			}
			defs = append(defs, def{rel, im.Image, cs[im.Index].Line, "data_source"})
		}
	}
	return defs, st
}

// defsOut is the tier-A definitions file: no references or cross-references
// key at all, so lang-oracle scores nothing else at this file's tier.
type defsOut struct {
	Tool        string `json:"tool"`
	Version     string `json:"version"`
	Tier        string `json:"tier"`
	Definitions []def  `json:"definitions"`
}

// kubectlVersion is the client's gitVersion.
func kubectlVersion(bin string) (string, error) {
	cmd := exec.Command(bin, "version", "--client", "-o", "json")
	cmd.Env = append(os.Environ(), "KUBECONFIG=/dev/null")
	b, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("kubectl version: %w", err)
	}
	var v struct {
		ClientVersion struct {
			GitVersion string `json:"gitVersion"`
		} `json:"clientVersion"`
	}
	if err := json.Unmarshal(b, &v); err != nil || v.ClientVersion.GitVersion == "" {
		return "", fmt.Errorf("kubectl version: unreadable %q", b)
	}
	return v.ClientVersion.GitVersion, nil
}

// walkDisagreement counts the definitions only one of kubectl's decode and
// the walk (o.Definitions, over the same accepted files) holds: disclosed,
// never scored; kubectl's are the truth.
func walkDisagreement(walk, kubectl []def) (onlyWalk, onlyKubectl []def) {
	in := func(ds []def) map[def]bool {
		m := map[def]bool{}
		for _, d := range ds {
			m[d] = true
		}
		return m
	}
	w, k := in(walk), in(kubectl)
	for _, d := range walk {
		if !k[d] {
			onlyWalk = append(onlyWalk, d)
		}
	}
	for _, d := range kubectl {
		if !w[d] {
			onlyKubectl = append(onlyKubectl, d)
		}
	}
	return onlyWalk, onlyKubectl
}

// writeKubectl writes the tier-A definitions to stdout, the walk's tier-C
// references, cross_references and route pairs to refsOut, the accepted
// files to filesOut, and the counts to stderr.
func writeKubectl(o out, kdefs []def, st locateStats, accepted []string, rejected int, bin, refsOut, filesOut string) error {
	ver, err := kubectlVersion(bin)
	if err != nil {
		return err
	}
	onlyWalk, onlyKubectl := walkDisagreement(o.Definitions, kdefs)
	fmt.Fprintf(os.Stderr, "kubernetes-oracle: kubectl %s accepts %d files, rejects %d; %d objects (%d nameless, %d unlocated), %d images (%d unlocated); %d definitions; the walk disagrees on %d (walk only) and %d (kubectl only)\n",
		ver, len(accepted), rejected, st.objects, st.nameless, st.unlocated, st.images, st.imagesUnlocated, len(kdefs), len(onlyWalk), len(onlyKubectl))
	for i, d := range append(onlyWalk, onlyKubectl...) {
		if i == 10 {
			break
		}
		fmt.Fprintf(os.Stderr, "kubernetes-oracle:   disagree %s:%d %s\n", d.Path, d.Line, d.Name)
	}
	if kdefs == nil {
		kdefs = []def{}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(defsOut{Tool: "kubectl annotate --local --dry-run=client -o json (lines: gopkg.in/yaml.v3)", Version: "kubectl " + ver, Tier: "A", Definitions: kdefs}); err != nil {
		return err
	}
	o.Definitions = []def{}
	b, err := encodeOut(o)
	if err != nil {
		return err
	}
	if err := os.WriteFile(refsOut, b, 0o644); err != nil {
		return err
	}
	if filesOut != "" {
		return os.WriteFile(filesOut, []byte(strings.Join(accepted, "\n")+"\n"), 0o644)
	}
	return nil
}
