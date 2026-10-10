package routes

import (
	"reflect"
	"strings"
	"testing"
)

func TestJAXRSRoutes(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "class and method @Path compose; no method @Path is the class path",
			src: `package r;
import jakarta.ws.rs.*;
@Path("/articles")
@Produces(MediaType.APPLICATION_JSON)
public class ArticlesResource {
	@GET
	public Articles list(@QueryParam("tag") String tag) { return null; }
	@GET
	@Path("/{slug}")
	public Article get(@PathParam("slug") String slug) { return null; }
	@DELETE
	@Path("{slug}")
	public void delete(@PathParam("slug") String slug) {}
	@POST
	@Path("/{slug}/favorite")
	@Transactional
	public Article favor(@PathParam("slug") String slug) { return null; }
	@PUT @Path("/{slug}") public Article update(String slug) { return null; }
	@PATCH @Path("/{slug}") public Article patch(String slug) { return null; }
	@HEAD public void head() {}
	@OPTIONS public void options() {}
}`,
			want: []string{
				"DELETE /articles/{slug} ArticlesResource.delete:11",
				"GET /articles ArticlesResource.list:6",
				"GET /articles/{slug} ArticlesResource.get:8",
				"HEAD /articles ArticlesResource.head:20",
				"OPTIONS /articles ArticlesResource.options:21",
				"PATCH /articles/{slug} ArticlesResource.patch:19",
				"POST /articles/{slug}/favorite ArticlesResource.favor:14",
				"PUT /articles/{slug} ArticlesResource.update:18",
			},
		},
		{
			name: "a sub-resource locator and a class with no @Path are not read",
			src: `import javax.ws.rs.*;
@Path("/")
public class Root {
	@Path("users")
	@GET
	public String users() { return ""; }
	@Path("orders")
	public OrdersResource orders() { return new OrdersResource(); }
}
class OrdersResource {
	@GET
	public String list() { return ""; }
}`,
			want: []string{"GET /users Root.users:5"},
		},
		{
			name: "a REST client interface is not a server; a resource interface is",
			src: `import jakarta.ws.rs.*;
@RegisterRestClient(configKey = "users")
@Path("/users")
public interface UsersClient {
	@GET
	@Path("/{id}")
	User get(@PathParam("id") long id);
}
@Path("/greet")
public interface GreetApi {
	@GET
	String hello();
}`,
			want: []string{"GET /greet GreetApi.hello:11"},
		},
		{
			name: "constants fold, regex templates stay, nested resources qualify",
			src: `import jakarta.ws.rs.*;
public class Api {
	public static final String BASE = "/api/v1";
	@Path(Api.BASE + "/items")
	public static class Items {
		private static final String ID = "/{id: [0-9]+}";
		@GET @Path(ID) public Item get(@PathParam("id") long id) { return null; }
		@POST public void add(Item i) {}
		@GET @Path(Other.PATH) public void elsewhere() {}
	}
}`,
			want: []string{"GET /api/v1/items/{id: [0-9]+} Api.Items.get:7", "POST /api/v1/items Api.Items.add:8"},
		},
		{
			name: "a file that never imports ws.rs is not read",
			src:  `@Path("/x") class X { @GET void x() {} }`,
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := javaRoutes(t, readJAXRS, tc.src)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("routes:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(tc.want, "\n  "))
			}
		})
	}
}
