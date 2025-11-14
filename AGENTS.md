# any-hub Development Guidelines

Auto-generated from all feature plans. Last updated: 2025-11-13

## Active Technologies
- Go 1.25+ (静态链接，单二进制交付) + Fiber v3（HTTP 服务）、Viper（配置）、Logrus + Lumberjack（结构化日志 & 滚动）、标准库 `net/http`/`io` (002-fiber-single-proxy)
- 本地文件系统缓存目录 `StoragePath/<Hub>/<path>`，结合文件 `mtime` + 上游 HEAD 再验证 (002-fiber-single-proxy)
- Go 1.25+（静态链接单二进制） + Fiber v3（HTTP 服务）、Viper（配置加载/校验）、Logrus + Lumberjack（结构化日志 & 滚动）、标准库 `net/http`/`io`（代理回源） (003-hub-auth-fields)
- 仍使用本地 `StoragePath/<Hub>/<path>` 目录缓存正文，并依赖 HEAD 对动态标签再验证 (003-hub-auth-fields)
- 本地文件系统缓存目录 `StoragePath/<Hub>/<path>.body` + `.meta` 元数据（模块必须复用同一布局） (004-modular-proxy-cache)

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
- 004-modular-proxy-cache: Added Go 1.25+ (静态链接，单二进制交付) + Fiber v3（HTTP 服务）、Viper（配置）、Logrus + Lumberjack（结构化日志 & 滚动）、标准库 `net/http`/`io`
- 003-hub-auth-fields: Added Go 1.25+（静态链接单二进制） + Fiber v3（HTTP 服务）、Viper（配置加载/校验）、Logrus + Lumberjack（结构化日志 & 滚动）、标准库 `net/http`/`io`（代理回源）
- 002-fiber-single-proxy: Added Go 1.25+ (静态链接，单二进制交付) + Fiber v3（HTTP 服务）、Viper（配置）、Logrus + Lumberjack（结构化日志 & 滚动）、标准库 `net/http`/`io`


<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
