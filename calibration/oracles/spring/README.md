# spring: Spring MVC route oracle (tier A)

`main.go` converts a running Spring Boot application's own route dump, `GET /actuator/mappings`,
into the route oracle shape (`calibration/oracles/routes/README.md`). The dump is the framework's
registry of the handler methods its dispatcher servlet routes to, read from the running application.

Conversion rules:

- A dispatcher-servlet mapping is a route when its handler method is declared under `-package`.
- Each pattern and each method of the mapping is one route; a mapping that names no method is `ANY`.
- Mappings the framework or a library contributes are left out: the error controller, actuator
  endpoints, the actuator links handler and static resources.
- The handler is the class's simple name plus the method name (`OwnerController.showOwner`); a nested
  class's `Outer$Inner` is written `Outer.Inner`.

## spring-projects/spring-petclinic

- Pin: `500158f732419217507c7656904b8e6aa1bcc0d6` (2026-09-29).
- Licence: Apache-2.0 (`LICENSE.txt` at the pin).
- Framework: Spring Boot 4.1.0.
- Runtime: OpenJDK 27 (Homebrew), with the default in-memory H2 database.
- Output: `spring-petclinic.json`.

Build, run and dump (scratch on the external volume; Maven's local repository and the wrapper's
distribution stay there too):

```sh
S=<scratch>
git clone https://github.com/spring-projects/spring-petclinic.git && cd spring-petclinic
git checkout 500158f732419217507c7656904b8e6aa1bcc0d6
export JAVA_HOME=/opt/homebrew/opt/openjdk MAVEN_USER_HOME=$S/m2home
./mvnw -B -Dmaven.repo.local=$S/m2repo -DskipTests -Dcheckstyle.skip=true -Dspring-javaformat.skip=true \
  -Djacoco.skip=true -Dmaven.gitcommitid.skip=true -Dcyclonedx.skip=true package
java -Xmx512m -jar target/spring-petclinic-4.0.0-SNAPSHOT.jar --server.port=18734 &
curl -sf http://localhost:18734/actuator/mappings -o mappings.json    # once the port answers
kill %1
cd calibration/oracles/spring && go run . -in mappings.json -package org.springframework.samples.petclinic \
  -subject spring-projects/spring-petclinic -sha 500158f732419217507c7656904b8e6aa1bcc0d6 \
  -licence Apache-2.0 -tool-version "spring-boot 4.1.0" > spring-petclinic.json
```

The dump held 46 dispatcher-servlet mappings. 17 are the application's. The other 29 were left out:

- 24 actuator operation handlers;
- 1 actuator links handler;
- 2 error-controller mappings;
- 2 static-resource handlers.

`go test ./...` in this directory pins the conversion.
