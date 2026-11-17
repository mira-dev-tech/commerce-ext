package commerceext

import "context"

// Meta describes a plugin identity.
type Meta struct {
	ID             string
	Version        string
	CompatibleCore string
	Description    string
}

// Plugin is implemented by a partner process (commerceext.Serve) or by an
// in-process plugin maintained inside the core. Partner code does not link
// the core and is not started by the API.
type Plugin interface {
	Meta() Meta
	Init(ctx context.Context, rt *Runtime) error
	Register(reg *Registry) error
	Shutdown(ctx context.Context) error
}

// Runtime is injected into plugins at Init.
type Runtime struct {
	Config  map[string]any
	Secrets map[string]string
	Logger  Logger
	Events  EventPublisher
}

// Logger is a minimal structured logger for plugins.
type Logger interface {
	Info(msg string, kv ...any)
	Warn(msg string, kv ...any)
	Error(msg string, kv ...any)
}

// EventPublisher allows plugins to subscribe to core events.
type EventPublisher interface {
	Subscribe(eventType, pluginID string, handler EventHandler)
}

// NopLogger discards log lines.
type NopLogger struct{}

func (NopLogger) Info(string, ...any)  {}
func (NopLogger) Warn(string, ...any)  {}
func (NopLogger) Error(string, ...any) {}
