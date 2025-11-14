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
1. Edit `config.toml` and set `Module = "<module-key>"` inside the target `[[Hub]]` block (omit to use `legacy`).
2. (Optional) Override cache behavior per hub using existing fields (`CacheTTL`, etc.).
3. Run `ANY_HUB_CONFIG=./config.toml go test ./...` to ensure loader validation passes.

## 4. Run and Verify
1. Start the binary: `go run ./cmd/any-hub --config ./config.toml`.
2. Send traffic to the hub's domain/port and watch logs for `module_key=<module-key>` tags.
3. Inspect `./storage/<hub>/` to confirm `.body` files are written by the module.
4. Exercise rollback by switching `Module` back to `legacy` if needed.

## 5. Ship
1. Commit module code + config docs.
2. Update release notes mentioning the module key, migration guidance, and related diagnostics.
3. Monitor cache hit/miss metrics post-deploy; adjust TTL overrides if necessary.
