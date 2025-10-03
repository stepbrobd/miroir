{
  outputs = inputs: inputs.parts.lib.mkFlake { inherit inputs; } {
    systems = import inputs.systems;

    perSystem = { lib, pkgs, system, self', ... }: {
      _module.args = {
        lib = with inputs; builtins // nixpkgs.lib // parts.lib;
        pkgs = import inputs.nixpkgs {
          inherit system;
          overlays = [
            (final: prev: {
              ocamlPackages = prev.ocamlPackages.overrideScope (_: prev: {
                # -_-
                dune = prev.dune_3;
                # https://github.com/nixos/nixpkgs/pull/356634
                mirage-crypto-rng = prev.mirage-crypto-rng.overrideAttrs {
                  doCheck = !(with final.stdenv; isDarwin && isAarch64);
                };
                # https://github.com/nixos/nixpkgs/pull/433017
                ppxlib = prev.ppxlib.override {
                  version = "0.33.0";
                };
              });
            })
          ];
        };
      };

      formatter = pkgs.writeShellScriptBin "formatter" ''
        ${pkgs.dune_3}/bin/dune fmt
        ${pkgs.nixpkgs-fmt}/bin/nixpkgs-fmt .
      '';

      devShells.default = pkgs.mkShell {
        inputsFrom = [ self'.packages.default ];
        packages = with pkgs.ocamlPackages; [
          dune
          findlib
          ocaml
          ocamlformat
          utop
        ];
      };

      packages.default = pkgs.ocamlPackages.buildDunePackage (finalAttrs: {
        pname = "miroir";
        meta.mainProgram = finalAttrs.pname;
        version = "0-unstable-git-${with inputs; self.shortRev or self.dirtyShortRev}";

        env.DUNE_CACHE = "disabled";

        src = with lib.fileset; toSource {
          root = ./.;
          fileset = unions [
            ./bin
            ./dune-project
            ./miroir.opam
          ];
        };

        propagatedBuildInputs = with pkgs.ocamlPackages; [
          cmdliner
          otoml
          ppx_deriving
          ppx_deriving_cmdliner
          ppxlib
        ];
      });
    };
  };

  inputs.nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
  inputs.parts.url = "github:hercules-ci/flake-parts";
  inputs.parts.inputs.nixpkgs-lib.follows = "nixpkgs";
  inputs.systems.url = "github:nix-systems/default";
}
