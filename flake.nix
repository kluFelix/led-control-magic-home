{
  description = "LED Controller Server";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};

      default = pkgs.buildGoModule rec {
        pname = "led-server";
        version = "0.1.0";
        src = ./.;
        vendorHash = null;
        ldflags = [ "-s" "-w" ];
      };
    in {
      packages.x86_64-linux.default = default;

      apps.x86_64-linux.default = {
        type = "app";
        program = "${default}/bin/led-server";
      };

      devShells.x86_64-linux.default = pkgs.mkShell {
        buildInputs = with pkgs; [
          go
          gopls
        ];

        shellHook = ''
          echo "LED Server dev environment ready!"
          echo "Go version: $(go version)"
          echo "Run 'nix build' to build the binary"
          echo "Binary will be at: $(nix build --print-out-paths)/bin/led-server"
        '';
      };
    };
}
