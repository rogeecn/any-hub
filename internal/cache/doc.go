// Package cache defines the disk-backed store responsible for translating hub
// requests into StoragePath/<hub>/<path> files. The store exposes read/write
// primitives with safe semantics (temp file + rename) and surfaces file info
// (size, modtime) for higher layers to implement conditional revalidation.
// Proxy handlers depend on this package to stream cached responses or trigger
// upstream fetches without duplicating filesystem logic.
package cache
