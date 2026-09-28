import org.gradle.api.Project

val version = "1.0"

fun configure(project: Project) {
    project.apply(plugin = "java")
}

configure(rootProject)
