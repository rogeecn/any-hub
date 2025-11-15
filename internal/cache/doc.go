// Package cache defines the disk-backed store responsible for translating hub
// requests into StoragePath/<hub>/<path> directories that mirror upstream
// paths. When a given path also needs to act as the parent of other entries
// (例如 npm metadata + tarball目录), the body is stored in a `__content` file
// under that directory so两种形态可以共存。The store exposes read/write primitives
// with safe semantics (temp file + rename) and surfaces file info (size, modtime)
// for higher layers to implement conditional revalidation. Proxy handlers depend
// on this package to stream cached responses or trigger upstream fetches without
// duplicating filesystem logic.
package cache
