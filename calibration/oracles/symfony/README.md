# Symfony route oracle

`symfony_routes.php` is the Symfony oracle for `indexbench route-oracle` (LB-F332, LB-F336; shape in
`calibration/oracles/routes/README.md`). Tier A: it boots the subject's own kernel and reads the
router's compiled `RouteCollection`, the collection `bin/console debug:router --format=json` prints:
every `#[Route]` attribute and every `config/routes` import, prefixes already applied.

| Reads | Tier | Emits |
|---|---|---|
| Symfony's `router` service, `getRouteCollection()`, in the given environment (default `prod`) | A | method, path, handler |

## What is Symfony's

Every path, method set and `_controller`. A controller `Ns\Class::method` is spelled
`Class.method`; a class alone is `Class.__invoke`. No methods is `ANY`; HEAD beside GET is dropped.

## What is not

The scope. A route is the repository's when its controller class lives outside `vendor/`, or when
the repository's own `config/routes.yaml` or `config/routes/*.yaml` declares it by name (a
`TemplateController` route such as symfony/demo's `homepage`). A bundle's imported routes and a
generated route (the security logout route; the live-component route, whose controller is resolved
by a listener) are left out and counted on stderr.

## Run (2026-10-09, PHP 8.5.11, Composer 2.10.3, a scratch checkout)

```sh
git clone --depth 1 --branch v3.1.0 https://github.com/symfony/demo.git symfony-demo
cd symfony-demo && composer install --no-interaction --no-scripts --prefer-dist
php symfony_routes.php . symfony/demo "$(git rev-parse HEAD)" MIT > symfony-demo.json
```

symfony-demo.json: Symfony 8.1.0, env prod, 19 routes; 2 generated routes left out.
