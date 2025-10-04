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
              ocamlPackages = prev.ocamlPackages.overrideScope (ocamlFinal: ocamlPrev: {
                # -_-
                dune = ocamlPrev.dune_3;
                # https://github.com/nixos/nixpkgs/pull/356634
                mirage-crypto-rng = ocamlPrev.mirage-crypto-rng.overrideAttrs {
                  doCheck = !(with final.stdenv; isDarwin && isAarch64);
                };
                # https://github.com/nixos/nixpkgs/pull/433017
                ppxlib = ocamlPrev.ppxlib.override {
                  version = "0.33.0";
                };
                # https://github.com/andreypopp/ppx_deriving/tree/0.4/toml
                ppx_deriving_toml = ocamlFinal.callPackage ./pkg/ppx_deriving_toml { };
              });
            })
          ];
        };
      };

      formatter = pkgs.writeShellScriptBin "formatter" ''
        ${lib.getExe pkgs.ocamlPackages.dune} fmt
        ${lib.getExe pkgs.nixpkgs-fmt} .
        ${lib.getExe pkgs.taplo} format test/**/*.toml
      '';

      devShells.default = pkgs.mkShell {
        inputsFrom = [ self'.packages.default ];
        packages = with pkgs.ocamlPackages; [
          dune
          findlib
          ocaml
          ocaml-print-intf
          ocamlformat
          utop
        ];
      };

      packages.default = pkgs.ocamlPackages.buildDunePackage (finalAttrs: {
        pname = "miroir";
        meta.mainProgram = finalAttrs.pname;
        version = with lib; pipe ./dune-project [
          readFile
          (match ".*\\(version ([^\n]+)\\).*")
          head
        ];

        src = with lib.fileset; toSource {
          root = ./.;
          fileset = unions [
            ./bin
            ./lib
            ./test
            ./dune-project
            ./miroir.opam
          ];
        };

        env.DUNE_CACHE = "disabled";

        buildInputs = with pkgs.ocamlPackages; [
          cmdliner
          dune-build-info
          otoml
          ppx_deriving
          ppx_deriving_cmdliner
          ppx_deriving_toml
        ];

        doCheck = true;
        checkInputs = with pkgs.ocamlPackages; [
          alcotest
          containers
        ];
      });
    };
  };

  inputs.nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
  inputs.parts.url = "github:hercules-ci/flake-parts";
  inputs.parts.inputs.nixpkgs-lib.follows = "nixpkgs";
  inputs.systems.url = "github:nix-systems/default";
}
