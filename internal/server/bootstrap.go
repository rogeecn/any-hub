package server

import (
	"fmt"

	"github.com/any-hub/any-hub/internal/hubmodule"
)

func moduleMetadataForKey(key string) (hubmodule.ModuleMetadata, error) {
	if meta, ok := hubmodule.Resolve(key); ok {
		return meta, nil
	}
	return hubmodule.ModuleMetadata{}, fmt.Errorf("module %s is not registered", key)
}
