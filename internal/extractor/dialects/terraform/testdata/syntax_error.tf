resource "aws_vpc" "main" {
  cidr_block = "10.0.0.0/16"
}

resource "aws_subnet" "broken" {
  vpc_id = aws_vpc.main.id
  # missing closing brace below is a real, recoverable syntax error
