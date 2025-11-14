# Data Model: Modular Proxy & Cache Segmentation

## Overview

The modular architecture introduces explicit metadata describing which proxy+cache module each hub uses, how modules register themselves, and what cache policies they expose. The underlying storage layout (`StoragePath/<Hub>/<path>.body`) remains unchanged, but new metadata ensures the runtime can resolve modules, enforce compatibility, and migrate legacy hubs incrementally.

## Entities

### 1. HubConfigEntry
- **Source**: `[[Hub]]` blocks in `config.toml` (decoded via `internal/config`).
- **Fields**:
  - `Name` *(string, required)* – unique per config; used as hub identifier and storage namespace.
  - `Domain` *(string, required)* – hostname clients access; must be unique per process.
  - `Port` *(int, required)* – listen port; validated to 1–65535.
  - `Upstream` *(string, required)* – base URL for upstream registry; must be HTTPS or explicitly whitelisted HTTP.
  - `Module` *(string, optional, default `"legacy"`)* – key resolved through module registry. Validation ensures module exists at load time.
  - `CacheTTL`, `Proxy`, and other overrides *(optional)* – reuse existing schema; modules may read these via dependency injection.
- **Relationships**:
  - `HubConfigEntry.Module` → `ModuleMetadata.Key` (many-to-one).
- **Validation Rules**:
  - Missing `Module` implicitly maps to `legacy` to preserve backward compatibility.
  - Changing `Module` requires a migration plan; config loader logs module name for observability.

### 2. ModuleMetadata
- **Fields**:
  - `Key` *(string, required)* – canonical identifier (e.g., `npm-tarball`).
  - `Description` *(string)* – human-readable summary.
  - `SupportedProtocols` *([]string)* – e.g., `HTTP`, `HTTPS`, `OCI`.
  - `CacheStrategy` *(CacheStrategyProfile)* – embedded policy descriptor.
  - `MigrationState` *(enum: `legacy`, `beta`, `ga`)* – used for rollout dashboards.
  - `Factory` *(function)* – constructs proxy+cache handlers; not serialized but referenced in registry code.
- **Relationships**:
  - One `ModuleMetadata` may serve many hubs via config binding.

### 3. ModuleRegistry
- **Representation**: in-memory map maintained by `internal/hubmodule/registry.go` at process boot.
- **Fields**:
  - `Modules` *(map[string]ModuleMetadata)* – keyed by `ModuleMetadata.Key`.
  - `DefaultKey` *(string)* – `legacy`.
- **Behavior**:
  - `Register(meta ModuleMetadata)` called during init of each module package.
  - `Resolve(key string) (ModuleMetadata, error)` used by router bootstrap; errors bubble to config validation.
- **Constraints**:
  - Duplicate registrations fail fast.
  - Registry must export a list function for diagnostics (`List()`), enabling observability endpoints if needed.

### 4. CacheStrategyProfile
- **Fields**:
  - `TTL` *(duration)* – default TTL per module; hubs may override via config.
  - `ValidationMode` *(enum: `etag`, `last-modified`, `never`)* – defines revalidation behavior.
  - `DiskLayout` *(string)* – description of path mapping rules (default `.body` suffix).
  - `RequiresMetadataFile` *(bool)* – whether `.meta` entries are required.
  - `SupportsStreamingWrite` *(bool)* – indicates module can write cache while proxying upstream.
- **Relationships**:
  - Owned by `ModuleMetadata`; not independently referenced.
- **Validation**:
  - TTL must be positive.
  - Modules flagged as `SupportsStreamingWrite=false` must document fallback behavior before registration.

### 5. LegacyAdapterState
- **Purpose**: Tracks which hubs still run through the old shared implementation to support progressive migration.
- **Fields**:
  - `HubName` *(string)* – references `HubConfigEntry.Name`.
  - ` rolloutFlag` *(enum: `legacy-only`, `dual`, `modular`)* – indicates traffic split for that hub.
  - `FallbackDeadline` *(timestamp, optional)* – when legacy path will be removed.
- **Storage**: In-memory map derived from config + environment flags; optionally surfaced via diagnostics endpoint.

## State Transitions

1. **Module Adoption**
   - Start: `HubConfigEntry.Module = "legacy"`.
   - Transition: operator edits config to new module key, runs validation.
   - Result: registry resolves new module, `LegacyAdapterState` updated to `dual` until rollout flag toggled fully.

2. **Cache Strategy Update**
   - Start: Module uses default TTL.
   - Transition: hub-level override applied in config.
   - Result: Module receives override via dependency injection and persists it in module-local settings without affecting other hubs.

3. **Module Registration Lifecycle**
   - Start: module package calls `Register` in its `init()`.
   - Transition: duplicate key registration rejected; module must rename key or remove old registration.
   - Result: `ModuleRegistry.Modules[key]` available during server bootstrap.

## Data Volume & Scale Assumptions

- Module metadata count is small (<20) and loaded entirely in memory.
- Hub count typically <50 per binary, so per-hub module resolution happens at startup and is cached.
- Disk usage remains the dominant storage cost; metadata adds negligible overhead.

## Identity & Uniqueness Rules

- `HubConfigEntry.Name` and `ModuleMetadata.Key` must each be unique (case-insensitive) within a config/process.
- Module registry rejects duplicate keys to avoid ambiguous bindings.

