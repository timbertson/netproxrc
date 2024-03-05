{ pkgs ? import <nixpkgs> {} }:
with pkgs;
let
  fetlock = callPackage (builtins.fetchTarball "https://github.com/timbertson/fetlock/archive/master.tar.gz") {};
  selection = fetlock.gomod.load ./lock.nix {};
in selection
