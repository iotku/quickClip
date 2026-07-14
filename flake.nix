{
  description = "Go development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs {
          inherit system;
        };
      in
      {
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gopls

            # Required by CGO packages
            pkg-config

            # Native libraries
            wayland
            libxkbcommon
            alsa-lib
            vulkan-headers

            # Often needed by Gio on Linux
            libGL
            libX11
            libXcursor
            libXfixes
            libXi
            libXrandr
            libXinerama
            libXext
            libXrender
            libXdamage
            libXxf86vm
            libxcb
          ];

          shellHook = ''
            echo "Go development environment loaded."
          '';
        };
      }
    );
}
