package config

import (
	"testing"
)

func TestCloudConfigTemplating(t *testing.T) {
	cfg, err := newConfigFromBytes([]byte(MinimalConfigYaml))
	if err != nil {
		t.Fatalf("Unable to load cluster config: %v", err)
	}

	if err := cfg.GenerateDefaultAssets(); err != nil {
		t.Fatalf("Error reading assets from files: %v", err)
	}

	//Template and encode tls assets
	if err := cfg.TLSConfig.buffers.TemplateBuffers(cfg); err != nil {
		t.Fatalf("Failed generating TLS assets: %v", err)
	}
	if err := cfg.TLSConfig.buffers.EncodeBuffers(); err != nil {
		t.Fatalf("Failed encoding TLS assets: %v", err)
	}

	if err := cfg.UserData.buffers.TemplateBuffers(cfg); err != nil {
		t.Fatalf("Failed templating userdata assets: %v", err)
	}

	if err := cfg.UserData.validate(); err != nil {
		t.Fatalf("Invalid userdata : %v", err)
	}
}
