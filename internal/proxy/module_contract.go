package proxy

import (
	"errors"
	"fmt"
	"strings"

	"github.com/any-hub/any-hub/internal/server"
)

// ModuleHandler is the runtime contract each hubmodule must provide to serve requests.
// It aligns with server.ProxyHandler so existing handlers remain compatible.
type ModuleHandler = server.ProxyHandler

// ModuleRegistration captures a module_key and its handler for safe registration.
// Future registration flows can validate this struct before wiring into the dispatcher.
type ModuleRegistration struct {
	Key     string
	Handler ModuleHandler
}

// ErrModuleHandlerExists indicates a handler has already been registered for the key.
var ErrModuleHandlerExists = errors.New("module handler already registered")

// Validate ensures both key and handler are present before registration.
func (r ModuleRegistration) Validate() error {
	if strings.TrimSpace(r.Key) == "" {
		return errors.New("module key required")
	}
	if r.Handler == nil {
		return errors.New("module handler required")
	}
	return nil
}

// RegisterModule registers validated metadata/runtime handler pair.
func RegisterModule(reg ModuleRegistration) error {
	if err := reg.Validate(); err != nil {
		return err
	}
	normalized := normalizeModuleKey(reg.Key)
	if normalized == "" {
		return errors.New("module key required")
	}
	if _, loaded := moduleHandlers.LoadOrStore(normalized, reg.Handler); loaded {
		return fmt.Errorf("%w: %s", ErrModuleHandlerExists, normalized)
	}
	return nil
}

// MustRegisterModule panics when registration fails; suitable for module init().
func MustRegisterModule(reg ModuleRegistration) {
	if err := RegisterModule(reg); err != nil {
		panic(err)
	}
}
