package routes

import (
	"reflect"
	"testing"
)

func TestReadSymfony(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		src    string
		routes []string
		mounts []string
	}{
		{
			name: "class prefix and method routes",
			path: "src/Controller/Admin/BlogController.php",
			src: `<?php
namespace App\Controller\Admin;

use Symfony\Component\Routing\Attribute\Route;
use Symfony\Component\Security\Http\Attribute\IsGranted;

#[Route('/admin/post'), IsGranted('ROLE_ADMIN')]
final class BlogController extends AbstractController
{
    #[Route('/', name: 'admin_index', methods: ['GET'])]
    #[Route('/', name: 'admin_post_index', methods: ['GET'])]
    public function index(): Response { return $this->render('x'); }

    #[Route('/new', name: 'admin_post_new', methods: ['GET', 'POST'])]
    public function new(Request $request, #[CurrentUser] User $user): Response {}

    #[Route('/{id:post}/delete', name: 'admin_post_delete', requirements: ['id' => Requirement::POSITIVE_INT], methods: 'POST')]
    public function delete(Post $post): Response {}

    #[Route(path: '/search', methods: ['GET', 'HEAD'])]
    public function search(): Response {}

    #[Route('/any')]
    public function any(): Response {}

    public function helper(): void {}
}
`,
			routes: []string{
				"ANY /admin/post/any BlogController.any",
				"GET /admin/post/ BlogController.index",
				"GET /admin/post/ BlogController.index",
				"GET /admin/post/new BlogController.new",
				"GET /admin/post/search BlogController.search",
				"POST /admin/post/new BlogController.new",
				"POST /admin/post/{id:post}/delete BlogController.delete",
			},
		},
		{
			name: "an invokable controller routed at class level",
			path: "src/Controller/HealthController.php",
			src: `<?php
#[Route('/health', methods: ['GET'])]
class HealthController
{
    public function __invoke(): Response {}
}
`,
			routes: []string{"GET /health HealthController.__invoke"},
		},
		{
			name: "config/routes.yaml routes and imports",
			path: "config/routes.yaml",
			src: `homepage:
    path: /{_locale}
    controller: Symfony\Bundle\FrameworkBundle\Controller\TemplateController::templateAction
    defaults:
        template: default/homepage.html.twig

legacy:
    path: /legacy
    defaults:
        _controller: App\Controller\LegacyController::show
    methods: GET|HEAD

about:
    path: /about
    template: about.html.twig

controllers:
    resource: routing.controllers
    prefix: /{_locale}

api:
    resource: ../src/Api/Controller/
    type: attribute
    prefix:
        en: /api
        nl: /nl/api

kernel:
    resource: ../src/Kernel.php
    type: attribute

bundle:
    resource: '@SomeBundle/config/routes.php'
    prefix: /bundle

when@dev:
    _errors:
        resource: '@FrameworkBundle/Resources/config/routing/errors.php'
    debug:
        path: /debug
        controller: App\Controller\DebugController

when@prod:
    status:
        path: /status
        controller: App\Controller\StatusController
`,
			routes: []string{
				"ANY /about TemplateController.templateAction",
				"ANY /status StatusController.__invoke",
				"ANY /{_locale} TemplateController.templateAction",
				"GET /legacy LegacyController.show",
			},
			mounts: []string{
				"/api -> ../src/Api/Controller/",
				"/nl/api -> ../src/Api/Controller/",
				"/{_locale} -> /src/Controller/",
				" -> ../src/Kernel.php",
			},
		},
		{
			name: "a YAML file elsewhere is not a routing file",
			path: "config/packages/framework.yaml",
			src:  "home:\n  path: /x\n  controller: A::b\n",
		},
		{
			name: "a PHP file with no Route attribute",
			path: "src/Entity/Post.php",
			src:  "<?php\n#[ORM\\Entity]\nclass Post { #[ORM\\Column] private $id; public function route() {} }\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := readSymfony(c.path, []byte(c.src))
			if got := routeLines(res.Routes); !reflect.DeepEqual(got, sortedCopy(c.routes)) && !(len(got) == 0 && len(c.routes) == 0) {
				t.Errorf("routes:\n got  %q\n want %q", got, sortedCopy(c.routes))
			}
			if got := mountLines(res.Mounts); !reflect.DeepEqual(got, sortedCopy(c.mounts)) && !(len(got) == 0 && len(c.mounts) == 0) {
				t.Errorf("mounts:\n got  %q\n want %q", got, sortedCopy(c.mounts))
			}
		})
	}
}
