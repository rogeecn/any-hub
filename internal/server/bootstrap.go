package server

import (
	"fmt"

	"github.com/any-hub/any-hub/internal/config"
	"github.com/any-hub/any-hub/internal/hubmodule"
)

func moduleMetadataForHub(hub config.HubConfig) (hubmodule.ModuleMetadata, error) {
	if meta, ok := hubmodule.Resolve(hub.Module); ok {
		return meta, nil
	}
	return hubmodule.ModuleMetadata{}, fmt.Errorf("module %s is not registered", hub.Module)
}
