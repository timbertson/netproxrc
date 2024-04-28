final:
let
  pkgs = final.pkgs;
in
{
  context = {
    type = "gomod";
    version = "1";
    root = "netproxrc@development";
  };
  specs = {
    "netproxrc@development" = {
      pname = "netproxrc";
      version = "development";
      depKeys = [
      ];
      src = (final.pathSrc .././.);
      vendorHash = "sha256-iJojbLeaUjr3KmMEOhawT7yZfjGpkXFX2Z3X/hpV9W0=";
    };
  };
}