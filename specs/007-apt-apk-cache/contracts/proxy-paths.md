# Proxy Path Contracts: APT/APK

## APT

| Downstream Request | Upstream Target | Caching | Notes |
|--------------------|-----------------|---------|-------|
| `/dists/<suite>/<component>/binary-<arch>/Packages[.gz/.xz]` | same path on Upstream | RequireRevalidate=true | Send If-None-Match/If-Modified-Since if cached |
| `/dists/<suite>/Release` | same path | RequireRevalidate=true | Preserve content; may have accompanying `.gpg` |
| `/dists/<suite>/InRelease` | same path | RequireRevalidate=true | Signed inline; no body changes |
| `/dists/<suite>/Release.gpg` | same path | RequireRevalidate=true | Signature only; no rewrite |
| `/pool/<vendor>/<name>.deb` | same path | AllowCache/AllowStore=true, RequireRevalidate=false | Treat as immutable |
| `/dists/<suite>/by-hash/<algo>/<hash>` | same path | AllowCache/AllowStore=true, RequireRevalidate=false | Path encodes hash, no extra validation |

## Alpine APK

| Downstream Request | Upstream Target | Caching | Notes |
|--------------------|-----------------|---------|-------|
| `/v3.<branch>/main/<arch>/APKINDEX.tar.gz` (and variants) | same path | RequireRevalidate=true | Preserve signature/headers |
| `/v3.<branch>/<repo>/<arch>/APKINDEX.tar.gz` | same path | RequireRevalidate=true | Same handling as above |
| `/v3.<branch>/<repo>/<arch>/packages/<file>.apk` | same path | AllowCache/AllowStore=true, RequireRevalidate=false | Immutable package |
| APKINDEX signature files | same path | RequireRevalidate=true | No rewrite |

## Behaviors
- No body/URL rewrite; proxy only normalizes cache policy per path shape.
- Preserve status codes, headers, and signature files exactly as upstream.
- Conditional requests (If-None-Match/If-Modified-Since) applied to all index/signature paths when cached.
