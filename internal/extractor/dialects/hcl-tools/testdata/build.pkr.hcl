packer {
  required_plugins {
    docker = {
      version = ">= 1.0.0"
      source  = "github.com/hashicorp/docker"
    }
  }
}

variable "base_image" {
  type    = string
  default = "ubuntu:22.04"
}

variable "tag" {
  default = "latest"
}

locals {
  image = "${var.base_image}-${var.tag}"
}

data "docker-image" "base" {
  name = var.base_image
}

source "docker" "example" {
  image  = local.image
  commit = true
}

source "docker" "other" {
  image  = data.docker-image.base.name
  commit = true
}

build {
  name    = "app"
  sources = ["source.docker.example", "source.docker.other"]

  provisioner "shell" {
    inline = ["echo ${var.tag}"]
  }

  provisioner "shell" {
    script = "setup.sh"
  }

  post-processors {
    post-processor "docker-tag" {
      repository = "example.com/app"
      tags       = [var.tag]
    }
  }
}
