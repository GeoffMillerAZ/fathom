{ lib, stdenv, fetchurl }:

stdenv.mkDerivation {
  pname = "foo";
  version = "1.0";
  src = fetchurl { url = "https://example.invalid/foo.tar.gz"; };
  meta.description = lib.mkForce "a package";
}
