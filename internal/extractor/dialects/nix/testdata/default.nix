{ pkgs ? import <nixpkgs> { } }:
let
  mod = import ./module.nix;
  dir = import ./sub;
in
pkgs.callPackage ./pkg.nix { }
