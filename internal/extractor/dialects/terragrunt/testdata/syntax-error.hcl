locals {
  ok = "fine"
}

inputs = {
  a = 1
  b = (
}

dependency "after" {
  config_path = "../after"
}
