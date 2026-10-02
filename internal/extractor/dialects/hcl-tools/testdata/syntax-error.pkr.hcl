variable "ok" {
  default = "fine"
}

source "docker" "broken" {
  image = (
}

build {
  sources = ["source.docker.broken"]
}
