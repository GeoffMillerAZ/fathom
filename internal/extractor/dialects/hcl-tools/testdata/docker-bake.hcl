variable "REGISTRY" {
  default = "example.com"
}

variable "TAG" {
  default = "dev"
}

function "tagged" {
  params = [name]
  result = ["${REGISTRY}/${name}:${TAG}"]
}

group "default" {
  targets = ["app", "worker"]
}

group "all" {
  targets = ["default", "docs"]
}

target "base" {
  context    = "."
  dockerfile = "Dockerfile"
}

target "app" {
  inherits = ["base"]
  tags     = tagged("app")
  contexts = {
    shared = "target:base"
  }
}

target "worker" {
  inherits = ["base"]
  tags     = ["${REGISTRY}/worker:${TAG}"]
}

target "docs" {
  inherits   = ["app"]
  dockerfile = "docs/Dockerfile"
  args = {
    BUILDER = target.app.tags
  }
}
