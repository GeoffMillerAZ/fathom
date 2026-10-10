# Laravel route oracle

`laravel_routes.php` is the Laravel oracle for `indexbench route-oracle` (LB-F331, LB-F336; shape in
`calibration/oracles/routes/README.md`). Tier A: it boots the subject's own application the way
`php artisan route:list` does (the console kernel's bootstrap loads every route file through the
app's `RouteServiceProvider` or `bootstrap/app.php` `withRouting`) and prints the router's route
collection. It shares no code with fathom's reader.

| Reads | Tier | Emits |
|---|---|---|
| Laravel's `Router::getRoutes()` after the console kernel's bootstrap | A | method, path, handler |

## What is Laravel's

Every method, URI and action. A controller action is the router's `Class@method` (an invokable
controller is `Class@__invoke`), spelled `ShortClass.method` as fathom's PHP dialect names a method;
a closure is `""`.

## What is not

The scope and the spelling. A route whose action lives under `vendor/` (a package's) is left out and
counted on stderr; a route whose action is `Illuminate\Routing\ViewController` or `RedirectController`
(`Route::view`, `Route::redirect`) is the repository's, since only the repository's route files
declare one. HEAD beside GET is dropped (the router adds it to every GET); the seven verbs of
`Route::any` are `ANY`.

## Run (2026-10-09, PHP 8.5.11, Composer 2.10.3, scratch checkouts)

```sh
git clone --depth 1 --branch v26.09.1 https://github.com/BookStackApp/BookStack.git bookstack
cd bookstack && composer install --no-interaction --no-scripts --no-dev --prefer-dist
cp .env.example .env   # then set APP_KEY=base64:<32 random bytes, base64>
php -d display_errors=stderr laravel_routes.php . BookStackApp/BookStack "$(git rev-parse HEAD)" MIT > bookstack.json
```

koel v9.15.0 the same way (`koel/koel`, MIT). bookstack.json: Laravel 12.69.3, 343 routes, none a
package's. koel.json: Laravel 13.24.0, 306 routes, 2 a package's left out.
