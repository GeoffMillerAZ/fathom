package techpack

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/capability"
)

// pinnedRepo is a repository that pins two kits, each shipping one tech
// pack, resolved through local replaces (no cache, no network).
func pinnedRepo(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	offlineEnv(t, base)
	t.Setenv("FATHOM_HOME", filepath.Join(base, "fathom-home"))
	repo := filepath.Join(base, "repo")
	writeFile(t, repo, "go.mod", "module example.test/app\n\ngo 1.22\n\nrequire (\n\texample.test/kita v1.0.0\n\texample.test/kitb v1.0.0\n)\n\nreplace (\n\texample.test/kita => ../kita\n\texample.test/kitb => ../kitb\n)\n")
	writeFile(t, base, "kita/go.mod", "module example.test/kita\n\ngo 1.22\n")
	writeFile(t, base, "kita/fathom-pack/pack.cue", packCUE("pack-a", 1))
	writeFile(t, base, "kitb/go.mod", "module example.test/kitb\n\ngo 1.22\n")
	writeFile(t, base, "kitb/fathom-pack/pack.cue", packCUE("pack-b", 1))
	return repo
}

func names(d Discovery) []string {
	var out []string
	for _, p := range d.Packs {
		out = append(out, p.Name)
	}
	return out
}

// TestCapabilityTechPacksAndExcludePacks is FD-20's anchor: the `tech-packs`
// capability is registered, switchable and on by default; with it off the
// served discovery is empty (what a repository with no pack serves) and the
// pins are not read; `exclude_packs` in modules.cue drops the named packs
// from the same one filter; an unreadable list drops nothing and says why.
func TestCapabilityTechPacksAndExcludePacks(t *testing.T) {
	c, ok := capability.Default().Get(CapabilityID)
	if !ok || !c.Switchable || !c.Default {
		t.Fatalf("tech-packs capability: registered=%v switchable=%v default=%v, want a switchable default-on capability", ok, c.Switchable, c.Default)
	}

	t.Run("default serves every discovered pack", func(t *testing.T) {
		repo := pinnedRepo(t)
		d, err := Served(repo)
		if err != nil {
			t.Fatal(err)
		}
		if got := names(d); !reflect.DeepEqual(got, []string{"pack-a", "pack-b"}) {
			t.Fatalf("served %v, want pack-a and pack-b", got)
		}
	})

	t.Run("exclude_packs drops the named pack", func(t *testing.T) {
		repo := pinnedRepo(t)
		writeFile(t, repo, ".fathom/modules.cue", "bindings: []\nexclude_packs: [\"pack-a\", \"pack-a\", \"not-installed\"]\n")
		d, err := Served(repo)
		if err != nil {
			t.Fatal(err)
		}
		if got := names(d); !reflect.DeepEqual(got, []string{"pack-b"}) {
			t.Fatalf("served %v, want only pack-b", got)
		}
		if pol := PolicyFor(repo); !reflect.DeepEqual(pol.Exclude, []string{"not-installed", "pack-a"}) {
			t.Fatalf("policy exclude = %v, want sorted and de-duplicated", pol.Exclude)
		}
	})

	t.Run("capability off serves nothing and reads no pin", func(t *testing.T) {
		repo := pinnedRepo(t)
		writeFile(t, repo, ".fathom/capabilities.cue", "capabilities: \"tech-packs\": on: false\n")
		// A pin file that cannot parse would be an error from Discover; off
		// must not reach it.
		writeFile(t, repo, "go.mod", "this is not a go.mod\n")
		d, err := Served(repo)
		if err != nil {
			t.Fatalf("off must not read the pins: %v", err)
		}
		if !reflect.DeepEqual(d, Discovery{}) {
			t.Fatalf("off served %+v, want the zero Discovery", d)
		}
		if pol := PolicyFor(repo); pol.Enabled || pol.Scope != capability.ScopeProject {
			t.Fatalf("policy = %+v, want disabled by the project scope", pol)
		}
	})

	t.Run("an unreadable list drops nothing and is named", func(t *testing.T) {
		repo := pinnedRepo(t)
		writeFile(t, repo, ".fathom/modules.cue", "exclude_packs: \"pack-a\"\n")
		d, err := Served(repo)
		if err != nil {
			t.Fatal(err)
		}
		if got := names(d); !reflect.DeepEqual(got, []string{"pack-a", "pack-b"}) {
			t.Fatalf("served %v, want both packs", got)
		}
		if PolicyFor(repo).Err == nil {
			t.Fatal("the unreadable exclude_packs was not named")
		}
	})
}

// TestEveryDiscoverCallRoutesThroughServed holds FD-20's "one filter" rule: no
// non-test file outside this package calls Discover except the capability
// probe, which counts what the pins carry before any exclusion.
func TestEveryDiscoverCallRoutesThroughServed(t *testing.T) {
	root := filepath.Join("..", "..")
	allowed := map[string]bool{
		filepath.Join("cmd", "fathom", "capabilityprobe.go"):  true,
		filepath.Join("cmd", "fathom", "doctor_techpacks.go"): true,
	}
	for _, top := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, p)
			if d.IsDir() && (d.Name() == "testdata" || rel == filepath.Join("internal", "techpack")) {
				return fs.SkipDir
			}
			if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") || allowed[rel] {
				return nil
			}
			data, rerr := os.ReadFile(p)
			if rerr != nil {
				return rerr
			}
			if strings.Contains(string(data), "techpack.Discover(") && !strings.HasPrefix(strings.TrimSpace(string(data)), "//") {
				for _, line := range strings.Split(string(data), "\n") {
					if strings.Contains(line, "techpack.Discover(") && !strings.HasPrefix(strings.TrimSpace(line), "//") {
						t.Errorf("%s calls techpack.Discover directly; route it through techpack.Served", rel)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// TestPolicyForLoadsProjectOnlyWhenExcludePacksIsWritten pins the policy's
// cost on the obligations path (semantic's discoverTechPacks runs it per
// call; pkg/client's 2 MiB per-call allocation target): a `.fathom/` that
// never writes `exclude_packs` is not evaluated as CUE, while a file that
// does still yields the list.
func TestPolicyForLoadsProjectOnlyWhenExcludePacksIsWritten(t *testing.T) {
	repo := pinnedRepo(t)
	writeFile(t, repo, ".fathom/guard.cue", `guards: [{scopes: ["**"], note: "n", max_rules: 1}]`+"\n")
	PolicyFor(repo) // warm package-level state
	allocs := testing.AllocsPerRun(5, func() { PolicyFor(repo) })
	if allocs > 500 {
		t.Fatalf("PolicyFor without exclude_packs allocated %.0f times per call, want the project left unevaluated", allocs)
	}
	writeFile(t, repo, ".fathom/modules.cue", `exclude_packs: ["pack-a"]`+"\n")
	if pol := PolicyFor(repo); pol.Err != nil || !reflect.DeepEqual(pol.Exclude, []string{"pack-a"}) {
		t.Fatalf("policy = %+v, want exclude [pack-a]", pol)
	}
}
