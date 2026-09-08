{
  description = "Shared Go primitives for terminal tools";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    systems.url = "github:nix-systems/default";
    provider-spec = {
      url = "github:roshbhatia/provider-spec/v1.0.0";
      flake = false;
    };
  };

  outputs =
    {
      nixpkgs,
      systems,
      provider-spec,
      ...
    }:
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
            version = "0.10.1";
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
            vendorHash = "sha256-XoDgIPJ0E/DVLeHQ13LmYxhvLM5ipaSF0KTK3GQqxxg=";
            nativeCheckInputs = [
              pkgs.bashInteractive
              pkgs.fish
              pkgs.nushell
              pkgs.zsh
            ];
            doCheck = true;
            checkPhase = ''
              runHook preCheck
              export GO_UTILS_REQUIRE_COMPLETION_SHELLS=1
              go vet ./...
              go test -race ./...
              go run ./internal/cmd/animation-schema --check
              go run ./internal/cmd/provider-schema --check
              runHook postCheck
            '';
            installPhase = ''
              touch "$out"
            '';
          };

          # provider/spec is a copy of the pinned provider-spec release. A
          # bumped input without a refreshed copy fails here.
          spec =
            pkgs.runCommandLocal "go-utils-spec-check"
              {
                src = ./provider/spec;
                nativeBuildInputs = [ pkgs.diffutils ];
              }
              ''
                diff "${provider-spec}/VERSION" "$src/VERSION"
                diff "${provider-spec}/schema/provider.schema.json" "$src/provider.schema.json"
                diff -r "${provider-spec}/fixtures/manifest" "$src/fixtures/manifest"
                touch "$out"
              '';
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
              pkgs.bashInteractive
              pkgs.fish
              pkgs.nushell
              pkgs.ripgrep
              pkgs.zsh
            ];
            shellHook = ''
              export GOTOOLCHAIN=local
            '';
          };
        }
      );
    };
}
