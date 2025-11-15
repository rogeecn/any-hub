# Modular Hub Migration Playbook

This playbook describes how to cut a hub over from the shared legacy adapter to a dedicated module using the new rollout flags, diagnostics endpoint, and structured logs delivered in feature `004-modular-proxy-cache`.

## Prerequisites

- Target module must be registered via `hubmodule.MustRegister` and expose a proxy handler through `proxy.RegisterModuleHandler`.
- `config.toml` must already map the hub to its target module through `[[Hub]].Module`.
- Operators must have access to the running binary port (default `:5000`) to query `/-/modules`.

## Rollout Workflow

1. **Snapshot current state**  
   Run `curl -s http://localhost:5000/-/modules | jq '.hubs[] | select(.hub_name=="<hub>")'` to capture the current `module_key` and `rollout_flag`. Legacy hubs report `module_key=legacy` and `rollout_flag=legacy-only`.

2. **Prepare config for dual traffic**  
   Edit the hub block to target the new module while keeping rollback safety:

   ```toml
   [[Hub]]
   Name = "npm-prod"
   Domain = "npm.example.com"
   Upstream = "https://registry.npmjs.org"
   Module = "npm"
   Rollout = "dual"
   ```

   Dual mode keeps routing on the new module but keeps observability tagged as a partial rollout.

3. **Deploy and monitor**  
   Restart the service and tail logs filtered by `module_key`:

   ```sh
   jq 'select(.module_key=="npm" and .rollout_flag=="dual")' /var/log/any-hub.json
   ```

   Every request now carries `module_key`/`rollout_flag`, allowing dashboards or `grep`-based analyses without extra parsing.

4. **Verify diagnostics**  
   Query `/-/modules/npm` to inspect the registered metadata and confirm cache strategy, or `/-/modules` to ensure the hub binding reflects `rollout_flag=dual`.

5. **Promote to modular**  
   Once metrics are healthy, change `Rollout = "modular"` in config and redeploy. Continue monitoring logs to make sure both `module_key` and `rollout_flag` show the fully promoted state.

6. **Rollback procedure**  
   To rollback, set `Rollout = "legacy-only"` (without touching `Module`). The runtime forces traffic through the legacy module while keeping the desired module declaration for later reattempts. Confirm via diagnostics (`module_key` reverts to `legacy`) before announcing rollback complete.

## Observability Checklist

- **Logs**: Every proxy log line now contains `hub`, `module_key`, `rollout_flag`, upstream status, and `request_id`. Capture at least five minutes of traffic per flag change.
- **Diagnostics**: Store JSON snapshots from `/-/modules` before and after each rollout stage for incident timelines.
- **Config History**: Keep the `config.toml` diff (especially `Rollout` changes) attached to change records for auditability.

## Troubleshooting

- **Error: `module_not_found` during diagnostics** → module key not registered; ensure the module package’s `init()` calls `hubmodule.MustRegister`.
- **Requests still tagged with `legacy-only` after promotion** → double-check the running process uses the updated config path (`ANY_HUB_CONFIG` vs `--config`) and restart the service.
- **Diagnostics 404** → confirm you are hitting the correct port and that the CLI user/network path allows HTTP access; the endpoint ignores Host headers, so `curl http://127.0.0.1:<port>/-/modules` should succeed locally.
