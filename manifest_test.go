package commerceext

import (
	"os"
	"testing"
)

func TestVerifyPluginManifestValid(t *testing.T) {
	m := PluginManifest{
		APIVersion:     "commerce.mira.dev/v1",
		Kind:           "Plugin",
		ID:             "reference-antifraud",
		Version:        "1.0.0",
		CompatibleCore: "^0.2.0",
	}
	m.Capabilities.Hooks = []string{HookCheckoutRiskAssess}
	m.Capabilities.Events.Subscribe = []string{EventPaymentAttemptFailed}
	if err := VerifyPluginManifest(m, CoreLine); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyPluginManifestUnknownHook(t *testing.T) {
	m := PluginManifest{ID: "x", Version: "1.0.0"}
	m.Capabilities.Hooks = []string{"unknown.hook"}
	if err := VerifyPluginManifest(m, CoreLine); err == nil {
		t.Fatal("expected error for unknown hook")
	}
}

func TestLoadPluginManifestFile(t *testing.T) {
	path := "../extensions/reference-antifraud/manifest.yaml"
	if _, err := os.Stat(path); err != nil {
		t.Skip("reference manifest not present")
	}
	m, err := LoadPluginManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyPluginManifest(m, CoreLine); err != nil {
		t.Fatal(err)
	}
}
