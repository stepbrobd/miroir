{
  outputs = inputs: inputs.parts.lib.mkFlake { inherit inputs; } {
    systems = import inputs.systems;

    perSystem = { lib, pkgs, system, self', ... }: {
      _module.args = lib.fix (self: {
        lib = with inputs; builtins // nixpkgs.lib // parts.lib;
        pkgs = import inputs.nixpkgs {
          inherit system;
          config.allowDeprecatedx86_64Darwin = true;
          overlays = [ inputs.gomod2nix.overlays.default ];
        };
      });

      packages.default = pkgs.buildGoApplication (lib.fix (finalAttrs: {
        pname = "miroir";
        meta.mainProgram = finalAttrs.pname;
        version = lib.fileContents ./version.txt;
        src = with lib.fileset; toSource {
          root = ./.;
          fileset = unions [
            ./cmd
            ./config
            ./display
            ./forge
            ./gitops
            ./index
            ./miroir
            ./workspace
            ./go.mod
            ./go.sum
          ];
        };
        modules = ./gomod2nix.toml;
        subPackages = [ "cmd/miroir" ];
        ldflags = [ "-X" "main.version=${finalAttrs.version}" ];
        # tests run in the flake checks, never inside a package build
        doCheck = false;
        nativeBuildInputs = [ pkgs.installShellFiles ];
        postInstall = ''
          for shell in bash zsh fish; do
            installShellCompletion --cmd ${finalAttrs.pname} --''${shell} <("$out/bin/${finalAttrs.pname}" completion "$shell")
          done
        '';
      }));

      checks =
        let
          # a go tool run inside the package build, so the vendored modules are on hand
          go = name: tools: command: self'.packages.default.overrideAttrs (old: {
            pname = "miroir-${name}";
            nativeBuildInputs = old.nativeBuildInputs ++ tools;
            buildPhase = ''
              export HOME="$TMPDIR"
              ${command}
            '';
            installPhase = ''touch "$out"'';
            # tests bind loopback listeners, the darwin sandbox forbids that by default
            __darwinAllowLocalNetworking = true;
          });

          # a tool that only reads the tree
          over = name: tools: command: pkgs.runCommand "miroir-${name}" { nativeBuildInputs = tools; } ''
            export HOME="$TMPDIR"
            cd ${inputs.self}
            ${command}
            touch "$out"
          '';
        in
        {
          deno = over "deno" [ pkgs.deno ] "deno fmt --check readme.md";
          gofmt = go "gofmt" [ ] ''test -z "$(gofmt -l $(go list -f '{{.Dir}}' ./...))"'';
          nixpkgs-fmt = over "nixpkgs-fmt" [ pkgs.nixpkgs-fmt ] "nixpkgs-fmt --check .";
          staticcheck = go "staticcheck" [ pkgs.go-tools ] "staticcheck ./...";
          taplo = over "taplo" [ pkgs.taplo ] "taplo fmt --check";
          test = go "test" [ pkgs.git ] "go test -race ./...";
          vet = go "vet" [ ] "go vet ./...";
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
        ${lib.getExe pkgs.go} fix ./...
        ${lib.getExe pkgs.go} fmt ./...
        ${lib.getExe pkgs.go} test -race ./...
        ${lib.getExe pkgs.go} vet ./...
        ${lib.getExe pkgs.nixpkgs-fmt} .
        ${lib.getExe pkgs.taplo} format **/*.toml
        ${lib.getExe' pkgs.go-tools "staticcheck"} ./...
        ${lib.getExe' pkgs.gomod2nix "gomod2nix"}

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
