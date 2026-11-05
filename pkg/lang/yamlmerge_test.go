package lang_test

import (
	"strings"
	"testing"
	"time"
)

// A mapping that merges itself, or merges that fan out and meet again, cost a
// step for every path through them: a 52-byte Chart.yaml and an 86-byte
// manifest never returned. A lookup reads a mapping once.
func TestYAMLMergeGraphsCostTheSizeOfTheDocument(t *testing.T) {
	r := newReader(t)
	for name, src := range map[string]string{
		"chart":    "x: &a {<<: [*a,*a,*a]}\n<<: *a\nname: x\nversion: 1.0.0\n",
		"manifest": "a: &a {<<: [*a,*a,*a]}\napiVersion: v1\nkind: Pod\nmetadata: {name: p}\nspec: *a\nx: 1\nx: 2\n",
	} {
		path := "k/pod.yaml"
		if name == "chart" {
			path = "chart/Chart.yaml"
		}
		t.Run(name, func(t *testing.T) {
			within(t, 20*time.Second, name, func() { extract(t, r, path, src) })
		})
	}
	// A non-cyclic graph of 30 anchors, each merging the one before it twice, has
	// 2^30 paths and 30 nodes.
	var b strings.Builder
	b.WriteString("apiVersion: v1\nkind: Pod\nmetadata:\n  name: p\n")
	b.WriteString("m0: &m0 {z: 1}\n")
	for i := 1; i < 30; i++ {
		b.WriteString("m" + itoa(i) + ": &m" + itoa(i) + " {<<: [*m" + itoa(i-1) + ", *m" + itoa(i-1) + "]}\n")
	}
	b.WriteString("spec: *m29\nx: 1\nx: 2\n")
	within(t, 20*time.Second, "diamond chain", func() { extract(t, r, "k/pod.yaml", b.String()) })
}

// A merge reaches the key through a chain of anchors, and the chain's bound is
// wide enough for a real one: a bound of 2 loses the container.
func TestYAMLMergeChainOfTwentyIsRead(t *testing.T) {
	r := newReader(t)
	var b strings.Builder
	b.WriteString("apiVersion: v1\nkind: Pod\nmetadata:\n  name: p\n")
	b.WriteString("m0: &m0\n  containers:\n  - name: app\n    image: reg/deep:1\n")
	for i := 1; i <= 20; i++ {
		b.WriteString("m" + itoa(i) + ": &m" + itoa(i) + " {<<: *m" + itoa(i-1) + "}\n")
	}
	b.WriteString("spec: *m20\nx: 1\nx: 2\n")
	got := extract(t, r, "k/pod.yaml", b.String())
	for _, s := range got.Symbols {
		if s.Kind == "K8sContainer" && s.Name == "reg/deep:1" {
			return
		}
	}
	t.Fatalf("the container behind a 20-merge chain is lost: incomplete=%v notes=%v", got.Incomplete, got.Notes)
}

func itoa(i int) string {
	const d = "0123456789"
	if i < 10 {
		return d[i : i+1]
	}
	return itoa(i/10) + d[i%10:i%10+1]
}

// `namespace: null` is no namespace, as yaml.v3's decode reads it, so the node
// reader (a repeated key routes the document to it) names the same workload.
// An alias key is the scalar it names: with `q: &spec other` the key `*spec` is
// `other`, and the real `spec` is not hidden by a block under another name.
func TestNodeReaderAgreesWithDecode(t *testing.T) {
	r := newReader(t)
	names := func(src string) []string {
		var out []string
		for _, s := range extract(t, r, "k/pod.yaml", src).Symbols {
			if s.Kind == "K8sContainer" || s.Kind == "K8sWorkload" {
				out = append(out, s.Name)
			}
		}
		return out
	}
	const pod = "apiVersion: v1\nkind: Pod\nmetadata:\n  name: web\n  namespace: null\nspec:\n  containers:\n  - name: a\n    image: reg/a:1\n"
	decode, node := names(pod), names(pod+"x: 1\nx: 2\n")
	if strings.Join(decode, ",") != strings.Join(node, ",") || len(decode) == 0 {
		t.Errorf("namespace: null reads differently through the node reader: decode=%v node=%v", decode, node)
	}
	for _, n := range node {
		if strings.Contains(n, "null") {
			t.Errorf("null read as a namespace: %v", node)
		}
	}
	hide := "apiVersion: v1\nkind: Pod\nmetadata:\n  name: web\nq: &spec other\n" +
		"spec:\n  containers:\n  - name: a\n    image: reg/a:1\n" +
		"*spec : {containers: [{name: x, image: reg/wrong:1}]}\nx: 1\nx: 2\n"
	var imgs []string
	for _, s := range extract(t, r, "k/pod.yaml", hide).Symbols {
		if s.Kind == "K8sContainer" {
			imgs = append(imgs, s.Name)
		}
	}
	if strings.Join(imgs, ",") != "reg/a:1" {
		t.Errorf("an alias key named for its anchor hides the real container: %v", imgs)
	}
}
