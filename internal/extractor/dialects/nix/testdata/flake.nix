{
  description = "fixture flake";

  inputs = {
    nixpkgs.url = "github:example/nixpkgs";
    local.url = "./sub";
  };

  outputs = { self, nixpkgs, local }:
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs { inherit system; };
    in
    {
      packages.${system}.default = pkgs.callPackage ./pkg.nix { };
      lib = import ./lib.nix;
    };
}
