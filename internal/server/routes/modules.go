package routes

import (
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/any-hub/any-hub/internal/hubmodule"
	"github.com/any-hub/any-hub/internal/proxy/hooks"
	"github.com/any-hub/any-hub/internal/server"
)

// RegisterModuleRoutes 暴露 /-/modules 诊断接口，供 SRE 查询模块与 Hub 绑定关系。
func RegisterModuleRoutes(app *fiber.App, registry *server.HubRegistry) {
	if app == nil || registry == nil {
		return
	}

	app.Get("/-/modules", func(c fiber.Ctx) error {
		hookStatus := hooks.Snapshot(hubmodule.Keys())
		payload := fiber.Map{
			"modules":       encodeModules(hubmodule.List(), hookStatus),
			"hubs":          encodeHubBindings(registry.List()),
			"hook_registry": hookStatus,
		}
		return c.JSON(payload)
	})

	app.Get("/-/modules/:key", func(c fiber.Ctx) error {
		key := strings.ToLower(strings.TrimSpace(c.Params("key")))
		if key == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "module_key_required"})
		}
		meta, ok := hubmodule.Resolve(key)
		if !ok {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "module_not_found"})
		}
		encoded := encodeModule(meta)
		encoded.HookStatus = hooks.Status(key)
		return c.JSON(encoded)
	})
}

type modulePayload struct {
	Key                string                   `json:"key"`
	Description        string                   `json:"description"`
	MigrationState     hubmodule.MigrationState `json:"migration_state"`
	SupportedProtocols []string                 `json:"supported_protocols"`
	CacheStrategy      cacheStrategyPayload     `json:"cache_strategy"`
	HookStatus         string                   `json:"hook_status,omitempty"`
}

type cacheStrategyPayload struct {
	TTLSeconds             int64  `json:"ttl_seconds"`
	ValidationMode         string `json:"validation_mode"`
	DiskLayout             string `json:"disk_layout"`
	RequiresMetadataFile   bool   `json:"requires_metadata_file"`
	SupportsStreamingWrite bool   `json:"supports_streaming_write"`
}

type hubBindingPayload struct {
	HubName   string `json:"hub_name"`
	ModuleKey string `json:"module_key"`
	Domain    string `json:"domain"`
	Port      int    `json:"port"`
	Rollout   string `json:"rollout_flag"`
	Legacy    bool   `json:"legacy_only"`
}

func encodeModules(mods []hubmodule.ModuleMetadata, status map[string]string) []modulePayload {
	if len(mods) == 0 {
		return nil
	}
	sort.Slice(mods, func(i, j int) bool {
		return mods[i].Key < mods[j].Key
	})
	result := make([]modulePayload, 0, len(mods))
	for _, meta := range mods {
		item := encodeModule(meta)
		if s, ok := status[meta.Key]; ok {
			item.HookStatus = s
		}
		result = append(result, item)
	}
	return result
}

func encodeModule(meta hubmodule.ModuleMetadata) modulePayload {
	strategy := meta.CacheStrategy
	return modulePayload{
		Key:                meta.Key,
		Description:        meta.Description,
		MigrationState:     meta.MigrationState,
		SupportedProtocols: append([]string(nil), meta.SupportedProtocols...),
		CacheStrategy: cacheStrategyPayload{
			TTLSeconds:             int64(strategy.TTLHint / time.Second),
			ValidationMode:         string(strategy.ValidationMode),
			DiskLayout:             strategy.DiskLayout,
			RequiresMetadataFile:   strategy.RequiresMetadataFile,
			SupportsStreamingWrite: strategy.SupportsStreamingWrite,
		},
	}
}

func encodeHubBindings(routes []server.HubRoute) []hubBindingPayload {
	if len(routes) == 0 {
		return nil
	}
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Config.Name < routes[j].Config.Name
	})
	result := make([]hubBindingPayload, 0, len(routes))
	for _, route := range routes {
		result = append(result, hubBindingPayload{
			HubName:   route.Config.Name,
			ModuleKey: route.ModuleKey,
			Domain:    route.Config.Domain,
			Port:      route.ListenPort,
			Rollout:   string(route.RolloutFlag),
			Legacy:    route.ModuleKey == hubmodule.DefaultModuleKey(),
		})
	}
	return result
}
