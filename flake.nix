{
  outputs = inputs: inputs.parts.lib.mkFlake { inherit inputs; } {
    systems = import inputs.systems;

    perSystem = { lib, pkgs, system, self', ... }: {
      _module.args = lib.fix (self: {
        lib = with inputs; builtins // nixpkgs.lib // parts.lib;
        pkgs = import inputs.nixpkgs {
          inherit system;
          overlays = [
            inputs.gomod2nix.overlays.default
          ];
        };
      });

      packages.default = pkgs.buildGoApplication {
        pname = "miroir";
        version = "dev";
        src = ./.;
        modules = ./gomod2nix.toml;
        subPackages = [ "cmd/miroir" ];
      };

      devShells.default = pkgs.mkShell {
        inputsFrom = lib.attrValues self'.packages;
        packages = with pkgs; [
          go
          go-tools
          gomod2nix
          gopls
        ];
      };

      formatter = pkgs.writeShellScriptBin "formatter" ''
        ${lib.getExe pkgs.nixpkgs-fmt} .
      '';
    };
  };

  inputs.nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
  inputs.systems.url = "github:nix-systems/default";
  inputs.parts.url = "github:hercules-ci/flake-parts";
  inputs.parts.inputs.nixpkgs-lib.follows = "nixpkgs";
  inputs.utils.url = "github:numtide/flake-utils";
  inputs.utils.inputs.systems.follows = "systems";
  inputs.gomod2nix.url = "github:nix-community/gomod2nix";
  inputs.gomod2nix.inputs.nixpkgs.follows = "nixpkgs";
  inputs.gomod2nix.inputs.flake-utils.follows = "utils";
}
