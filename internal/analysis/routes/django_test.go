package routes

import (
	"reflect"
	"testing"
)

func TestReadDjango(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		src    string
		routes []string
		mounts []string
	}{
		{
			name: "path, re_path, include chains and view forms",
			path: "mysite/urls.py",
			src: `from django.urls import include, path, re_path
from django.views.decorators.csrf import csrf_exempt
from . import views
from .api import urls as api_urls
from polls.views import PollDetail

extra = [
    path('reports/', views.reports),
    path('reports/<int:year>/', views.year_report, name='year'),
]

urlpatterns = [
    path('', views.home, name='home'),
    path('polls/<int:pk>/', PollDetail.as_view(), name='detail'),
    re_path(r'^archive/(?P<year>[0-9]{4})/(?P<slug>[\w-]+)/?$', views.archive),
    path('hook/', csrf_exempt(views.Hook.as_view())),
    path('blog/', include('blog.urls')),
    path('api/', include(api_urls)),
    path('admin-x/', include((extra, 'x'), namespace='x')),
    path("docs/", include([
        path("intro/", views.intro),
    ])),
]
urlpatterns += [path('about/', views.about)]
urlpatterns.append(path('contact/', views.contact))

if settings.DEBUG:
    urlpatterns += [path('__debug__/', include('debug_toolbar.urls'))]
`,
			routes: []string{
				"ANY / home",
				"ANY /about/ about",
				"ANY /admin-x/reports/ reports",
				"ANY /admin-x/reports/{year}/ year_report",
				"ANY /archive/{year}/{slug}/ archive",
				"ANY /contact/ contact",
				"ANY /docs/intro/ intro",
				"ANY /hook/ Hook",
				"ANY /polls/{pk}/ PollDetail",
			},
			mounts: []string{
				"/api/ -> ./api/urls",
				"/blog/ -> blog.urls",
			},
		},
		{
			name: "a DRF DefaultRouter with a ViewSet defined in the module",
			path: "shop/api/urls.py",
			src: `from rest_framework import routers, viewsets, mixins
from rest_framework.decorators import action
from . import views

class TagViewSet(viewsets.ReadOnlyModelViewSet):
    @action(detail=True, methods=['post'], url_path='pin-it')
    def pin(self, request, pk=None):
        pass

    @action(detail=False)
    def recent(self, request):
        pass

class NoteViewSet(mixins.CreateModelMixin, viewsets.GenericViewSet):
    pass

router = routers.DefaultRouter()
router.APIRootView = views.ShopRootView
router.register(r'tags', TagViewSet)
router.register('notes', NoteViewSet, basename='note')
router.register('orders', views.OrderViewSet)

urlpatterns = [
    path('', include(router.urls)),
]
`,
			routes: []string{
				"ANY / ShopRootView",
				"ANY /{format} ShopRootView",
				"DELETE /orders/{pk}.{format} OrderViewSet.destroy",
				"DELETE /orders/{pk}/ OrderViewSet.destroy",
				"GET /orders.{format} OrderViewSet.list",
				"GET /orders/ OrderViewSet.list",
				"GET /orders/{pk}.{format} OrderViewSet.retrieve",
				"GET /orders/{pk}/ OrderViewSet.retrieve",
				"GET /tags.{format} TagViewSet.list",
				"GET /tags/ TagViewSet.list",
				"GET /tags/recent.{format} TagViewSet.recent",
				"GET /tags/recent/ TagViewSet.recent",
				"GET /tags/{pk}.{format} TagViewSet.retrieve",
				"GET /tags/{pk}/ TagViewSet.retrieve",
				"PATCH /orders/{pk}.{format} OrderViewSet.partial_update",
				"PATCH /orders/{pk}/ OrderViewSet.partial_update",
				"POST /notes.{format} NoteViewSet.create",
				"POST /notes/ NoteViewSet.create",
				"POST /orders.{format} OrderViewSet.create",
				"POST /orders/ OrderViewSet.create",
				"POST /tags/{pk}/pin-it.{format} TagViewSet.pin",
				"POST /tags/{pk}/pin-it/ TagViewSet.pin",
				"PUT /orders/{pk}.{format} OrderViewSet.update",
				"PUT /orders/{pk}/ OrderViewSet.update",
			},
		},
		{
			name: "a SimpleRouter without trailing slash, urlpatterns = router.urls",
			path: "app/routes.py",
			src: `from rest_framework.routers import SimpleRouter
router = SimpleRouter(trailing_slash=False)
router.register('users', UserViewSet)
urlpatterns = router.urls
`,
			routes: []string{
				"DELETE /users/{pk} UserViewSet.destroy",
				"GET /users UserViewSet.list",
				"GET /users/{pk} UserViewSet.retrieve",
				"PATCH /users/{pk} UserViewSet.partial_update",
				"POST /users UserViewSet.create",
				"PUT /users/{pk} UserViewSet.update",
			},
		},
		{
			name: "a manual viewset binding",
			path: "app/urls.py",
			src: `urlpatterns = [
    path('items/', ItemViewSet.as_view({'get': 'list', 'post': 'create'})),
]
`,
			routes: []string{
				"GET /items/ ItemViewSet.list",
				"POST /items/ ItemViewSet.create",
			},
		},
		{
			name: "no urlpatterns, nothing read",
			path: "app/models.py",
			src:  "def path(x):\n    return x\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := readDjango(c.path, []byte(c.src))
			if got := routeLines(res.Routes); !reflect.DeepEqual(got, sortedCopy(c.routes)) && !(len(got) == 0 && len(c.routes) == 0) {
				t.Errorf("routes:\n got  %q\n want %q", got, sortedCopy(c.routes))
			}
			if got := mountLines(res.Mounts); !reflect.DeepEqual(got, sortedCopy(c.mounts)) && !(len(got) == 0 && len(c.mounts) == 0) {
				t.Errorf("mounts:\n got  %q\n want %q", got, sortedCopy(c.mounts))
			}
		})
	}
}

