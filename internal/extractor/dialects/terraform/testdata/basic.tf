variable "cidr" {
  type = string
}

locals {
  name = "demo-${var.cidr}"
}

data "aws_ami" "ubuntu" {
  most_recent = true
}

resource "aws_vpc" "main" {
  cidr_block = var.cidr
}

resource "aws_subnet" "public" {
  vpc_id     = aws_vpc.main.id
  ami        = data.aws_ami.ubuntu.id
  depends_on = [aws_vpc.main]
}

output "vpc_id" {
  value = aws_vpc.main.id
}

module "vpc" {
  source = "./modules/vpc"
}

module "registry_vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "5.0.0"
}

provider "aws" {
  region = var.cidr
}
