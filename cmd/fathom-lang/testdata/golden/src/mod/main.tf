resource "aws_s3_bucket" "logs" {
  bucket   = var.name
  acl      = "private"
  password = var.password
  region   = "${var.name}-east"

  versioning {
    enabled = true
  }
}
