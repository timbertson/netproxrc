# netproxrc

A HTTP proxy injecting credentials from a `.netrc` file.

# Why?

It's hard to give `nix` access to secrets, since it runs builds in a clean environment and intentionally doesn't let builds access files on your system. But secrets are often needed to fetch artifacts from private git repos or other non-public sources.

But you _can_ tell nix to use a proxy. This way, the proxy (outside nix) injects secrets from a .netrc file and nix builds get to use that without needing to read the .netrc file itself.

# Is this secure?

It's for local use only, so it depends on your threat model:

 - any user with access to the localhost proxy port could use the proxy to impersonate you to any site in your .netrc
 - if a domain has an endpoint which reflects back the `Authentication` header from the request into the response body, a malicious derivation could read your credentials for that domain

These seem inherent to the problem, so I consider it as secure as possible.

(perhaps a unix domain socket could knock off the first item, but that sounds hard)

# Nix multi-user mode

I built this proxy in order to have nix builds use authenticated sources (private git repos or company-internal package repositories).

It works out of the box for single-user installations, but in multi-user installations via `nix-daemon` it's trickier. To make this work, it adds `impure-env` to the `$NIX_CONFIG` environment variable. For this to work, you'll need two things:

 - your nix-daemon needs to use nix v2.19.0 or greater
 - you need to add `experimental-features = configurable-impure-env` to your `/etc/nix/nix.conf` (and restart the daemon after adding this)

To update the nix version used in the daemon, you can do this (as root):

```
root# nix-channel --update
root# nix-env --upgrade
root# nix --version
nix (Nix) 2.21.2
```

# The gnarly details of nix proxying

Software is pretty standard in looking for proxy config in the environment's `http_proxy`, `https_proxy`, `all_proxy`, etc.

Which was enough in the days of http, but a https proxy like this one also needs to intercept all your trafic to do its job. That's typically something bad people do, so it's prevented by default. The way we can allow it is to create a set of root certificates, with all the ones found on your system as well as the proxy's self-signed certificate.

Different software all supports the same format for this file, which is nice. But they differ in which environment variable they use to specify that file. Git uses `GIT_SSL_CAINFO`, openssl uses `SSL_CERT_FILE`, curl uses `CURL_CA_BUNDLE`, and there's likely more.

Nix adds another layer to understand. Nix's `cacert` package [exports `NIX_SSL_CERT_FILE`](https://github.com/NixOS/nixpkgs/blob/3a0030bfafd5c961cc148944450eefcbd1d3eeb2/pkgs/data/misc/cacert/setup-hook.sh#L1) as the canonical environment variable where nix-wrapped tools ought to read from.

Nix-packaged tools are supposed to read from this envvar instead of the default paths they would be configured with on a typical linux distro (typically somewhere in `/etc/`). Here's the [relevant patch for openssl](https://github.com/NixOS/nixpkgs/blob/f3565a2c088883636f198550eac349ed82c6a2b3/pkgs/development/libraries/openssl/3.0/nix-ssl-cert-file.patch)

But `cacert` _also_ exports `SSL_CERT_FILE` and `SYSTEM_CERTIFICATE_PATH`, for compatibility with tools that haven't been (or can't be?) wrapped in this way.

So we know how tools are configured, and we know that nix-packaged tools are supposed to use a nix-specific default which is provided by the `cacerts` package, instead of using the host system's certificates.

Finally we come to the third layer: fetchers. Fetchers are responsible for most filed-output derivations. These derivations are allowed to do impure things like talk to the internet, as long as they produce output matching the output hash that's specified.

These fetchers have an _additional_ convention. For example, the `fetchgit` implementation [reads `NIX_GIT_SSL_CAINFO` _in preference_ to `GIT_SSL_CAINFO`](https://github.com/NixOS/nixpkgs/blob/f3565a2c088883636f198550eac349ed82c6a2b3/pkgs/build-support/fetchgit/nix-prefetch-git#L22).

Why doesn't it use `NIX_SSL_CERT_FILE`? It doesn't need to, because the derivation [sets `GIT_SSL_CAINFO` to the equivalent path](https://github.com/NixOS/nixpkgs/blob/f3566a2c088883636f198550eac349ed82c6a2b3/pkgs/build-support/fetchgit/default.nix#L96). Maybe patching git to use a different environment variable is harder to maintain, or breaks other use cases.

So all in all, to get `fetchgit` to use custom certificates just by setting an environment variable:

 - Normally on a non-nix installation, you'd set `GIT_SSL_CAINFO`.
 - But that won't have any effect because any `fetchgit` derivation sets that to `"${cacert}/etc/ssl/certs/ca-bundle.crt"`
 - So you need to instead set `NIX_GIT_SSL_CAINFO`, which the implementation of fetchgit looks at in preference to `GIT_SSL_CAINFO`

In order for that variable to actually get copied into the derivation's otherwise clean environment, you need to add `NIX_GIT_SSL_CAINFO` to the list of `impureEnvVars` in the derivation. The fetchgit derivation does this already.

In order to support git go modules, I ended up making [a similar modification](https://github.com/NixOS/nixpkgs/pull/266643) to go's module fetching code.
