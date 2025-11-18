# Feature Specification: APT/APK 包缓存模块

**Feature Branch**: `007-apt-apk-cache`  
**Created**: 2025-11-17  
**Status**: Draft  
**Input**: User description: "Title: 设计并实现 APT/APK 包缓存模块以加速 Docker 构建 Goal: 为 any-hub 新增一个 debian/apk 模块，代理本地 Docker 构建中的 apt-get/apk add 请求，支持缓存索引与包文件并透明回源。 Constraints: - 适配 Debian/Ubuntu APT 路径（Release/InRelease、Packages(.gz|.xz)、pool/*.deb）及 Alpine APKINDEX/packages。 - 索引/Release 需 RequireRevalidate=true；二进制包 AllowStore=true；签名文件透传。 - 兼容 Acquire-By-Hash（APT）与 APKINDEX 签名校验，不破坏客户端校验。 - 复用现有缓存架构（hooks.go + module.go + CacheStrategyProfile），不得影响现有 docker/npm/pypi/go/composer 模块。 Inputs: - 代码库路径：/home/rogee/Projects/any-hub - 参考模块：internal/hubmodule/docker/hooks.go (cachePolicy)、internal/hubmodule/golang/hooks.go、internal/proxy/handler.go (ETag/Last-Modified 再验证) Deliverables: - 新增 internal/hubmodule/debian（或 alpine）下的 hooks.go、module.go 实现 - 配套测试（模拟 apt-get update/install 或 apk update/add 的最小集） - 使用说明（简要配置示例，如何在 config.toml 添加 Hub） AcceptanceCriteria: - “apt-get update” 或 “apk update” 指向代理时，首次 miss 回源，二次命中缓存；索引文件可被最新上游版本触发再验证；二进制包稳定命中缓存。 新分支开头数字是 007"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - APT 更新通过代理 (Priority: P1)

本地构建镜像的开发者将 apt 源指向代理，运行 `apt-get update` 获取最新索引并希望后续重复构建无需再次访问官方仓。

**Why this priority**: APT 更新是后续安装的前提，需保证首次拉取成功且缓存生效以降低构建时间与外网依赖。

**Independent Test**: 将 apt 源指向代理后执行两次 `apt-get update`，验证首轮回源、次轮命中缓存且返回内容与官方一致。

**Acceptance Scenarios**:

1. **Given** 缓存为空，**When** 运行 `apt-get update` 指向代理，**Then** 返回官方索引内容且缓存写入成功（Release/InRelease/Packages）。
2. **Given** 同一索引已缓存，**When** 再次运行 `apt-get update`，**Then** 返回 200/304 且无额外上游 GET（仅必要的条件请求），输出内容与首轮一致。

---

### User Story 2 - APT 安装包命中缓存 (Priority: P2)

在构建步骤中安装常用 `.deb` 包，希望重复构建时直接从本地缓存提供 pool 下的包体。

**Why this priority**: 包文件体积大、下载耗时，命中缓存可显著缩短镜像构建时长。

**Independent Test**: 在完成 User Story 1 后，执行 `apt-get install <常见包>` 两次，确认首轮回源、次轮包体命中缓存，且校验/签名不受影响。

**Acceptance Scenarios**:

1. **Given** 缓存已有对应索引且无包体，**When** 首次安装某包，**Then** 包体成功下载并写入缓存，安装过程不因代理失败。
2. **Given** 同一包体已缓存，**When** 再次安装，**Then** 不触发上游包体下载，安装成功且校验通过。

---

### User Story 3 - Alpine APK 加速 (Priority: P3)

将 Alpine 利用代理执行 `apk update && apk add`，希望索引与包体同样缓存并可重复命中。

**Why this priority**: Alpine 镜像在容器场景常用，通过同一代理获得一致的构建加速。

**Independent Test**: 将 `/etc/apk/repositories` 指向代理，执行两轮 `apk update && apk add <常用包>`，验证首轮回源、次轮命中缓存且 APKINDEX/包体校验正常。

**Acceptance Scenarios**:

1. **Given** 代理配置完毕，**When** 首次执行 `apk update`，**Then** 获取 APKINDEX 并写入缓存，随后 `apk add` 成功下载包体。
2. **Given** 同一索引与包体已缓存，**When** 再次执行 `apk update && apk add`，**Then** 索引再验证后命中，包体直接命中缓存且安装成功。

---

### Edge Cases

- 上游索引更新：Release/InRelease 或 APKINDEX 发生变更时，代理应通过条件请求检测到并刷新缓存。
- 校验失败：Acquire-By-Hash 校验或 APKINDEX 签名不匹配时，应返回上游原始错误而非吞掉错误。
- 离线/超时：上游暂不可达时，若有缓存且策略允许，应优先返回缓存；无缓存则向客户端返回明确错误。
- 部分源未配置：多源场景下，某源缺失或路径拼写错误时，代理应返回与上游一致的 404/403。
- 大文件/磁盘不足：缓存写入包体时磁盘不足，应优雅失败并告警，不影响后续直接透传回源。

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: 代理必须支持 APT 获取与缓存 Release/InRelease、Packages(.gz/.xz) 及 pool 下的包体；索引请求需携带条件请求并可根据上游变更刷新缓存。
- **FR-002**: 代理必须支持 Acquire-By-Hash 路径，若哈希不匹配需返回上游一致的错误，不得用缓存污染返回。
- **FR-003**: 代理必须支持 Alpine APKINDEX 与 packages 路径的获取与缓存，索引需再验证，包体可直接命中缓存。
- **FR-004**: 首次命中失败的索引/包体要回源写入缓存；后续同一路径在缓存有效期内应直接命中，除非索引再验证判定有新版本。
- **FR-005**: 代理行为须保持透明：返回状态码、头信息、签名/校验文件与上游一致，日志需记录命中/回源/失败原因，便于构建诊断。
- **FR-006**: 新增 Hub 配置字段（如仓类型、上游地址）需有默认值与示例，且不得影响现有 docker/npm/pypi/go/composer 的配置与行为。

### Key Entities *(include if feature involves data)*

- **索引文件**：APT 的 Release/InRelease/Packages*，Alpine 的 APKINDEX；含校验或签名信息，需可再验证。
- **包体文件**：`pool/*/*.deb`、Alpine `packages/*`；内容不可变，按路径/哈希缓存。
- **签名/校验文件**：APT 的 Release.gpg、Acquire-By-Hash 路径；用于客户端校验，需原样透传。
- **缓存条目**：存储索引或包体的本地副本，包含写入时间与可选校验标识，用于命中与再验证判定。

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 在同一构建机器上，第二次执行 `apt-get update` 或 `apk update` 对同一源时，上游请求数较首次减少 ≥90%（仅保留必要条件请求），且耗时降低至少 50%。
- **SC-002**: 已缓存的包体再次安装时，不触发上游下载，安装成功率达到 100%（以连续 20 次同包安装为样本）。
- **SC-003**: 当上游索引有新版本发布时，下一次索引请求能够在一次交互内获取最新数据（无旧数据残留），并保持客户端校验/签名通过率 100%。
- **SC-004**: 引入新模块后，现有 docker/npm/pypi/go/composer 代理的功能及配置使用无变化，回归用例全部通过。
