package fusion

// hcltools_dialect_test.go — LB-F143, LB-F144: the native HCL tool dialect
// (Packer, Nomad, docker-bake) is reached by ingest (a manifest basename,
// extension or glob, or a .hcl whose top-level block is job), ReingestFile
// decides as ingest does, an .hcl no sniff claims is never extracted (counted
// ambiguous), and a Nomad task's config.image reaches the image-tag index: it
// resolves, through a docker-bake target's tag (fusion.BakeBuilds, read once,
// by imageref.go), to the Dockerfile that bake target builds.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
)

func hclToolsCorpus() map[string]string {
	return map[string]string{
		"images/app.pkr.hcl": `variable "tag" {
  default = "latest"
}

source "docker" "base" {
  image = "ubuntu:22.04"
}

build {
  sources = ["source.docker.base"]

  provisioner "shell" {
    inline = ["echo ${var.tag}"]
  }
}
`,
		"images/vars.pkrvars.hcl": "tag = \"v1\"\n",
		"deploy/web.nomad": `job "web" {
  group "app" {
    task "server" {
      driver = "docker"
      config {
        image = "example.com/web:1.0"
      }
    }
  }
}
`,
		"deploy/api.nomad.hcl": `job "api" {
  group "app" {
    task "server" {
      driver = "docker"
    }
  }
}
`,
		// a plain .hcl whose top-level block is job: a Nomad jobspec by content
		"deploy/batch.hcl": "job \"batch\" {\n  type = \"batch\"\n}\n",
		"bake/docker-bake.hcl": `target "web" {
  context    = "."
  dockerfile = "Dockerfile"
  tags       = ["example.com/web:1.0"]
}

group "default" {
  targets = ["web"]
}
`,
		"bake/Dockerfile": "FROM scratch\n",
		// .hcl no sniff claims: a Vault configuration and a Consul one
		"vault.hcl":  "ui = true\n\nlistener \"tcp\" {\n  address = \"0.0.0.0:8200\"\n}\n",
		"consul.hcl": "datacenter = \"dc1\"\n\nservice {\n  name = \"web\"\n}\n",
	}
}

// TestIngest_HCLToolsDialect: ingest extracts every Packer, Nomad and bake
// file through the hcl-tools dialect, never a Vault or Consul configuration
// (counted ambiguous), and a one-file refresh reads a globbed and a sniffed
// file through the same dialect.
func TestIngest_HCLToolsDialect(t *testing.T) {
	p, st, dir, rep := ingestStored(t, "ht", hclToolsCorpus())
	ctx := context.Background()
	if rep.AmbiguousUnclassified != 2 {
		t.Errorf("AmbiguousUnclassified = %d, want 2 (vault.hcl, consul.hcl)", rep.AmbiguousUnclassified)
	}
	obs, err := st.QueryObservations(ctx, "ht", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, o := range obs {
		got[o.SourceRef+"|"+o.Symbol] = string(o.SourceKind)
		if o.SourceRef == "vault.hcl" || o.SourceRef == "consul.hcl" {
			t.Errorf("%s|%s: an unclaimed .hcl was extracted as %s", o.SourceRef, o.Symbol, o.SourceKind)
		}
	}
	for key, kind := range map[string]string{
		"images/app.pkr.hcl|source.docker.base":      "PackerSource",
		"images/app.pkr.hcl|build":                   "PackerBuild",
		"images/app.pkr.hcl|build.provisioner.shell": "PackerProvisioner",
		"images/vars.pkrvars.hcl|tag":                "PackerVarValue",
		"deploy/web.nomad|web":                       "NomadJob",
		"deploy/web.nomad|web.app.server":            "NomadTask",
		"deploy/api.nomad.hcl|api.app.server":        "NomadTask",
		"deploy/batch.hcl|batch":                     "NomadJob",
		"bake/docker-bake.hcl|web":                   "BakeTarget",
		"bake/docker-bake.hcl|group.default":         "BakeGroup",
	} {
		if got[key] != kind {
			t.Errorf("%s = %q, want %q", key, got[key], kind)
		}
	}
	calls := map[string][]string{}
	for _, o := range obs {
		calls[o.SourceRef+"|"+o.Symbol] = o.Calls
	}
	if c := calls["images/app.pkr.hcl|build"]; len(c) != 1 || c[0] != "source.docker.base" {
		t.Errorf("build calls %q, want [source.docker.base]", c)
	}
	if c := calls["bake/docker-bake.hcl|group.default"]; len(c) != 1 || c[0] != "web" {
		t.Errorf("group.default calls %q, want [web]", c)
	}

	// the task image resolves to the Dockerfile the bake target builds
	edges := storedEdges(t, st, "ht")
	want := "deploy/web.nomad|ht::deploy/web.web.app.server->ht::bake/Dockerfile.scratch"
	if !containsString(edges, want) {
		t.Errorf("stored edges lack %q; have %d: %q", want, len(edges), edges)
	}

	// ReingestFile decides as ingest does.
	for rel, body := range map[string]string{
		"images/app.pkr.hcl": "variable \"edited\" {}\n",
		"deploy/batch.hcl":   "job \"edited\" {\n}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := p.ReingestFile(ctx, st, "ht", dir, rel); err != nil {
			t.Fatal(err)
		}
	}
	obs, err = st.QueryObservations(ctx, "ht", "")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, o := range obs {
		seen[o.SourceRef+"|"+o.Symbol] = true
	}
	for _, key := range []string{"images/app.pkr.hcl|var.edited", "deploy/batch.hcl|edited"} {
		if !seen[key] {
			t.Errorf("after ReingestFile: %s missing", key)
		}
	}
	if seen["images/app.pkr.hcl|build"] || seen["deploy/batch.hcl|batch"] {
		t.Error("after ReingestFile: an edited file's old entities survived")
	}
}

// TestHCLToolsDialectClassification: the manifest's match blocks classify
// each file as the dispatch expects.
func TestHCLToolsDialectClassification(t *testing.T) {
	rules := dialectRules()
	src := hclToolsCorpus()
	for path, want := range map[string]struct{ dialect, by string }{
		"images/app.pkr.hcl":            {"hcl-tools", connector.ClassifiedByGlob},
		"images/vars.pkrvars.hcl":       {"hcl-tools", connector.ClassifiedByGlob},
		"deploy/web.nomad":              {"hcl-tools", connector.ClassifiedByExtension},
		"deploy/api.nomad.hcl":          {"hcl-tools", connector.ClassifiedByGlob},
		"deploy/batch.hcl":              {"hcl-tools", connector.ClassifiedBySniff},
		"bake/docker-bake.hcl":          {"hcl-tools", connector.ClassifiedByBasename},
		"bake/docker-bake.override.hcl": {"hcl-tools", connector.ClassifiedByBasename},
		"vault.hcl":                     {"", connector.ClassifiedAmbiguous},
		"consul.hcl":                    {"", connector.ClassifiedAmbiguous},
	} {
		v := connector.ClassifyDialect(rules, path, []byte(src[path]))
		if v.Dialect != want.dialect || v.By != want.by {
			t.Errorf("ClassifyDialect(%q) = %q by %q, want %q by %q", path, v.Dialect, v.By, want.dialect, want.by)
		}
	}
	for path, want := range map[string]string{"bake/docker-bake.hcl": "hcl-tools", "a/job.nomad": "hcl-tools"} {
		if got := connector.LanguageForPath(path); got != want {
			t.Errorf("LanguageForPath(%q) = %q, want %q", path, got, want)
		}
	}
}
