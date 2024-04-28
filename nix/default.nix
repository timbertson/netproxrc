{ pkgs ? import <nixpkgs> {} }:
with pkgs;
let
  sources = pkgs.callPackage ./sources.nix {};
  fetlock = callPackage sources.fetlock {};
  selection = fetlock.gomod.load ./lock.nix {
    pkgOverrides = self: [
      (self.overrideAttrs {
        netproxrc = _: {
          doCheck = false; # tests require network access
        };
      })
    ];

  };
in selection
