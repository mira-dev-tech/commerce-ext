package commerceext

import (
	"context"
	"errors"
	"time"
)

// Meta describes a plugin identity.
type Meta struct {
	ID             string
	Version        string
	CompatibleCore string
	Description    string
}

// Plugin is implemented by a partner process (Serve) or by an in-process
// plugin maintained inside the core. Partner code does not link the core
// and is not started by the API.
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
	// DB é o schema próprio do plugin (ADR 0227 D1/D3). nil quando o manifesto
	// não declara `database.schema`; usar devolve ErrNotProvisioned em vez de
	// estourar — pedir banco sem o declarar é erro de manifesto, e o plugin
	// tem de poder dizê-lo.
	DB PluginDB
	// Usage é o medidor que o host debita (ADR 0334 D6). nil devolve
	// ErrUsageUnavailable: o plugin não cobra por uma porta que não existe.
	Usage Usage
	// Core são as leituras do core declaradas em `capabilities.data`
	// (ADR 0227 D9). Chamada não declarada devolve ErrNotDeclared.
	Core CoreReader
	// Documents entrega documento ao cliente (`customer_documents`, ADR 0309 D6).
	Documents DocumentHost
	// Signer assina com o certificado da pessoa que age (`signing`, ADR 0317 D5).
	Signer Signer
	// Access é a portaria do core (`access`, ADR 0318 F5): só o que a extensão criou.
	Access AccessHost
	// Parcels são as encomendas da portaria para a pessoa do comprador (`parcels`, ADR 0318
	// F6): a lista e o passe de retirada.
	Parcels ParcelsHost
}

// DocumentHost entrega à área do cliente, pela regra dos documentos sensíveis.
type DocumentHost interface {
	Release(ctx context.Context, doc DocumentRelease) (ReleasedDocument, error)
	Revoke(ctx context.Context, customerID, documentID string, actor Actor, reason string) error
}

// DocumentRelease é o que a extensão entrega. O documento é sempre sensível.
type DocumentRelease struct {
	CustomerID string
	Actor      Actor
	Title      string
	Category   string
	Filename   string
	MIME       string
	Content    []byte
	// OriginRef liga o documento ao registo da extensão que o gerou.
	OriginRef string
	Notify    bool
}

// ReleasedDocument identifica o documento entregue.
type ReleasedDocument struct {
	ID     string
	FileID string
}

// Signer assina em nome da pessoa da chamada, e só dela.
type Signer interface {
	SignDetached(ctx context.Context, actor Actor, payload []byte) (DetachedSignature, error)
}

// DetachedSignature é o CMS destacado (.p7s) e o que identifica quem assinou.
type DetachedSignature struct {
	Signature       []byte
	CertSubject     string
	CertFingerprint string
	CertNotAfter    time.Time
	SignedAt        time.Time
}

// ErrSigningCertificateMissing diz que a pessoa não tem certificado válido
// gravado. A extensão recusa: nunca assina de forma mais fraca em silêncio.
var ErrSigningCertificateMissing = errors.New("commerceext: pessoa sem certificado de assinatura válido")

type refusedDocuments struct{}

func (refusedDocuments) Release(context.Context, DocumentRelease) (ReleasedDocument, error) {
	return ReleasedDocument{}, ErrNotDeclared
}

func (refusedDocuments) Revoke(context.Context, string, string, Actor, string) error {
	return ErrNotDeclared
}

type refusedSigner struct{}

func (refusedSigner) SignDetached(context.Context, Actor, []byte) (DetachedSignature, error) {
	return DetachedSignature{}, ErrNotDeclared
}

// DocumentsHost devolve o host, ou um que recusa com ErrNotDeclared.
func (r *Runtime) DocumentsHost() DocumentHost {
	if r == nil || r.Documents == nil {
		return refusedDocuments{}
	}
	return r.Documents
}

// SignerHost devolve o assinador, ou um que recusa com ErrNotDeclared.
func (r *Runtime) SignerHost() Signer {
	if r == nil || r.Signer == nil {
		return refusedSigner{}
	}
	return r.Signer
}

// Database devolve o acessor do schema do plugin, ou um que recusa tudo com
// ErrNotProvisioned. Evita o `if rt.DB != nil` espalhado por cada plugin — e
// um `nil` esquecido num caminho raro é um panic em produção.
func (r *Runtime) Database() PluginDB {
	if r == nil || r.DB == nil {
		return unprovisionedDB{}
	}
	return r.DB
}

// CoreReads devolve o leitor do core, ou um que recusa tudo com ErrNotDeclared.
func (r *Runtime) CoreReads() CoreReader {
	if r == nil || r.Core == nil {
		return undeclaredCore{}
	}
	return r.Core
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
