# Module Binding Notes

Legacy rollout flags (`Module`/`Rollout`) have been removed. Hubs now bind to modules solely through their `Type` values, which map 1:1 to registered modules (docker, npm, go, pypi, composer, debian, apk, ...).

## Migrating to a New Module

1. **Register the module**  
   Implement the new module under `internal/hubmodule/<type>/`, call `hubmodule.MustRegister` in `init()`, and register hooks via `hooks.MustRegister`.

2. **Expose a handler**  
   New modules continue to reuse the shared proxy handler registered via `proxy.RegisterModule`. No per-module handler wiring is required unless the module supplies a bespoke handler.

3. **Update the config schema**  
   Add the new type to `internal/config/validation.go`’s `supportedHubTypes`, then redeploy. Every hub that should use the new module only needs `Type = "<type>"` plus the usual `Domain`/`Upstream` fields.

4. **Verify diagnostics**  
   `curl http://127.0.0.1:<port>/-/modules` to ensure the new type appears under `modules[]` and that the desired hubs show `module_key="<type>"`.

5. **Monitor logs**  
   Structured logs still carry `module_key`, making it easy to confirm that traffic is flowing through the expected module. Example:

   ```json
   {"action":"proxy","hub":"npm","module_key":"npm","cache_hit":false,"upstream_status":200}
   ```

6. **Rollback**  
   Since modules are now type-driven, rollback is as simple as reverting the `Type` value (or config deployment) back to the previous module’s type.

## Troubleshooting

- **`module_not_found` in diagnostics** → ensure the module registered via `hubmodule.MustRegister` before the hub references its type.
- **Hooks missing** → `/-/modules` exposes `hook_registry`; confirm the new type reports `registered`.
- **Unexpected module key in logs** → confirm the running binary includes your module (imported in `internal/config/modules.go`) and that the config `/--config` path matches the deployed file.
