include {
  path = get_env("PARENT_CONFIG")
}

terraform {
  source = "git::https://example.com/org/modules.git//app?ref=v1.0.0"
}

dependency "net" {
  config_path = dependency.other.outputs.path
}

inputs = {
  name = "example"
}
