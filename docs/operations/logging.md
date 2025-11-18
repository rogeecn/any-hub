# Logging & Cache Semantics (APT/APK)

## Common Fields
- `hub`/`domain`/`hub_type`：当前 Hub 标识与协议类型，例如 `debian`/`apk`。
- `module_key`：命中的模块键（与 `Type` 同名）。
- `cache_hit`：`true` 表示直接复用缓存；`false` 表示从上游获取或已刷新。
- `upstream`/`upstream_status`：实际访问的上游地址与状态码。
- `action`：`proxy`，表明代理链路日志。

## APT (debian 模块)
- 索引路径（Release/InRelease/Packages*）：`cache_hit=true` 仍会在后台进行 HEAD 再验证；命中 304 时保持缓存。
- 包体路径（`/pool/*` 和 `/dists/.../by-hash/...`）：视为不可变，首次 GET 落盘，后续直接命中，无 HEAD。
- 日志可结合 `X-Any-Hub-Cache-Hit` 响应头进行对照。

## APK (apk 模块)
- APKINDEX 及签名：每次命中会触发 HEAD 再验证，缓存命中返回 304 时继续使用本地文件。
- 包体 (`packages/*.apk`)：不可变资源，首轮 GET 落盘，后续直接命中，无 HEAD。

## Quick Checks
- 观察 `cache_hit` 与 `upstream_status`：`cache_hit=true`、`upstream_status=200/304` 表示缓存复用成功；`cache_hit=false` 表示回源或刷新。
- 若 `module_key` 与配置的 `Type` 不符，检查该类型的 hook 是否已注册，或是否误用了旧版二进制。
