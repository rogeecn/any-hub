# Quickstart: Modular Proxy & Cache Segmentation

## 1. Prepare Workspace
1. Ensure Go 1.25+ toolchain is installed (`go version`).
2. From repo root, run `go mod tidy` (or `make deps` if defined) to sync modules.
3. Export `ANY_HUB_CONFIG` pointing to your working config (optional).

## 2. Create/Update Hub Module
1. Copy `internal/hubmodule/template/` to `internal/hubmodule/<module-key>/` and rename the package/types.
2. In the new package's `init()`, call `hubmodule.MustRegister(hubmodule.ModuleMetadata{Key: "<module-key>", ...})` to describe supported protocols、缓存策略与迁移阶段。
3. Register runtime behavior (proxy handler) from your module by calling `proxy.RegisterModuleHandler("<module-key>", handler)` during initialization.
4. Add tests under the module directory and run `make modules-test` (delegates to `go test ./internal/hubmodule/...`).

## 3. Bind Module via Config
1. Add your module type to `internal/config/validation.go` and the sample config if it represents a new protocol.
2. Edit `config.toml` and set `Type = "<module-type>"` inside the target `[[Hub]]` block.
3. (Optional) Override cache behavior per hub using existing fields (`CacheTTL`, etc.).
4. Run `ANY_HUB_CONFIG=./config.toml go test ./...` (or `make modules-test`) to ensure loader validation passes and the module registry sees your key.

## 4. Run and Verify
1. Start the binary: `go run ./cmd/any-hub --config ./config.toml`.
2. Use `curl -H "Host: <hub-domain>" http://127.0.0.1:<port>/<path>` to produce traffic, then hit `curl http://127.0.0.1:<port>/-/modules` and confirm the hub binding points to your module key.
3. Inspect `./storage/<hub>/` to confirm the cached files mirror the upstream path (no suffix). When a path also has child entries (e.g., `/pkg` metadata plus `/pkg/-/...` tarballs), the metadata payload is stored in a `__content` file under that directory so both artifacts can coexist. PyPI Simple responses rewrite distribution links to `/files/<scheme>/<host>/<path>` so that wheels/tarballs are fetched through the proxy and cached alongside the HTML/JSON index. Verify TTL overrides are propagated.
4. Monitor `logs/any-hub.log` (or the sample `logs/module_migration_sample.log`) to verify each entry exposes `module_key`. Example:
   ```json
   {"action":"proxy","hub":"testhub","module_key":"testhub","cache_hit":false,"upstream_status":200}
   ```
5. Exercise rollback by reverting the config change (or type rename) and re-running the traffic to ensure diagnostics/logs show the transition.

## 5. Ship
1. Commit module code + config docs.
2. Update release notes mentioning the module key, migration guidance, and related diagnostics.
3. Monitor cache hit/miss metrics post-deploy; adjust TTL overrides if necessary.

## 6. Attach Validation Artifacts
- Save the JSON snapshot from `/-/modules` and a short log excerpt (see `logs/module_migration_sample.log`) with both legacy + modular hubs present; attach them to the change request so reviewers can confirm you followed the playbook.
