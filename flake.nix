{
  description = "Go development environment for LED controller server";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};
    in {
      devShells.${system}.default = pkgs.mkShell {
        buildInputs = with pkgs; [
          go
          gopls
        ];

        shellHook = ''
          echo "Go development environment ready!"
          echo "Go version: $(go version)"
        '';
      };
    };
}