### Fixed

- Corrected language-board claims for `Containerfile`, `.tf.json`, `.sln`, `build.gradle.kts` and `settings.gradle*`: they were listed as supported but had no route for the named format. Generic JSON routing for `.tf.json` and Kotlin routing for `build.gradle.kts` do not provide Terraform or Gradle DSL support. The systemd entry now lists finite, unsupported unit patterns.

- Removed the unsupported bare `Taskfile` claim. The go-task entry now names the routed `Taskfile.yml` explicitly.
