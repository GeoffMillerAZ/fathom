rec {
  double = x: x * 2;
  quad = x: double (double x);
  names = [ "a" "b" ];
  nested = { inner = x: x; };
  inherit (builtins) map;
}