func TestDjangoRegexTemplate(t *testing.T) {
	for rx, want := range map[string]string{
		`^articles/(?P<year>[0-9]{4})/$`:          "articles/{year}/",
		`^api/devices\.(?P<format>[a-z0-9]+)/?$`:  "api/devices.{format}/",
		`^page/(\d+)/$`:                           "page/{}/",
		`^(?P<path>.*)$`:                          "{path}",
		`^files/(?P<p>(?:[a-z]+/)*[a-z]+)\.txt\Z`: "files/{p}.txt",
		`^x/(?:optional/)?y/$`:                    "x/{}y/",
		`^schema/$`:                               "schema/",
	} {
		if got := djangoRegexTemplate(rx); got != want {
			t.Errorf("djangoRegexTemplate(%q) = %q, want %q", rx, got, want)
		}
	}
}

// The Python tokenizer keeps line numbers through triple-quoted strings
// and bracket continuations, and folds an f-string's fields to parts.
func TestLexPythonLinesAndFStrings(t *testing.T) {
	src := "x = '''a\nb'''\nf = (\n  1,\n)\ny = f\"/api/{settings.V}/x\"\nz = 1\n"
	toks := lexPython([]byte(src))
	var zLine int
	var fparts []pyStrPart
	for i, tk := range toks {
		if tk.kind == qtName && tk.text == "z" {
			zLine = tk.line
		}
		if tk.kind == qtString && i > 0 && toks[i-2].text == "y" {
			fparts = tk.parts
		}
	}
	if zLine != 7 {
		t.Fatalf("z on line %d, want 7", zLine)
	}
	want := []pyStrPart{{lit: "/api/"}, {dyn: true, expr: "settings.V"}, {lit: "/x"}}
	if !reflect.DeepEqual(fparts, want) {
		t.Fatalf("f-string parts %+v, want %+v", fparts, want)
	}
}
