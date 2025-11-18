# Quickstart: APT/APK 缓存代理

## 1) 启用新 Hub

在 `config.toml` 增加示例：

```toml
[[Hub]]
Domain = "apt.hub.local"
Name = "apt"
Upstream = "https://mirrors.edge.kernel.org/ubuntu"
Type = "debian"
Module = "debian" # 待实现模块键

[[Hub]]
Domain = "apk.hub.local"
Name = "apk"
Upstream = "https://dl-cdn.alpinelinux.org/alpine"
Type = "apk"
Module = "apk" # 待实现模块键
```

## 2) 指向代理

- APT：将 `/etc/apt/sources.list` 中的官方域名替换为 `http://apt.hub.local:5000`（或全局 ListenPort），保持原 suite/component 路径。
- APK：在 `/etc/apk/repositories` 中写入 `http://apk.hub.local:5000/v3.19/main` 等路径（与全局 ListenPort 一致）。

## 3) 验证

```bash
# APT
apt-get update
apt-get install -y curl
apt-get install -y curl # 第二次安装应直接命中缓存（pool/* 与 by-hash/* 不再 HEAD 再验证）

# Alpine
apk update
apk add curl
apk add curl # 第二次安装应直接命中缓存（APKINDEX 再验证，packages/*.apk 直接命中）
```

观察 `logs/` 输出：首次请求应为回源，二次请求命中缓存（索引可能返回 304）。如上游不可达且缓存已有包体，应继续命中缓存；无缓存则透传错误。
