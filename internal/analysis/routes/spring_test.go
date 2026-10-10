package routes

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// renderRoutes renders routes as "METHOD path handler:line", sorted.
func renderRoutes(rs []Route) []string {
	var out []string
	for _, r := range rs {
		out = append(out, fmt.Sprintf("%s %s %s:%d", r.Method, r.Path, r.Handler, r.Line))
	}
	sort.Strings(out)
	return out
}

// javaRoutes reads src with read directly and through the registry
// (ExtractFile, every java reader), and requires the two to agree: the cases
// of one framework's table are invisible to the other's reader.
func javaRoutes(t *testing.T, read func(string, []byte) Result, src string) []string {
	t.Helper()
	direct := renderRoutes(read("src/Ctl.java", []byte(src)).Routes)
	viaRegistry := renderRoutes(ExtractFile("java", "src/Ctl.java", []byte(src)).Routes)
	if !reflect.DeepEqual(direct, viaRegistry) {
		t.Fatalf("the reader read\n  %v\nbut ExtractFile read\n  %v", direct, viaRegistry)
	}
	return direct
}

func TestSpringRoutes(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "class prefix composes with each method mapping",
			src: `package x;
@Controller
@RequestMapping("/owners/{ownerId}")
class PetController {
	@ModelAttribute("types")
	public Collection<PetType> types() { return null; }

	@GetMapping("/pets/new")
	public String initCreationForm(Owner owner, ModelMap model) { return "x"; }

	@PostMapping("/pets/new")
	public String processCreationForm(@Valid Pet pet, BindingResult result) { return "y"; }
}`,
			want: []string{
				"GET /owners/{ownerId}/pets/new PetController.initCreationForm:8",
				"POST /owners/{ownerId}/pets/new PetController.processCreationForm:11",
			},
		},
		{
			name: "RequestMapping method element, single and array; none is ANY",
			src: `@RestController
@RequestMapping(path = "/api")
public class ItemController {
	@RequestMapping(value = "/items", method = RequestMethod.POST)
	public void create() {}
	@RequestMapping(value = "/items/{id}", method = {RequestMethod.GET, RequestMethod.HEAD})
	public Item get(@PathVariable("id") long id) { return null; }
	@RequestMapping(path = "/any", produces = "application/json")
	public void any() {}
	@RequestMapping(value = "/static", method = GET)
	public void staticImport() {}
}`,
			want: []string{
				"ANY /api/any ItemController.any:8",
				"GET /api/items/{id} ItemController.get:6",
				"GET /api/static ItemController.staticImport:10",
				"HEAD /api/items/{id} ItemController.get:6",
				"POST /api/items ItemController.create:4",
			},
		},
		{
			name: "array paths on the type and the method multiply",
			src: `@RestController
@RequestMapping({"/v1", "/v2"})
class A {
	@GetMapping({ "/a", "/b" })
	String m() { return ""; }
}`,
			want: []string{"GET /v1/a A.m:4", "GET /v1/b A.m:4", "GET /v2/a A.m:4", "GET /v2/b A.m:4"},
		},
		{
			name: "an empty or absent path is the prefix alone, or the root",
			src: `@RestController
@RequestMapping("/vets")
class Vets {
	@GetMapping
	String list() { return ""; }
	@PostMapping("")
	String add() { return ""; }
}
@Controller
class Welcome {
	@GetMapping
	String welcome() { return ""; }
	@GetMapping("/")
	String root() { return ""; }
}`,
			want: []string{"GET / Welcome.root:13", "GET / Welcome.welcome:11", "GET /vets Vets.list:4", "POST /vets Vets.add:6"},
		},
		{
			name: "paths without a leading slash gain one",
			src: `@Controller
@RequestMapping("owners")
class O {
	@GetMapping("new")
	String n() { return ""; }
}`,
			want: []string{"GET /owners/new O.n:4"},
		},
		{
			name: "String constants declared in the file fold; others drop the route",
			src: `@RestController
@RequestMapping(Paths.BASE)
class C {
	static final String ITEM = Paths.ITEMS + "/{id}";
	private static final String NOT_FINAL_LOOKALIKE = "x";
	static String mutable = "/mutable";

	interface Paths {
		String BASE = "/api";
		String ITEMS = "/items";
	}

	@GetMapping(ITEM)
	Item get() { return null; }
	@GetMapping(value = Paths.ITEMS + "/" + "count")
	int count() { return 0; }
	@GetMapping(Elsewhere.PATH)
	void unknown() {}
	@GetMapping(mutable)
	void notConstant() {}
}`,
			want: []string{"GET /api/items/count C.count:15", "GET /api/items/{id} C.get:13"},
		},
		{
			name: "nested types qualify as the java dialect names them",
			src: `public class Outer {
	@RestController
	@RequestMapping("/in")
	public static class Inner {
		@DeleteMapping("/{id}")
		public void remove() {}
		@PutMapping("/{id}")
		public void replace() {}
		@PatchMapping("/{id}")
		public void patch() {}
	}
	@GetMapping("/outer")
	public void outer() {}
}`,
			want: []string{
				"DELETE /in/{id} Outer.Inner.remove:5",
				"GET /outer Outer.outer:12",
				"PATCH /in/{id} Outer.Inner.patch:9",
				"PUT /in/{id} Outer.Inner.replace:7",
			},
		},
		{
			name: "comments, strings and bodies never make a route",
			src: `@RestController
class Quiet {
	// @GetMapping("/commented")
	/* @GetMapping("/blocked") */
	private final String s = "@GetMapping(\"/string\")";
	private final Runnable r = () -> { new Object() { @GetMapping("/anon") void a() {} }; };
	void body() {
		class Local { @GetMapping("/local") void l() {} }
		String t = """
			@GetMapping("/textblock")
			""";
	}
	@GetMapping("/real")
	void real() {}
}`,
			want: []string{"GET /real Quiet.real:13"},
		},
		{
			name: "a Feign client is not a server; an API interface is",
			src: `@FeignClient(name = "users")
interface UsersClient {
	@GetMapping("/users/{id}")
	User get(@PathVariable("id") long id);
}
@RequestMapping("/pets")
public interface PetsApi {
	@GetMapping(value = "/{petId}", produces = { "application/json" })
	default ResponseEntity<Pet> showPetById(@PathVariable("petId") String petId) { return null; }
	@PostMapping
	ResponseEntity<Void> createPets(@RequestBody Pet pet);
}`,
			want: []string{"GET /pets/{petId} PetsApi.showPetById:8", "POST /pets PetsApi.createPets:10"},
		},
		{
			name: "enums, records, generics and annotation types around a controller",
			src: `package p;
import java.util.*;
@interface Marker { String value() default ""; }
enum Kind { A("a") { @Override String x() { return "1"; } }, B("b"); Kind(String s) {} String x() { return ""; } }
record Pair(int a, int b) { Pair { if (a < 0) throw new IllegalArgumentException(); } }
@RestController
@RequestMapping(value = "/g", headers = "X-A=1")
class G<T extends Comparable<T>> {
	private final Map<String, List<T>> m = new HashMap<>();
	@Marker("ignored")
	@GetMapping(path = "/{k}", params = {"a", "b"})
	public <R> List<R> get(@PathVariable String k) throws java.io.IOException { return null; }
	static { System.out.println("init"); }
	@SuppressWarnings({"unchecked", "rawtypes"})
	@RequestMapping(value = "/h", method = RequestMethod.PUT)
	public @ResponseBody String h() { return ""; }
}`,
			want: []string{"GET /g/{k} G.get:11", "PUT /g/h G.h:15"},
		},
		{
			name: "a type-level method element joins every mapping's method",
			src: `@Controller
@RequestMapping(value = "/f", method = RequestMethod.POST)
class F {
	@RequestMapping("/a")
	void a() {}
	@GetMapping("/b")
	void b() {}
}`,
			want: []string{"GET /f/b F.b:6", "POST /f/a F.a:4", "POST /f/b F.b:6"},
		},
		{
			name: "a mapping inside a string is no route",
			src:  `class Plain { @Override public String toString() { return "@GetMapping(\"/x\")"; } }`,
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := javaRoutes(t, readSpring, tc.src)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("routes:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(tc.want, "\n  "))
			}
		})
	}
}

// The java language token reaches the Spring and JAX-RS readers through the
// registry, and ExtractFile stays deterministic over them.
func TestSpringRoutes_RegistryAndDeterminism(t *testing.T) {
	if !HasReader("java") {
		t.Fatal("HasReader(java) = false")
	}
	src := []byte("@RestController class A { @GetMapping(\"/a\") void a() {} @PostMapping(\"/a\") void b() {} }")
	a, b := ExtractFile("java", "A.java", src).Routes, ExtractFile("java", "A.java", src).Routes
	if !reflect.DeepEqual(a, b) || len(a) != 2 {
		t.Fatalf("ExtractFile = %v then %v", a, b)
	}
	if a[0].File != "A.java" {
		t.Fatalf("File = %q", a[0].File)
	}
}
