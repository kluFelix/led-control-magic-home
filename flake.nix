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

      nixosModule = { config, lib, pkgs, ... }:
        let
          cfg = config.services.led-server;
        in {
          options.services.led-server = {
            enable = lib.mkEnableOption "LED Controller Server";

            port = lib.mkOption {
              type = lib.types.port;
              default = 5002;
              description = "Port to listen on";
            };

            bindAddress = lib.mkOption {
              type = lib.types.str;
              default = "127.0.0.1";
              description = "Address to bind to";
            };
          };

          config = lib.mkIf cfg.enable {
            systemd.services.led-server = {
              description = "LED Controller Server";
              wantedBy = [ "multi-user.target" ];
              after = [ "network.target" ];
              serviceConfig = {
                Type = "simple";
                ExecStart = "${default}/bin/led-server";
                Restart = "always";
                DynamicUser = true;
                NoNewPrivileges = true;
                ProtectSystem = "strict";
                PrivateTmp = true;
                ProtectKernelTunables = true;
                ProtectKernelModules = true;
                ProtectControlGroups = true;
                MemoryDenyWriteExecute = true;
                RestrictAddressFamilies = [ "AF_INET" "AF_INET6" ];
              };
              environment = {
                PORT = toString cfg.port;
                BIND_ADDRESS = cfg.bindAddress;
              };
            };
          };
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

      nixosModules.led-server = nixosModule;
    };
}
