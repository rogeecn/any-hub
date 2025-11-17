# any-hub Architecture (Mermaid)

```mermaid
flowchart LR
    subgraph Clients
        user[Anonymous Clients]
    end

    subgraph Server["Fiber App (internal/server)"]
        routes["/- routes & Hub registry\ninternal/server/routes"]
        forwarder["Forwarder (module dispatch)\ninternal/proxy/forwarder"]
        handler["Proxy Handler (orchestration)\ninternal/proxy/handler"]
    end

    subgraph Modules["Hub Modules + Hooks"]
        hookreg["Hook Registry\ninternal/proxy/hooks"]
        hubreg["Module Metadata Registry\ninternal/hubmodule"]
        hooks["Module Hooks\n(docker/npm/pypi/composer/go)"]
    end

    subgraph CacheAndUpstream["Cache + Upstream"]
        store["Cache Store\ninternal/cache"]
        upstream["External Upstreams\nregistry/npm/simple/etc"]
    end

    user -->|HTTP| routes
    routes -->|HubRoute| forwarder
    forwarder -->|module_key| handler
    handler -->|lookup| hubreg
    handler -->|hook fetch| hookreg
    handler --> hooks
    handler -->|read/write| store
    handler -->|HTTP| upstream

    hooks --> handler
    store --> handler
    upstream --> handler
```

- Requests flow from clients into Fiber routes, which use the Hub registry to select a `HubRoute`.
- Forwarder chooses the module handler based on `module_key`; the proxy handler orchestrates cache lookup/write and upstream streaming.
- Module-specific logic (path normalization, upstream resolution, rewrites, cache policy) lives in Hooks per module; hook registry enforces registration.
- Cache store manages local filesystem layout, while upstreams provide original artifacts/content.
