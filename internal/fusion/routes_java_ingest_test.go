package fusion

import (
	"context"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/syslink"
)

// javaRoutesCorpus is a small Java web app: a Spring controller under a
// class-level prefix (one handler in a nested class, one path through a
// constant) and a JAX-RS resource.
func javaRoutesCorpus() map[string]string {
	return map[string]string{
		"src/main/java/app/owner/PetController.java": `package app.owner;

import org.springframework.stereotype.Controller;
import org.springframework.web.bind.annotation.*;

@Controller
@RequestMapping("/owners/{ownerId}")
class PetController {
	static final String NEW = "/pets/new";

	@GetMapping(NEW)
	public String initCreationForm() { return "pets/createOrUpdatePetForm"; }

	@PostMapping("/pets/{petId}/edit")
	public String processUpdateForm() { return "redirect:/owners/{ownerId}"; }

	@RestController
	static class Api {
		@DeleteMapping("/api/pets/{petId}")
		void remove() {}
	}
}
`,
		"src/main/java/app/tags/TagsResource.java": `package app.tags;

import jakarta.ws.rs.GET;
import jakarta.ws.rs.Path;

@Path("/tags")
public class TagsResource {
	@GET
	@Path("/{name}")
	public String tag() { return ""; }
}
`,
	}
}

// LB-F322, LB-F323: an ingest stores a Java app's Spring and JAX-RS routes;
// each stored route is a route: key and binds to an indexed JavaMethod by
// its handler name.
func TestIngest_JavaFrameworkRoutes(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, javaRoutesCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "jroutes", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "jroutes", "")
	if err != nil {
		t.Fatal(err)
	}
	methods := map[string]bool{}
	for _, o := range obs {
		if o.SourceKind == core.KindJavaMethod {
			methods[o.Symbol] = true
		}
	}
	refs, complete, err := st.ServiceRefsFor(ctx, "jroutes")
	if err != nil || !complete {
		t.Fatalf("ServiceRefsFor: complete=%v err=%v", complete, err)
	}
	got := map[string]string{}
	for _, r := range serviceref.RoutesFromRows(refs.Routes) {
		key, err := syslink.ParseContractKey("route:" + r.Method + " " + r.Path)
		if err != nil {
			t.Fatalf("route %+v is no route: key: %v", r, err)
		}
		got[key.Body] = r.Handler
		if !methods[r.Handler] {
			var have []string
			for m := range methods {
				have = append(have, m)
			}
			sort.Strings(have)
			t.Errorf("route %s handler %q is no indexed JavaMethod (have %q)", key.Body, r.Handler, have)
		}
	}
	want := map[string]string{
		"GET /owners/{}/pets/new":      "PetController.initCreationForm",
		"POST /owners/{}/pets/{}/edit": "PetController.processUpdateForm",
		"DELETE /api/pets/{}":          "PetController.Api.remove", // a nested controller has its own prefix
		"GET /tags/{}":                 "TagsResource.tag",
	}
	if len(got) != len(want) {
		t.Fatalf("stored routes = %v, want %v", got, want)
	}
	for body, h := range want {
		if got[body] != h {
			t.Errorf("route:%s handler = %q, want %q (stored %v)", body, got[body], h, got)
		}
	}
}
