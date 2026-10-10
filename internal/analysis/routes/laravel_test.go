package routes

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// routeLines renders routes as sorted "METHOD /path Handler" lines.
func routeLines(rs []Route) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, strings.TrimSpace(r.Method+" "+r.Path+" "+r.Handler))
	}
	sort.Strings(out)
	return out
}

// mountLines renders mounts as sorted "prefix -> target" lines.
func mountLines(ms []Mount) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, fmt.Sprintf("%s -> %s", m.Prefix, m.Target))
	}
	sort.Strings(out)
	return out
}

func sortedCopy(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}

func TestReadLaravel(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		src    string
		routes []string
		mounts []string
	}{
		{
			name: "verbs and handler forms",
			path: "routes/web.php",
			src: `<?php
use App\Http\Controllers\PhotoController;
use App\Http\Controllers\Api\{LambdaSongController as S3Song, TagController};
use App\Http\Controllers\Auth\DisableController as DisableTwoFactor;
use Illuminate\Support\Facades\Route;

Route::get('/', [PhotoController::class, 'index'])->name('home');
Route::post('photos', 'App\Http\Controllers\PhotoController@store');
Route::put('photos/{id}', PhotoController::class);
Route::patch('/photos/{id}/tags', fn () => 'ok');
Route::get('ping', static fn () => null);
Route::delete('photos/{id?}', function ($id) { return Route::get('/not-a-route', 'X@y'); });
Route::options('photos', [PhotoController::class, 'options']);
Route::any('ping', [PhotoController::class, 'ping']);
Route::match(['get', 'head', 'post'], 'search', [PhotoController::class, 'search']);
Route::view('/about', 'about');
Route::post('song', [S3Song::class, 'put']);
Route::delete('me/two-factor', DisableTwoFactor::class);
Route::redirect('/old', '/new');
// Route::get('/commented', 'X@y');
$s = "Route::get('/in-a-string', 'X@y');";
Route::get(config('app.path'), 'X@dynamic');
`,
			routes: []string{
				"ANY /old RedirectController.__invoke",
				"ANY /ping PhotoController.ping",
				"DELETE /photos/{id?}",
				"GET / PhotoController.index",
				"GET /about ViewController.__invoke",
				"POST /song LambdaSongController.put",
				"DELETE /me/two-factor DisableController.__invoke",
				"GET /ping",
				"GET /search PhotoController.search",
				"OPTIONS /photos PhotoController.options",
				"PATCH /photos/{id}/tags",
				"POST /photos PhotoController.store",
				"POST /search PhotoController.search",
				"PUT /photos/{id} PhotoController.__invoke",
			},
		},
		{
			name: "nested groups, prefix and controller",
			path: "routes/api.php",
			src: `<?php
Route::prefix('v1')->middleware('auth')->group(function () {
    Route::group(['prefix' => 'admin', 'middleware' => ['can:admin']], function () {
        Route::get('users', [UserController::class, 'index']);
        Route::controller(OrderController::class)->prefix('orders')->group(function () {
            Route::get('/{order}', 'show');
            Route::post('/', 'store');
        });
    });
    Route::name('pub.')->group(static function (): void {
        Route::get('status', StatusController::class);
    });
});
Route::get('outside', [HomeController::class, 'outside']);
`,
			routes: []string{
				"GET /outside HomeController.outside",
				"GET /v1/admin/orders/{order} OrderController.show",
				"GET /v1/admin/users UserController.index",
				"GET /v1/status StatusController.__invoke",
				"POST /v1/admin/orders OrderController.store",
			},
		},
		{
			name: "resource expansion with only, except and nesting",
			path: "routes/web.php",
			src: `<?php
Route::resource('photos', PhotoController::class);
Route::apiResource('tags', TagController::class)->only(['index', 'show']);
Route::resource('albums', AlbumController::class, ['except' => ['create', 'edit', 'destroy']]);
Route::resource('photos.comments', CommentController::class)->except('create', 'edit');
Route::prefix('admin')->group(function () {
    Route::apiResources(['users' => UserController::class]);
    Route::singleton('profile', ProfileController::class);
});
`,
			routes: []string{
				"DELETE /admin/users/{users} UserController.destroy",
				"DELETE /photos/{photos} PhotoController.destroy",
				"DELETE /photos/{photos}/comments/{comments} CommentController.destroy",
				"GET /admin/profile ProfileController.show",
				"GET /admin/profile/edit ProfileController.edit",
				"GET /admin/users UserController.index",
				"GET /admin/users/{users} UserController.show",
				"GET /albums AlbumController.index",
				"GET /albums/{albums} AlbumController.show",
				"GET /photos PhotoController.index",
				"GET /photos/create PhotoController.create",
				"GET /photos/{photos} PhotoController.show",
				"GET /photos/{photos}/edit PhotoController.edit",
				"GET /photos/{photos}/comments CommentController.index",
				"GET /photos/{photos}/comments/{comments} CommentController.show",
				"GET /tags TagController.index",
				"GET /tags/{tags} TagController.show",
				"PATCH /admin/profile ProfileController.update",
				"PATCH /admin/users/{users} UserController.update",
				"PATCH /albums/{albums} AlbumController.update",
				"PATCH /photos/{photos} PhotoController.update",
				"PATCH /photos/{photos}/comments/{comments} CommentController.update",
				"POST /admin/users UserController.store",
				"POST /albums AlbumController.store",
				"POST /photos PhotoController.store",
				"POST /photos/{photos}/comments CommentController.store",
				"PUT /admin/profile ProfileController.update",
				"PUT /admin/users/{users} UserController.update",
				"PUT /albums/{albums} AlbumController.update",
				"PUT /photos/{photos} PhotoController.update",
				"PUT /photos/{photos}/comments/{comments} CommentController.update",
			},
		},
		{
			name: "a provider mounts route files under its group prefix",
			path: "app/Providers/RouteServiceProvider.php",
			src: `<?php
namespace App\Providers;

class RouteServiceProvider extends ServiceProvider
{
    public function boot(): void
    {
        $this->routes(function () {
            Route::middleware('api')->prefix('api')->group(base_path('routes/api.php'));
            Route::middleware('web')->group(base_path('routes/web.php'));
        });
    }

    protected function mapAdminRoutes(): void
    {
        Route::group(['prefix' => 'admin', 'namespace' => $this->namespace], function ($router) {
            require base_path('routes/admin.php');
        });
    }
}
`,
			mounts: []string{
				"/ -> /routes/web.php",
				"/admin -> /routes/admin.php",
				"/api -> /routes/api.php",
			},
		},
		{
			name: "bootstrap/app.php withRouting",
			path: "bootstrap/app.php",
			src: `<?php
return Application::configure(basePath: dirname(__DIR__))
    ->withRouting(
        web: __DIR__.'/../routes/web.php',
        api: __DIR__.'/../routes/api.php',
        apiPrefix: 'api/v1',
        commands: __DIR__.'/../routes/console.php',
        health: '/up',
        then: function () {
            Route::prefix('webhooks')->group(base_path('routes/webhooks.php'));
        },
    )
    ->withMiddleware(function (Middleware $middleware) {})
    ->create();
`,
			mounts: []string{
				"/ -> ../routes/web.php",
				"/api/v1 -> ../routes/api.php",
				"/webhooks -> /routes/webhooks.php",
			},
		},
		{
			name: "a loop over a literal array is unrolled",
			path: "routes/subsonic.php",
			src: `<?php
$endpoints = [
    'ping' => PingController::class,
    'getAlbum' => GetAlbumController::class,
];
Route::prefix('rest')->group(static function () use ($endpoints): void {
    foreach ($endpoints as $endpoint => $controller) {
        Route::match(['get', 'post'], "{$endpoint}{format?}", $controller);
    }
});
foreach (['a', 'b'] as $v) {
    Route::get("letters/$v", [LetterController::class, $v]);
}
`,
			routes: []string{
				"GET /letters/a LetterController.a",
				"GET /letters/b LetterController.b",
				"GET /rest/getAlbum{format?} GetAlbumController.__invoke",
				"GET /rest/ping{format?} PingController.__invoke",
				"POST /rest/getAlbum{format?} GetAlbumController.__invoke",
				"POST /rest/ping{format?} PingController.__invoke",
			},
		},
		{
			name: "require inside a prefix group in a route file mounts relative",
			path: "routes/web.php",
			src: `<?php
Route::prefix('auth')->group(function () {
    require __DIR__.'/auth.php';
});
require __DIR__ . '/channels-http.php';
`,
			mounts: []string{
				"/ -> ./channels-http.php",
				"/auth -> ./auth.php",
			},
		},
		{
			name: "no Route facade, nothing read",
			path: "app/Models/User.php",
			src:  `<?php class User { public function get() { return $this->router; } }`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := readLaravel(c.path, []byte(c.src))
			if got := routeLines(res.Routes); !reflect.DeepEqual(got, sortedCopy(c.routes)) && !(len(got) == 0 && len(c.routes) == 0) {
				t.Errorf("routes:\n got  %q\n want %q", got, sortedCopy(c.routes))
			}
			if got := mountLines(res.Mounts); !reflect.DeepEqual(got, sortedCopy(c.mounts)) && !(len(got) == 0 && len(c.mounts) == 0) {
				t.Errorf("mounts:\n got  %q\n want %q", got, sortedCopy(c.mounts))
			}
			for _, r := range res.Routes {
				if r.File != c.path || r.Line <= 0 {
					t.Errorf("route %+v: want File %q and a line", r, c.path)
				}
			}
		})
	}
}

// The PHP lexer keeps line numbers through comments, heredocs and strings,
// and reads an attribute group as one token.
func TestLexPHPLinesAndAttributes(t *testing.T) {
	src := "<html>\n<?php\n/* a\n b */\n$x = <<<EOT\nline\nEOT;\n#[Route('/a')]\n# comment\nfoo();\n"
	toks := lexPHP([]byte(src))
	var attrLine, fooLine int
	for _, tk := range toks {
		if tk.kind == ptAttrOpen {
			attrLine = tk.line
		}
		if tk.kind == ptIdent && tk.text == "foo" {
			fooLine = tk.line
		}
	}
	if attrLine != 8 || fooLine != 10 {
		t.Fatalf("attribute on line %d, foo on line %d; want 8 and 10", attrLine, fooLine)
	}
}
