{ lib }:
let
  before = x: x + 1;
  broken = (1 + ;
  after = y: y;
in
{
  kept = before 1;
  last = after 2;
}
