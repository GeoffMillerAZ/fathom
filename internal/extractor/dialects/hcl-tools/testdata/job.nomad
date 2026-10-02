variable "image" {
  type    = string
  default = "example.com/web:1.0"
}

locals {
  count = 2
}

job "web" {
  datacenters = ["dc1"]

  group "frontend" {
    count = local.count

    service {
      name = "web-http"
      port = "http"
    }

    task "server" {
      driver = "docker"

      config {
        image = "example.com/web:1.0"
      }

      service {
        name = "web-metrics"
      }
    }

    task "sidecar" {
      driver = "docker"

      config {
        image = var.image
      }
    }
  }

  group "cache" {
    task "redis" {
      driver = "docker"

      config {
        image = "redis:7"
      }
    }
  }
}
