locals {
  region = "us-east-1"
  name   = "example-${local.region}"
}

generate "provider" {
  path      = "provider.tf"
  if_exists = "overwrite_terragrunt"
  contents  = <<EOF
provider "aws" {
  region = "${local.region}"
}
EOF
}

remote_state {
  backend = "s3"
  config = {
    bucket = local.name
    key    = "${path_relative_to_include()}/tf.tfstate"
  }
}
