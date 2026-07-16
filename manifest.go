package commerceext

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const manifestAPIVersion = "commerce.mira.dev/v1"

// ClientManifest is the deploy-level manifest (repo cliente).
type ClientManifest struct {
	APIVersion string `yaml:"apiVersion"`
	Core       struct {
		Image string `yaml:"image"`
	} `yaml:"core"`
	Modules  map[string]bool `yaml:"modules"`
	Plugins  []PluginRef     `yaml:"plugins"`
	Vertical struct {
		Preset string `yaml:"preset"`
	} `yaml:"vertical"`
}

// PluginManifest is kind: Plugin (per extension).
type PluginManifest struct {
	APIVersion     string `yaml:"apiVersion"`
	Kind           string `yaml:"kind"`
	ID             string `yaml:"id"`
	Version        string `yaml:"version"`
	CompatibleCore string `yaml:"compatibleCore"`
	Runtime        struct {
		Type   string `yaml:"type"`
		Binary string `yaml:"binary"`
	} `yaml:"runtime"`
	Capabilities struct {
		Hooks  []string `yaml:"hooks"`
		Events struct {
			Subscribe []string `yaml:"subscribe"`
			Publish   []string `yaml:"publish"`
		} `yaml:"events"`
		Integrations []string `yaml:"integrations"`
	} `yaml:"capabilities"`
}

// PluginRef is a plugin pin in the client manifest.
type PluginRef struct {
	ID      string `yaml:"id"`
	Version string `yaml:"version"`
}

// LoadClientManifest reads and parses a client manifest YAML file.
func LoadClientManifest(path string) (ClientManifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ClientManifest{}, err
	}
	var m ClientManifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return ClientManifest{}, err
	}
	return m, nil
}

// LoadPluginManifest reads a plugin manifest YAML file.
func LoadPluginManifest(path string) (PluginManifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return PluginManifest{}, err
	}
	var m PluginManifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return PluginManifest{}, err
	}
	return m, nil
}

// VerifyClientManifest validates client manifest against core version.
func VerifyClientManifest(m ClientManifest, coreVersion string) error {
	if m.APIVersion != "" && m.APIVersion != manifestAPIVersion {
		return fmt.Errorf("apiVersion %q unsupported (want %s)", m.APIVersion, manifestAPIVersion)
	}
	if m.Core.Image == "" {
		return fmt.Errorf("core.image is required")
	}
	if len(m.Plugins) == 0 {
		return fmt.Errorf("at least one plugin entry required for verify (or use --skip-plugins)")
	}
	for _, p := range m.Plugins {
		if strings.TrimSpace(p.ID) == "" {
			return fmt.Errorf("plugin id is required")
		}
	}
	_ = coreVersion
	return nil
}

// VerifyPluginManifest validates plugin manifest hooks/events against catalog.
func VerifyPluginManifest(m PluginManifest, coreVersion string) error {
	if m.Kind != "" && m.Kind != "Plugin" {
		return fmt.Errorf("kind must be Plugin")
	}
	if m.APIVersion != "" && m.APIVersion != manifestAPIVersion {
		return fmt.Errorf("apiVersion %q unsupported", m.APIVersion)
	}
	if m.ID == "" {
		return fmt.Errorf("id is required")
	}
	if m.Version == "" {
		return fmt.Errorf("version is required")
	}
	if m.CompatibleCore != "" && !compatibleCoreMatches(m.CompatibleCore, coreVersion) {
		return fmt.Errorf("compatibleCore %q does not match core %q", m.CompatibleCore, coreVersion)
	}
	for _, h := range m.Capabilities.Hooks {
		if !knownHook(h) {
			return fmt.Errorf("unknown hook %q", h)
		}
	}
	for _, e := range m.Capabilities.Events.Subscribe {
		if !knownEvent(e) {
			return fmt.Errorf("unknown subscribe event %q", e)
		}
	}
	return nil
}

func knownHook(id string) bool {
	for _, h := range AllHooks {
		if h == id {
			return true
		}
	}
	return false
}

func knownEvent(id string) bool {
	for _, e := range AllCoreEvents {
		if e == id {
			return true
		}
	}
	// namespaced plugin events: vendor.action
	return strings.Contains(id, ".")
}

func compatibleCoreMatches(rangeExpr, coreVersion string) bool {
	rangeExpr = strings.TrimSpace(rangeExpr)
	coreVersion = strings.TrimSpace(coreVersion)
	if rangeExpr == "" || coreVersion == "" {
		return true
	}
	if strings.HasPrefix(rangeExpr, "^") {
		prefix := strings.TrimPrefix(rangeExpr, "^")
		major := strings.Split(prefix, ".")[0]
		coreMajor := strings.Split(coreVersion, ".")[0]
		return major == coreMajor
	}
	return rangeExpr == coreVersion
}
