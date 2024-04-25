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
      vendorHash = "sha256-LWNn5qp+Z/M9xTtOZ5RDHq1QEFK/Y2XgBi7H5S7Z7XE=";
    };
  };
}