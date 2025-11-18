# any-hub Development Guidelines

Auto-generated from all feature plans. Last updated: 2025-11-13

## Active Technologies
- Go 1.25+ (静态链接，单二进制交付) + Fiber v3（HTTP 服务）、Viper（配置）、Logrus + Lumberjack（结构化日志 & 滚动）、标准库 `net/http`/`io` (002-fiber-single-proxy)
- 本地文件系统缓存目录 `StoragePath/<Hub>/<path>`，结合文件 `mtime` + 上游 HEAD 再验证 (002-fiber-single-proxy)
- Go 1.25+（静态链接单二进制） + Fiber v3（HTTP 服务）、Viper（配置加载/校验）、Logrus + Lumberjack（结构化日志 & 滚动）、标准库 `net/http`/`io`（代理回源） (003-hub-auth-fields)
- 仍使用本地 `StoragePath/<Hub>/<path>` 目录缓存正文，并依赖 HEAD 对动态标签再验证 (003-hub-auth-fields)
- 本地文件系统缓存目录 `StoragePath/<Hub>/<path>`，模块需直接复用原始路径布局 (004-modular-proxy-cache)
- 本地文件系统缓存目录 `StoragePath/<Hub>/<path>`（按模块定义的布局） (005-proxy-module-delegation)
- 本地文件系统缓存目录 `StoragePath/<Hub>/<path>`（由模块 Hook 定义布局） (006-module-hook-refactor)
- Go 1.25+ (静态链接，单二进制) + Fiber v3（HTTP 服务）、Viper（配置）、Logrus + Lumberjack、标准库 `net/http`/`io` (007-apt-apk-cache)
- 本地 `StoragePath/<Hub>/<path>` + `.meta`；索引需带 Last-Modified/ETag 元信息；包体按原路径落盘 (007-apt-apk-cache)

- Go 1.25+ (静态链接，单二进制交付) + Fiber v3（HTTP 服务）、Viper（配置）、Logrus + Lumberjack（结构化日志 [EXTRACTED FROM ALL PLAN.MD FILES] 滚动）、标准库 `net/http`/`io` (001-config-bootstrap)

## Project Structure

```text
src/
tests/
```

## Commands

# Add commands for Go 1.25+ (静态链接，单二进制交付)

## Code Style

Go 1.25+ (静态链接，单二进制交付): Follow standard conventions

## Recent Changes
- 007-apt-apk-cache: Added Go 1.25+ (静态链接，单二进制) + Fiber v3（HTTP 服务）、Viper（配置）、Logrus + Lumberjack、标准库 `net/http`/`io`
- 006-module-hook-refactor: Added Go 1.25+ (静态链接，单二进制交付) + Fiber v3（HTTP 服务）、Viper（配置）、Logrus + Lumberjack（结构化日志 & 滚动）、标准库 `net/http`/`io`
- 005-proxy-module-delegation: Added Go 1.25+ (静态链接，单二进制交付) + Fiber v3（HTTP 服务）、Viper（配置）、Logrus + Lumberjack（结构化日志 & 滚动）、标准库 `net/http`/`io`


<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
