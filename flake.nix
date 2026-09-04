{
  description = "Shared Go primitives for terminal tools";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    systems.url = "github:nix-systems/default";
  };

  outputs =
    { nixpkgs, systems, ... }:
    let
      supportedSystems = builtins.filter (system: system != "x86_64-darwin") (import systems);
      eachSystem = nixpkgs.lib.genAttrs supportedSystems;
    in
    {
      formatter = eachSystem (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        pkgs.writeShellApplication {
          name = "go-utils-format";
          runtimeInputs = [
            pkgs.fd
            pkgs.nixfmt
          ];
          text = ''
            if [ "$#" -gt 0 ] && [ "''${1#-}" = "$1" ]; then
              exec nixfmt "$@"
            fi
            exec fd --extension nix --type file --exec-batch nixfmt "$@"
          '';
        }
      );

      packages = eachSystem (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.stdenvNoCC.mkDerivation {
            pname = "go-utils";
            version = "0.1.0";
            src = ./.;
            installPhase = ''
              runHook preInstall
              mkdir -p "$out/share/go-utils"
              cp -R . "$out/share/go-utils/source"
              runHook postInstall
            '';
          };
        }
      );

      checks = eachSystem (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          test = pkgs.buildGoModule {
            pname = "go-utils-test";
            version = "0";
            src = ./.;
            vendorHash = "sha256-CSP6mGPQQf8VCiHKPNdYMr/+HhUJjvO3eM6UE04OzwE=";
            nativeCheckInputs = [ pkgs.cue ];
            doCheck = true;
            checkPhase = ''
              runHook preCheck
              go vet ./...
              go test -race ./...
              go run ./internal/cmd/provider-schema --check
              cue vet schema/provider.cue
              runHook postCheck
            '';
            installPhase = ''
              touch "$out"
            '';
          };
        }
      );

      devShells = eachSystem (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            packages = [
              pkgs.go
              pkgs.gopls
              pkgs.gotools
              pkgs.go-tools
              pkgs.cue
              pkgs.ripgrep
            ];
            shellHook = ''
              export GOTOOLCHAIN=local
            '';
          };
        }
      );
    };
}
