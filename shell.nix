{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  nativeBuildInputs = with pkgs; [
    go
    gopls
    golangci-lint
    python3
    mkdocs
    nodejs
  ];
}
