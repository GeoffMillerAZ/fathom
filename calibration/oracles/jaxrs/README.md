# jaxrs: JAX-RS route oracle (tier A)

`main.go` converts the OpenAPI document a running JAX-RS application serves into the route oracle
shape (`calibration/oracles/routes/README.md`). The document is the runtime's own model of the
resource classes it deployed, read from the running application.

Conversion rules:

- Every operation is one route: its method upper-cased and its path as the document writes it.
- The handler comes from the operation id. The build generates operation ids with SmallRye's
  `CLASS_METHOD` strategy, so `ArticlesResource_create` becomes `ArticlesResource.create` (split
  at the first `_`). An operation with no id has handler `""`.
- `-strip-prefix` removes a root path that the deployment configures rather than the source
  declares, such as `quarkus.rest.path` or a servlet mapping. The Spring converter leaves out the
  servlet context path for the same reason. A reading that strips a prefix counts every route it
  stripped as a ceiling.

## appiepollo14/quarkus-realworld-api

- Pin: `2bd6b33a555958a1f2035860c41013c8de086076` (2026-10-09).
- Licence: MIT (`LICENSE` at the pin).
- Framework: Quarkus 3.40.1, Quarkus REST.
- Output: `quarkus-realworld-api.json`, 19 routes.

Notes on the build and run:

- **JDK.** Lombok 1.18.48 cannot process annotations on JDK 27, so the subject builds and runs on
  Temurin 21.
- **Dump-only pom changes.** The build runs in a scratch copy whose `pom.xml` gains
  `io.quarkus:quarkus-smallrye-openapi` (the OpenAPI endpoint) and `io.quarkus:quarkus-jdbc-h2` (an
  in-memory database in place of the PostgreSQL dev service, which needs a container). No source file
  changes, and fathom ingests the untouched checkout.
- **Configured root path.** The subject sets `quarkus.rest.path=/api`, so every document path starts
  with `/api`. The converter strips it with `-strip-prefix /api`, and the reading counts all 19
  routes as carrying a configured root that no source reader sees.

```sh
S=<scratch>
git clone https://github.com/appiepollo14/quarkus-realworld-api.git && cd quarkus-realworld-api
git checkout 2bd6b33a555958a1f2035860c41013c8de086076
cp -R . $S/realworld-dump && cd $S/realworld-dump    # add the two dependencies to <dependencies>
export JAVA_HOME=<temurin-21> MAVEN_USER_HOME=$S/m2home
./mvnw -B -Dmaven.repo.local=$S/m2repo -DskipTests -Dquarkus.datasource.db-kind=h2 \
  -Dquarkus.smallrye-openapi.operation-id-strategy=CLASS_METHOD package
java -Xmx512m -Dquarkus.http.port=18735 -Dquarkus.datasource.jdbc.url=jdbc:h2:mem:rw \
  -Dquarkus.datasource.username=sa -Dquarkus.datasource.password=sa -jar target/quarkus-app/quarkus-run.jar &
curl -sf 'http://localhost:18735/q/openapi?format=json' -o openapi.json    # once the port answers
kill %1
cd calibration/oracles/jaxrs && go run . -in openapi.json -strip-prefix /api \
  -subject appiepollo14/quarkus-realworld-api -sha 2bd6b33a555958a1f2035860c41013c8de086076 \
  -licence MIT -tool smallrye-openapi -tool-version "quarkus 3.40.1" > quarkus-realworld-api.json
```

`go test ./...` in this directory pins the conversion.
