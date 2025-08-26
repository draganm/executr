{
  description = "Executr - Go development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.05";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # Go toolchain
            go
            gopls
            go-tools
            golangci-lint
            delve
            
            # Build tools
            gnumake
            pkg-config
            
            # Development tools
            git
            direnv
            
            # Optional: Database tools if needed
            # postgresql
            # sqlite
          ];

          shellHook = ''
            echo "Go development environment"
            echo "Go version: $(go version)"
            echo ""
            echo "Available commands:"
            echo "  go build    - Build the project"
            echo "  go test     - Run tests"
            echo "  go run      - Run the application"
            echo "  gopls       - Go language server"
            echo "  dlv         - Delve debugger"
            echo "  golangci-lint - Linter"
          '';

         
        };

        # Optional: Define the package
        packages.default = pkgs.buildGoModule {
          pname = "executr";
          version = "0.1.0";
          src = ./.;
          
          # Update this after running go mod vendor or getting the vendorHash
          vendorHash = null; # or use "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" 
          
          meta = with pkgs.lib; {
            description = "Executr application";
            license = licenses.mit; # Update with your license
            maintainers = [ ];
          };
        };
      });
}