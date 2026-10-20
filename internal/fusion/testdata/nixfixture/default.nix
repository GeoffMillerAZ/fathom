{ pkgs ? import <nixpkgs> { } }:
let
  lib = import ./lib;
  util = import ./util.nix;
  inherit (import ./util.nix) triple;
in
{
  app = pkgs.callPackage ./pkgs/app.nix { };
  six = lib.double 3;
  nine = triple 3;
  twelve = util.quadruple 3;
  unbound = quadruple 4;
}
