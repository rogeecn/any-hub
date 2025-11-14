# Config Schema Contract

```toml
# Global section
ListenPort = 5000
LogLevel = "info"
StoragePath = "./storage"
CacheTTL = 86400
# ... existing global keys ...

[[Hub]]
Name = "docker"
Domain = "docker.hub.local"
Upstream = "https://registry-1.docker.io"
Proxy = ""
Username = ""        # optional
Password = ""        # optional
Type = "docker"       # required, must be docker|npm|go
CacheTTL = 43200       # optional override
```
```

| Field                | Required | Type    | Notes |
|----------------------|----------|---------|-------|
| `ListenPort`         | Yes      | int     | 单端口监听，范围 1-65535 |
| `Username`/`Password`| No       | string  | 缺省表示匿名；若任一非空则视为 credentialed，日志仅显示掩码 |
| `Type`               | Yes      | enum    | 当前支持 `docker`/`npm`/`go`，输入其他值时报错 |

**Validation Contract**
1. `any-hub --check-config` 必须在缺失 `Type` 或非法 `ListenPort` 时返回非 0。
2. CLI 日志需输出 `hub_type` 与 `auth_mode` 字段，以便运维确认配置是否生效。
