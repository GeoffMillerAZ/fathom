# Java framework routes: Spring MVC and JAX-RS (wave c2, LB-F325)

**Verdict: PASS.** fathom now reads the HTTP routes of Spring MVC and JAX-RS applications from source,
and on both subjects it found exactly the routes each running application reported serving, every one
bound to the method that handles it:

| Subject | Routes the app serves | fathom found | Handler right |
|---|---|---|---|
| spring-petclinic | 17 | all 17, with no extras | 17 |
| quarkus-realworld-api | 19 | all 19, with no extras | 19 |

Bound recall and bound precision are 1.0 on each subject, against the .90 bar. Every handler fathom
named is a Java method the same ingest indexed. That match lets a `route:` key, a review or an incident
step from "GET /owners/{}" to `OwnerController.showOwner` with no runtime.

The comparison is with each framework's own answer, not with another indexer: Spring Boot's actuator
mappings and Quarkus's generated OpenAPI document, both read from the running app. fathom gives the
same answer without building or running anything. It needs no Maven run, no JDK at ingest time and no
database, and the readers cost well under a millisecond on these subjects.

Keep it, and use the runtime dump wherever an app can run. The dump stays the authority on what
configuration adds: here, a configured `/api` root that no source reader sees (below).

These are two small, conventional subjects: 36 routes in all. The static census below shows where
the readers stop on larger code. Spring reads cleanly. JAX-RS sub-resources are the gap that matters.

## Method

`indexbench route-oracle` (LB-F324) runs the measurement:

1. Each subject is ingested into a scratch store exactly as a product ingest stores it.
2. Its routes are read back through `serviceref.RoutesFromRows`, the composition
   `serviceref.BuildFromStore` joins on.
3. Those routes are scored against the oracle. Route recall and precision count (method, normalized
   path) pairs. Bound recall and precision also require the handler to equal the oracle's.
4. The run uses `-only 'src/main/**'`. It makes no difference here: neither subject declares a
   controller under `src/test`.

| Subject | Pin | Licence | Oracle (tier A) | Runtime |
|---|---|---|---|---|
| spring-projects/spring-petclinic | `500158f7` | Apache-2.0 | `GET /actuator/mappings`, Spring Boot 4.1.0 | OpenJDK 27 |
| appiepollo14/quarkus-realworld-api | `2bd6b33a` | MIT | `GET /q/openapi`, Quarkus 3.40.1, SmallRye `CLASS_METHOD` operation ids | Temurin 21 |

The converters and oracle files are in `calibration/oracles/spring/` and `calibration/oracles/jaxrs/`.
Their READMEs give the exact build, run and dump commands. The petclinic dump held 46 mappings, and
the converter left out 29 that the framework contributes (actuator, error controller, static
resources). Measured at fathom `18a05774` (wave c2) and re-read at `284b658d` (wave lf2): both
subjects score byte-identically. The numbers are in `java.json`.

## Ceilings

### Counted on the subjects

- **Configured root path, quarkus-realworld-api: 19 of 19 routes.** The subject sets
  `quarkus.rest.path=/api`, so the running app serves `/api/articles` where the source declares
  `/articles`. The converter strips the prefix (`-strip-prefix /api`) and the score counts the
  declared part.

  A client that calls `/api/articles` does not join fathom's `route:GET /articles`. The same holds for
  Spring's `server.servlet.context-path` and a JAX-RS `@ApplicationPath`: no source reader reads them
  today.

  - **Trigger:** a deployment-root reader. It would read `application.properties` or `.yml`, or an
    `@ApplicationPath` class, and report a mount over the module's files. The route seam has no
    target form for "every file of this module" yet, and adding one is a contract change (O693).

### Static census (counted by the readers' own scanner, not scored)

| Corpus | Read as routes | Not read |
|---|---|---|
| macrozheng/mall `dcaa93b3` (Apache-2.0) | 246 of 246 Spring mappings: none names an undeclared constant, none sits on an interface, none on a Feign client | none |
| keycloak/keycloak `eb6d2dde`, `services/.../resources` (Apache-2.0) | 15 HTTP methods on `@Path` root resources | 12 sub-resource locators, and 340 HTTP methods on sub-resource classes with no `@Path` of their own, which locators reach |

The keycloak row is the JAX-RS ceiling that matters: on that code fathom reads 15 of 355 HTTP
methods, about 4%.

- **Trigger:** a locator reader. A locator is a method with `@Path` and no HTTP method. It would
  report a mount whose target is the dotted name of its return type, resolved through the file's
  imports and package, which the seam's dotted-module form already matches. The sub-resource class's
  methods would then be routes only under that mount.

  The guard: an unmounted file keeps its local paths under Compose's rule, so a sub-resource class
  whose locator does not resolve must stay unread, not be read at `/`.

### Out of scope, uncounted here

- Spring mappings that use a constant declared in another file (the route is dropped, never guessed).
- Mappings declared on an interface that a controller implements: fathom names the interface method,
  and the runtime names the implementing class.
- Inherited JAX-RS annotations.
- Composed or meta-annotations, such as a custom `@MyGetMapping`.
- Spring WebFlux functional routes (`RouterFunctions.route`).
- Kotlin controllers.
- API-first controllers whose interfaces are generated at build time. Their mappings are not in the
  repository at all.

## Ingest cost

Measured on spring-petclinic: 50 Java files, 141 KB, three fresh-store ingests with each binary.

| Binary | Wall (s) | User (s) |
|---|---|---|
| `c2-seam` (no Java readers) | 3.53, 1.67, 1.60 | 1.74, 1.51, 1.45 |
| HEAD (Spring and JAX-RS readers) | 2.82, 1.58, 1.92 | 1.48, 1.45, 1.64 |

The difference is inside run-to-run noise. Called directly, the two readers take:

| Corpus | Files | Size | Time |
|---|---|---|---|
| spring-petclinic | 50 | 141 KB | 0.22 ms |
| quarkus-realworld-api | 161 | 192 KB | 0.28 ms |
| guava (no web annotations; the byte prefilter skips nearly every file) | 3,227 | 29 MB | 11.9 ms |

A full scan of guava that bypasses the prefilter finishes in 0.4 s with no panic, and
square/retrofit's client-side `@GET`/`@Path` annotations yield no route.
