variable "name" {
  default = "my-logs"
}

variable "password" {
  default   = "golden-sensitive-sentinel"
  sensitive = true
}
