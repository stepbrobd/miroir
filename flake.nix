{
  outputs = inputs: inputs.parts.lib.mkFlake { inherit inputs; } {
    systems = import inputs.systems;

    perSystem = { lib, pkgs, system, self', ... }: {
      _module.args = lib.fix (self: {
        lib = with inputs; builtins // nixpkgs.lib // parts.lib;
        pkgs = import inputs.nixpkgs {
          inherit system;
          overlays = [
            (final: prev: {
              ocamlPackages = prev.ocamlPackages.overrideScope (ocamlFinal: ocamlPrev:
                (with self.lib; genAttrs
                  (attrNames (readDir ./src/pkg))
                  (name: ocamlFinal.callPackage ./src/pkg/${name} { }))
                //
                {
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
                });
            })
          ];
        };
      });

      packages = { inherit (pkgs.ocamlPackages) miroir; };

      devShells.default = pkgs.mkShell {
        inputsFrom = lib.attrValues self'.packages;
        packages = with pkgs.ocamlPackages; [
          dune
          findlib
          ocaml
          ocaml-print-intf
          ocamlformat
          utop
        ];
      };

      formatter = pkgs.writeShellScriptBin "formatter" ''
        ${lib.getExe pkgs.ocamlPackages.dune} fmt
        ${lib.getExe pkgs.nixpkgs-fmt} .
        ${lib.getExe pkgs.taplo} format src/test/**/*.toml
      '';
    };
  };

  inputs.nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
  inputs.parts.url = "github:hercules-ci/flake-parts";
  inputs.parts.inputs.nixpkgs-lib.follows = "nixpkgs";
  inputs.systems.url = "github:nix-systems/default";
}
