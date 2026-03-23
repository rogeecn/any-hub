package proxy

import (
	"bytes"
	"net/url"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	"github.com/any-hub/any-hub/internal/config"
	"github.com/any-hub/any-hub/internal/hubmodule"
	"github.com/any-hub/any-hub/internal/server"
)

func TestRegistryK8sManifestFallbackLogsStructuredEvent(t *testing.T) {
	buf := new(bytes.Buffer)
	logger := logrus.New()
	logger.SetOutput(buf)
	logger.SetFormatter(&logrus.JSONFormatter{})

	upstreamURL, err := url.Parse("http://registry.k8s.io")
	if err != nil {
		t.Fatalf("parse upstream url: %v", err)
	}

	h := NewHandler(nil, logger, nil)
	route := &server.HubRoute{
		Config: config.HubConfig{
			Name:   "docker",
			Domain: "k8s.hub.local",
			Type:   "docker",
		},
		Module:      hubmodule.ModuleMetadata{Key: "docker"},
		UpstreamURL: upstreamURL,
	}

	h.logRegistryK8sFallback(route, "req-1", "/v2/coredns/manifests/v1.13.1", "/v2/coredns/coredns/manifests/v1.13.1", 404, "GET")

	output := buf.String()
	for _, want := range []string{
		"proxy_registry_k8s_fallback",
		`"hub":"docker"`,
		`"domain":"k8s.hub.local"`,
		`"upstream_host":"registry.k8s.io"`,
		`"original_path":"/v2/coredns/manifests/v1.13.1"`,
		`"fallback_path":"/v2/coredns/coredns/manifests/v1.13.1"`,
		`"original_status":404`,
		`"method":"GET"`,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected log output to contain %s, got %s", want, output)
		}
	}
}
