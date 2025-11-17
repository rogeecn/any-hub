package template

import (
	"testing"

	"github.com/any-hub/any-hub/internal/proxy/hooks"
)

// This test acts as a usage example for module authors.
func TestExampleHookDefinition(t *testing.T) {
	h := hooks.Hooks{
		NormalizePath: func(ctx *hooks.RequestContext, clean string, rawQuery []byte) (string, []byte) {
			return clean, rawQuery
		},
		CachePolicy: func(ctx *hooks.RequestContext, path string, current hooks.CachePolicy) hooks.CachePolicy {
			current.AllowCache = true
			current.AllowStore = true
			return current
		},
	}
	_ = h
}
