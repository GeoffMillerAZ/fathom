include "root" {
  path = find_in_parent_folders("root.hcl")
}

include "common" {
  path   = "${dirname(find_in_parent_folders("root.hcl"))}/_common/app.hcl"
  expose = true
}

locals {
  env  = "qa"
  tier = local.env
}

dependency "vpc" {
  config_path = "../vpc"

  mock_outputs = {
    vpc_id = "vpc-00000000"
  }
}

dependency "db" {
  config_path = "${get_terragrunt_dir()}/../${local.tier}-db"
}

dependencies {
  paths = ["../vpc", "../queue"]
}

terraform {
  source = "../../modules//app"
}

inputs = merge(
  include.common.locals,
  {
    vpc_id  = dependency.vpc.outputs.vpc_id
    db_host = dependency.db.outputs.host
    env     = local.env
    size    = 3
  }
)
