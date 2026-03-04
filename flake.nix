{
  outputs = inputs: inputs.parts.lib.mkFlake { inherit inputs; } {
    systems = import inputs.systems;

    perSystem = { lib, pkgs, system, self', ... }: {
      _module.args = lib.fix (self: {
        lib = with inputs; builtins // nixpkgs.lib // parts.lib;
        pkgs = import inputs.nixpkgs {
          inherit system;
          overlays = [ inputs.gomod2nix.overlays.default ];
        };
      });

      packages.default =
        let
          version = lib.fileContents ./version.txt;
        in
        pkgs.buildGoApplication {
          meta.mainProgram = "miroir";
          pname = "miroir";
          inherit version;
          src = ./.;
          modules = ./gomod2nix.toml;
          subPackages = [ "cmd/miroir" ];
          ldflags = [ "-X" "main.version=${version}" ];
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
        set -eoux pipefail
        shopt -s globstar

        root="$PWD"
        while [[ ! -f "$root/.git/index" ]]; do
          if [[ "$root" == "/" ]]; then
            exit 1
          fi
          root="$(dirname "$root")"
        done
        pushd "$root" > /dev/null

        ${lib.getExe pkgs.deno} fmt readme.md
        ${lib.getExe pkgs.gomod2nix}
        ${lib.getExe pkgs.go} fix ./...
        ${lib.getExe pkgs.go} fmt ./...
        ${lib.getExe pkgs.go} vet ./...
        ${lib.getExe pkgs.nixpkgs-fmt} .
        ${lib.getExe pkgs.taplo} format **/*.toml
        ${lib.getExe' pkgs.go-tools "staticcheck"} ./...

        popd
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
