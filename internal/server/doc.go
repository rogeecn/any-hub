// Package server hosts the Fiber HTTP service, request middleware chain, and
// hub registry glue that wires Host/port resolution into proxy handlers.
// Phase 1 focuses on a single binary that bootstraps Fiber, attaches logging
// and error middlewares, injects the HubRegistry built from config, and exposes
// router constructors that other packages (cmd/any-hub, proxy) can reuse.
// Future phases may extend this package with TLS, metrics endpoints, or admin
// surfaces, so keep exports narrow and accept explicit dependencies.
package server
