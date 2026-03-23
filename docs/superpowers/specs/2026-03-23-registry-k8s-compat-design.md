# registry.k8s.io coredns fallback design

## Goal

Add a narrow compatibility fallback for `registry.k8s.io` so requests like
`k8s.hub.ipao.vip/coredns:v1.13.1` can retry as
`k8s.hub.ipao.vip/coredns/coredns:v1.13.1` when the original manifest lookup is
not found upstream.

## Scope

This compatibility applies only when all of the following are true:

- hub module is `docker`
- upstream host is exactly `registry.k8s.io`
- requested repository name is a single segment such as `coredns`
- request targets `/manifests/<ref>`
- the first upstream request clearly fails as not found

Out of scope:

- changing behavior for Docker Hub
- changing behavior for other registries
- eagerly rewriting every single-segment repo under `registry.k8s.io`
- retrying more than one alternate repository form
- applying the initial compatibility behavior to blobs, tags, or referrers

## Recommended approach

Keep the current request path unchanged on the first attempt. After the normal
auth flow completes for that path, if the final first-attempt response from
`registry.k8s.io` is `404` for a single-segment manifest repository, perform
one internal retry against the derived path `/v2/<repo>/<repo>/...` and return
the retry result when it succeeds.

This keeps the existing behavior stable for paths that already work and limits
the compatibility behavior to the known `coredns -> coredns/coredns` style case.

## Alternatives considered

### 1. Always rewrite single-segment repos to `<repo>/<repo>`

Rejected because it would change successful existing paths and could break valid
single-segment repositories on `registry.k8s.io`.

### 2. Probe both paths before deciding

Rejected because it doubles upstream traffic and adds more complexity than the
known compatibility problem needs.

## Request flow

1. Receive docker request and run the existing normalization logic.
2. Send the first upstream request using the original normalized path.
3. Complete the existing auth retry behavior for that path first.
4. If the final response is successful, continue as today.
5. If the final response is `404`, check whether the request is eligible for
   the `registry.k8s.io` fallback.
6. Close the first response body before retrying.
7. If eligible, derive a second path by transforming `/v2/<repo>/...` into
   `/v2/<repo>/<repo>/...`.
8. Retry exactly once against the derived path, reusing the same method and the
   same auth behavior used by the normal upstream flow.
9. If the retry succeeds, use that response and mark the cache entry with the
   effective upstream path that produced the content.
10. If the retry also returns an upstream response, pass that second response
   through unchanged.
11. If the retry fails with a transport error, return the existing proxy error
   response path and do not continue retrying.

## Eligibility rules

The fallback should only trigger when all checks pass:

- upstream host matches `registry.k8s.io`
- request path parses as `/v2/<repo>/manifests/<ref>`
- repository name contains exactly one segment before `manifests`
- repository is not already `<repo>/<repo>`
- final response after normal auth handling is HTTP `404`

For the initial implementation, the trigger is status-based and limited to HTTP
`404`.

## Component changes

### `internal/hubmodule/docker/hooks.go`

- add helper logic to identify `registry.k8s.io`
- add helper logic to derive the duplicate-segment manifest path for eligible
  requests

### `internal/proxy/handler.go`

- add a targeted retry path after the first upstream response is received and
  before the response is finalized
- keep the retry count fixed at one alternate attempt
- complete the normal auth retry flow before evaluating fallback eligibility
- persist the effective upstream path when fallback succeeds so future cache
  revalidation can target the path that actually produced the cached content

The retry should reuse the same auth, request method, response handling, and
streaming behavior as the normal upstream path.

### Cache metadata

- extend cached metadata for fallback-derived entries with an optional effective
  upstream path field
- when a cached entry was populated through fallback, revalidate it against that
  effective upstream path rather than the original client-facing path
- do not apply the fallback derivation again during revalidation if an effective
  upstream path is already recorded

## Caching behavior

- first successful response remains cacheable under the locator path that served
  the client request
- if fallback succeeds, cache the successful fallback response under the client
  request path so the next identical client request can hit cache directly
- also persist the effective upstream path used to fill that cache entry
- do not cache the failed first attempt separately

This preserves the client-visible repository path while allowing the proxy to
internally source content from the alternate upstream location.

## Error handling

- if fallback path derivation is not possible, return the original response
- if the fallback request errors at the transport layer, log the retry attempt
  and return the existing proxy error response path
- never loop between variants

## Logging

Add one targeted structured log event for visibility when the fallback is used,
including:

- hub name
- domain
- upstream host
- original path
- fallback path
- original status
- request method

This makes it easy to confirm that `registry.k8s.io` compatibility is active
without changing normal logging volume for unrelated registries.

## Tests

### Unit tests

Add docker hook tests for:

- recognizing `registry.k8s.io`
- deriving `/v2/coredns/manifests/v1.13.1` to
  `/v2/coredns/coredns/manifests/v1.13.1`
- refusing fallback for already multi-segment repositories
- refusing fallback for non-`registry.k8s.io` hosts
- refusing fallback for non-manifest paths

### Integration tests

Add proxy tests for:

- original path returns `404`, fallback path returns `200` -> final client
  response is `200`
- original path returns `200` -> fallback is not attempted
- non-`registry.k8s.io` upstream returns `404` -> fallback is not attempted
- fallback succeeds, second identical request is served from cache without
  re-fetching the original `404` path
- fallback-derived cached entry revalidates against the recorded effective
  upstream path instead of the original client-facing path

## Success criteria

- `k8s.hub.ipao.vip/coredns:v1.13.1` can succeed through fallback when the
  upstream only exposes `coredns/coredns`
- existing Docker Hub behavior remains unchanged
- existing `registry.k8s.io` paths that already work continue to use the
  original path first
- fallback is isolated to `registry.k8s.io`
