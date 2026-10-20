{ config, lib, pkgs, ... }:
with lib;
let
  cfg = config.programs.foo;
  helper = name: value: mkIf cfg.enable { ${name} = value; };
  wrapped = pkgs.callPackage ./pkg.nix { };
  limit = 3;
in
{
  imports = [ ./lib.nix ];
  options.programs.foo.enable = mkEnableOption "foo";
  config = mkIf cfg.enable (helper "x" limit);
  greet = who: "hello ${who}";
  tool = pkgs.callPackage ./pkg.nix { };
  extra.deep = 1;
  extra.other = lib.mkDefault 2;
}
