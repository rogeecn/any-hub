# Tasks: APT/APK 包缓存模块

**Input**: Design documents from `/specs/007-apt-apk-cache/`
**Prerequisites**: plan.md (required), spec.md (user stories), research.md, data-model.md, contracts/

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: User story label (US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare module folders and examples for new hubs.

- [ ] T001 Create module directories `internal/hubmodule/debian/` and `internal/hubmodule/apk/` with placeholder go files (module.go/hooks.go scaffolds).
- [ ] T002 Add sample hub entries for APT/APK in `configs/config.example.toml`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core wiring for new hub types before story work.

- [ ] T003 Update hub type validation to accept `debian` 和 `apk` in `internal/config/validation.go`.
- [ ] T004 Register new modules in `internal/config/modules.go` and `internal/hubmodule/registry.go` (init side-effect includes debian/apk).
- [ ] T005 [P] Define debian module metadata (cache strategy, TTL, validation mode) in `internal/hubmodule/debian/module.go`.
- [ ] T006 [P] Define apk module metadata in `internal/hubmodule/apk/module.go`.
- [ ] T007 Ensure path locator rewrite strategy (raw_path) reused for new modules in `internal/hubmodule/strategy.go` or module options if needed.
- [ ] T008 Add constitution-mandated Chinese comments for new module metadata files.

**Checkpoint**: Foundation ready—new hub types recognized, modules load without runtime errors.

---

## Phase 3: User Story 1 - APT 更新通过代理 (Priority: P1) 🎯 MVP

**Goal**: 代理 APT 索引（Release/InRelease/Packages*），首次回源，后续带条件请求再验证并命中缓存。

**Independent Test**: `apt-get update` 两次指向代理，首轮回源，次轮 304/命中缓存且内容与官方一致。

### Tests for User Story 1

- [ ] T009 [P] [US1] Add unit tests for path classification与缓存策略（索引 RequireRevalidate）在 `internal/hubmodule/debian/hooks_test.go`.
- [ ] T010 [US1] Add integration test `tests/integration/apt_update_proxy_test.go` covering first/second `apt-get update` (Release/InRelease/Packages) with httptest upstream and temp storage.

### Implementation for User Story 1

- [ ] T011 [P] [US1] Implement APT hooks (NormalizePath/CachePolicy/ContentType/ResolveUpstream if needed) for index paths in `internal/hubmodule/debian/hooks.go`.
- [ ] T012 [P] [US1] Support conditional requests (ETag/Last-Modified passthrough) for index responses in `internal/hubmodule/debian/hooks.go`.
- [ ] T013 [US1] Wire debian module registration to use hooks in `internal/hubmodule/debian/module.go` and ensure hook registration in `hooks.go`.
- [ ] T014 [US1] Ensure logging fields include cache hit/upstream for APT requests (reuse proxy logging) and document in comments `internal/hubmodule/debian/hooks.go`.
- [ ] T015 [US1] Update quickstart instructions with APT usage validation steps in `specs/007-apt-apk-cache/quickstart.md`.

**Checkpoint**: APT 索引更新可独立验证并缓存。

---

## Phase 4: User Story 2 - APT 安装包命中缓存 (Priority: P2)

**Goal**: pool 下 `.deb` 包首次回源、后续直接命中缓存；保持 Acquire-By-Hash 路径透传且不污染哈希校验。

**Independent Test**: `apt-get install <包>` 两次，首轮下载并缓存，次轮无上游下载，安装成功且校验通过。

### Tests for User Story 2

- [ ] T016 [P] [US2] Extend debian hook unit tests to cover `/pool/...` 与 `/by-hash/...` 缓存策略 in `internal/hubmodule/debian/hooks_test.go`.
- [ ] T017 [US2] Integration test for package download caching and Acquire-By-Hash passthrough in `tests/integration/apt_package_proxy_test.go`.

### Implementation for User Story 2

- [ ] T018 [P] [US2] Implement package/dist path handling (AllowCache/AllowStore, RequireRevalidate=false) in `internal/hubmodule/debian/hooks.go`.
- [ ] T019 [P] [US2] Handle `/dists/<suite>/by-hash/<algo>/<hash>` as immutable cached resources in `internal/hubmodule/debian/hooks.go`.
- [ ] T020 [US2] Validate cache writer/reader streaming for large deb files in `internal/proxy/handler.go` (ensure no full-buffer reads) with comments/tests if changes required.
- [ ] T021 [US2] Update config docs/examples if additional APT-specific knobs are added in `configs/config.example.toml` or `README.md`.

**Checkpoint**: APT 包体可命中缓存且哈希/签名校验保持一致。

---

## Phase 5: User Story 3 - Alpine APK 加速 (Priority: P3)

**Goal**: 缓存 APKINDEX 并再验证；包体（packages/*.apk）首次回源后直接命中缓存。

**Independent Test**: `apk update && apk add <包>` 两次，索引次轮 304/命中，包体次轮直接命中，安装成功。

### Tests for User Story 3

- [ ] T022 [P] [US3] Add apk hook unit tests for index/package path policy in `internal/hubmodule/apk/hooks_test.go`.
- [ ] T023 [US3] Integration test for apk update/install caching in `tests/integration/apk_proxy_test.go`.

### Implementation for User Story 3

- [ ] T024 [P] [US3] Implement APK hooks (CachePolicy/ContentType/NormalizePath) for APKINDEX and packages in `internal/hubmodule/apk/hooks.go`.
- [ ] T025 [P] [US3] Ensure APKINDEX/signature files RequireRevalidate and package files immutable cache in `internal/hubmodule/apk/hooks.go`.
- [ ] T026 [US3] Register apk hooks in module init and update logging/observability comments in `internal/hubmodule/apk/module.go`.
- [ ] T027 [US3] Add Alpine repository usage notes to `specs/007-apt-apk-cache/quickstart.md`.

**Checkpoint**: Alpine 索引与包体缓存可独立验证。

---

## Phase 6: Polish & Cross-Cutting Concerns

- [ ] T028 [P] Add Chinese comments for key caching logic and path handling in new hook files (`internal/hubmodule/debian/hooks.go`, `internal/hubmodule/apk/hooks.go`).
- [ ] T029 [P] Document log fields and cache semantics in `docs/` or `README.md` (structure log examples for APT/APK).
- [ ] T030 Validate gofmt/go test ./... and update `specs/007-apt-apk-cache/quickstart.md` with final verification steps.
- [ ] T031 [P] Confirm no regressions to existing modules via smoke test list in `tests/integration/` (reuse existing suites, adjust configs if needed).

---

## Dependencies & Execution Order

- Phase 1 → Phase 2 → User stories (Phase 3/4/5) → Phase 6.
- User Story 1 (P1) must complete before US2 (shares debian hooks); US3 can start after Phase 2 independently.
- Parallel opportunities:
  - T005/T006 module metadata in parallel; hook/unit work can run in parallel within each story where marked [P].
  - US3 tasks can run in parallel with US1 late-stage tasks once foundational ready (different modules/files).
- Suggested MVP: Complete Phases 1-3 (US1) to deliver APT 更新加速；US2/US3 incrementally after validation.
