# Note: both of these overlays should be unnecessary
# if https://github.com/NixOS/nixpkgs/pull/303307 gets merged
let

	goModOverlay =
		self: super:
		let overrideModDrv = orig: {
			buildPhase = ''
				export GIT_SSL_CAINFO=''${NIX_GIT_SSL_CAINFO:-$GIT_SSL_CAINFO}
				echo "[info] GIT_SSL_CAINFO=$GIT_SSL_CAINFO"
			'' + orig.buildPhase;
		};

		in {
			buildGoModule = args: super.buildGoModule (args // {
				overrideModAttrs = overrideModDrv;
			});
		};

	proxyEnvvarsOverlay =
		self: super: {
			lib = super.lib // {
				fetchers = super.lib.fetchers // {
					proxyImpureEnvVars = super.lib.fetchers.proxyImpureEnvVars ++ [
						"NIX_GIT_SSL_CAINFO"
					];
				};
			};
		};

in
[ proxyEnvvarsOverlay goModOverlay ]
