# Polyglot fixture: Terraform

resource "aws_s3_bucket" "data" {
  bucket = "polyglot-data"
}

module "network" {
  source = "./modules/network"
}
