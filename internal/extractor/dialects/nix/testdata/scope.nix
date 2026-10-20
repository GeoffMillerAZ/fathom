let
  u = import ./lib.nix;
  v = import ./lib.nix { };
  inherit (import ./lib.nix) double;
  inherit ((import ./lib.nix { })) quad;
in
{
  a = u.double 1;
  b = v.quad 2;
  c = double 3;
  d = quad 4;
  e = ./data.json;
  f = ./nested/dir;
}
