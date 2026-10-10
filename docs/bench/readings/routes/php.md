# PHP framework routes: Laravel and Symfony (wave c2, item phppy, LB-F336)

**Verdict: Laravel PASS; Symfony reads every route right but composes 12 of 19 (a seam ceiling).**
Before this item fathom read no PHP route at all. It now reads Laravel and Symfony routes from
source, binds each to the controller method that handles it, and applies route groups and imports
across files. On the two Laravel applications it found exactly the routes the running framework
reported, every one bound to its handler:

| Subject | Routes the app serves | fathom found | Handler right | Verdict |
|---|---|---|---|---|
| BookStackApp/BookStack | 343 | all 343, no extras | 343 | PASS |
| koel/koel | 306 | all 306, one extra | 306 | PASS |
| symfony/demo | 19 | 19, of which 12 at the served path | 12 | FAIL (ceiling) |

The comparison is with each framework's own router, booted from the subject's code: Laravel's route
collection (the table `php artisan route:list` prints) and Symfony's compiled route collection (the
table `bin/console debug:router` prints). fathom gives the same answer without Composer, PHP or a
database at ingest time, at no measurable ingest cost.

Keep it. The Symfony reader is right on every route: each of the 19 has its method, its handler and
its path as the controller declares it. The 7 routes of `src/Controller/Admin/` miss the `/{_locale}`
prefix that `config/routes.yaml` imports them under, because the seam's directory mount names only
the files directly inside `src/Controller/`, and Symfony imports a controller directory recursively.
With a recursive directory target (one line of `Compose`, measured on a scratch build, never
committed), symfony/demo reads 19 of 19, bound recall and precision 1.0. That change is the seam's
to make (O693, O698).

## Method

`indexbench route-oracle` (LB-F324, item java) runs the measurement:

1. Each subject is ingested into a scratch store as a product ingest stores it.
2. Its routes are read back through `serviceref.RoutesFromRows`, the composition `BuildFromStore`
   joins on.
3. Those routes are scored against the oracle. Route recall and precision count (method, normalized
   path) pairs; bound recall and precision also require the handler to equal the oracle's.
4. The run uses `-only '**/*.php,**/*.yaml,**/*.yml'`: the reading measures the PHP readers. Without
   it, the legacy `express-regex` reader adds routes from each subject's JavaScript (BookStack 71,
   koel 37), every one a false route; that reader is item node's (O699).

| Subject | Pin | Licence | Oracle (tier A) | Runtime |
|---|---|---|---|---|
| BookStackApp/BookStack | v26.09.1 `31d94dd8` | MIT | Laravel 12.69.3 router | PHP 8.5.11 |
| koel/koel | v9.15.0 `7f9fe1ad` | MIT | Laravel 13.24.0 router | PHP 8.5.11 |
| symfony/demo | v3.1.0 `8f84bf89` | MIT | Symfony 8.1.0 router, env prod | PHP 8.5.11 |

Composer 2.10.3 installed each subject's dependencies inside its checkout (`--no-dev --no-scripts`),
with a scratch `.env` and a generated `APP_KEY` for the Laravel subjects. The converters are
`calibration/oracles/laravel/laravel_routes.php` and `calibration/oracles/symfony/symfony_routes.php`;
the oracle files beside them are what they printed. Each boots the application as its console
does and reads the router itself, so it can tell a route the repository declares from one a package
contributes: Laravel's two package routes on koel and Symfony's two generated routes (the security
logout route, the live-component route) are left out. A route whose action is Laravel's
`ViewController` or `RedirectController` (`Route::view`, `Route::redirect`) or Symfony's
`TemplateController` declared in `config/routes.yaml` is the repository's, and is kept. HEAD beside
GET is not a route of its own; Laravel's `Route::any` is ANY. Measured at fathom `21e55905`; the
numbers are in `php.json`.

## What the readers read

**Laravel** (`readLaravel`): `Route::get/post/put/patch/delete/options/any/match`, `view`, `redirect`,
`fallback`; `resource`, `apiResource`, `resources`, `singleton` with `only`/`except`; `prefix()->group`,
`group(['prefix' => ...])` and `controller()->group` nesting; a group of a route file, a `require`
inside a group, a `RouteServiceProvider` and `bootstrap/app.php`'s `withRouting(api:, apiPrefix:)` as
mounts; a `foreach` over a literal array unrolled (koel's 102 Subsonic routes are registered that
way). The handler is `Controller.method` with `use` aliases resolved, `Controller.__invoke`, or `""`
for a closure.

**Symfony** (`readSymfony`): a class `#[Route]` as the prefix of its methods' `#[Route]`s, concatenated
as Symfony's attribute loader concatenates them; `config/routes.yaml` and `config/routes/**/*.yaml`
read by a yaml-lang reader gated by path, a `path` entry a route and a `resource` entry a mount
under its prefix (`routing.controllers` as the project's `src/Controller/`).

## Composition: with and without mounts

The same binary with the mount rows ignored at read time (a scratch build):

| Subject | Bound recall, mounts applied | Bound recall, mounts ignored |
|---|---|---|
| BookStack | 1.0 | .7668 (the 80 routes `routes/api.php` declares sit at `/pages`, not `/api/pages`) |
| koel | 1.0 | 1.0 (its groups are all in-file) |
| symfony/demo | .6316 | .0526 (only the homepage, which `routes.yaml` declares itself) |

Before this item (the seam alone, `c2-seam` `6c738a22`), every PHP subject read 0 routes.

## Ceilings

- **Symfony directory imports are recursive; the seam's directory target is not.** symfony/demo:
  7 of 19 routes (`src/Controller/Admin/BlogController.php`) read at their local path. Trigger: a
  recursive directory target in `Compose` (O693 proposes the same amendment for Java's deployment
  roots; O698). Measured with it: 19 of 19.
- **A route registered under a runtime condition.** koel's `GET /api/youtube/search/song/{song}` is
  registered only when a YouTube key is configured (`if (YouTube::enabled())`); the reader reads both
  branches of an `if`, the oracle's configuration had none. One extra on koel.
- **Not read, uncounted here:** a route path or prefix from `config()` or another call (the route is
  skipped, or its prefix kept as `{}`); routes a package registers (`vendor/` is never ingested);
  `Route::macro` bodies; Symfony annotations in docblocks (`@Route`, Symfony 5 and older); Symfony
  PHP or XML route files; `when@dev` and `when@test` YAML sections, which production does not serve.

## Ingest cost

Measured on BookStack: 1,837 PHP files (9.8 MB with its YAML), three fresh-store ingests with each
binary (`indexbench route-oracle`, `-only '**/*.php'`).

| Binary | Wall (s) | User (s) |
|---|---|---|
| `c2-seam` (no PHP readers) | 13.06, 9.13, 9.29 | 6.38, 6.59, 6.63 |
| HEAD (Laravel and Symfony readers) | 10.68, 8.56, 11.22 | 6.33, 6.53, 6.79 |

The difference is inside run-to-run noise. Called directly over the same files, the Laravel reader
takes 3.2 ms (a byte prefilter, `Route::`, `withRouting` or `$router->`, passes 3 files to the parser)
and the Symfony reader 0.3 ms.
