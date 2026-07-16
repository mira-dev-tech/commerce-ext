// Package plugintest é um mini Extension Host EM MEMÓRIA para autores de
// plugin testarem integração localmente, sem acesso ao core.
//
// Ele reproduz as semânticas do host real que importam para um plugin:
//
//   - Cadeia por prioridade (menor executa primeiro), inclusive entre plugins.
//   - Timeout de 300ms + panic-safe: handler que estoura vira Outcome
//     PLUGIN_FAILURE (fail-closed), igual ao core.
//   - checkout.quote_adjust: Discount ACUMULA; TotalAmount > 0 SOBRESCREVE o
//     total corrente; deny para a cadeia.
//   - integration.transform_*: payload atravessa os handlers em pipeline.
//   - Eventos: entrega síncrona em ordem de inscrição; PublishAtLeastOnce
//     entrega o MESMO fato duas vezes (IDs distintos) para provar idempotência
//     pela chave de negócio.
//
// Uso típico:
//
//	host := plugintest.NewHost()
//	if err := host.Install(ctx, meuplugin.New(), plugintest.WithConfig(cfg)); err != nil { ... }
//	out := host.QuoteAdjust(ctx, commerceext.QuoteAdjustInput{Subtotal: 500})
//
// O que ele NÃO substitui: o aceite em ambiente Mirá (dados reais, workflow,
// outbox durável). Ele fecha o gap do teste de integração local.
package plugintest

import (
	"context"
	"sort"
	"time"

	commerceext "github.com/mira-dev-tech/commerce-ext"
)

// CodePluginFailure espelha o código do host real para falha de handler
// (timeout/panic). Trate como contrato.
const CodePluginFailure = "PLUGIN_FAILURE"

// DefaultHookTimeout espelha o timeout síncrono do host real.
const DefaultHookTimeout = 300 * time.Millisecond

// Host é o mini extension host de teste.
type Host struct {
	timeout time.Duration
	hooks   map[string][]hookEntry
	events  []eventEntry
}

type hookEntry struct {
	pluginID string
	priority int
	seq      int
	fn       any
}

type eventEntry struct {
	pluginID string
	handler  commerceext.EventHandler
}

// Option configura a instalação de um plugin ou o host.
type Option func(*installOpts)

type installOpts struct {
	config  map[string]any
	secrets map[string]string
	logger  commerceext.Logger
}

// WithConfig injeta Runtime.Config no Init do plugin.
func WithConfig(cfg map[string]any) Option {
	return func(o *installOpts) { o.config = cfg }
}

// WithSecrets injeta Runtime.Secrets no Init do plugin.
func WithSecrets(s map[string]string) Option {
	return func(o *installOpts) { o.secrets = s }
}

// WithLogger injeta o logger do Runtime (padrão: NopLogger).
func WithLogger(l commerceext.Logger) Option {
	return func(o *installOpts) { o.logger = l }
}

// NewHost cria um host de teste vazio.
func NewHost() *Host {
	return &Host{timeout: DefaultHookTimeout, hooks: map[string][]hookEntry{}}
}

// SetTimeout troca o timeout de hook (testes de lentidão usam valores menores).
func (h *Host) SetTimeout(d time.Duration) {
	if d > 0 {
		h.timeout = d
	}
}

type hostPublisher struct{ h *Host }

func (p hostPublisher) Subscribe(eventType, pluginID string, handler commerceext.EventHandler) {
	if handler == nil || eventType == "" {
		return
	}
	p.h.events = append(p.h.events, eventEntry{pluginID: pluginID, handler: wrapType(eventType, handler)})
}

func wrapType(eventType string, handler commerceext.EventHandler) commerceext.EventHandler {
	return func(ctx context.Context, e commerceext.Event) error {
		if e.Type != eventType {
			return nil
		}
		return handler(ctx, e)
	}
}

// Install roda Init + Register do plugin neste host, como o core faria.
func (h *Host) Install(ctx context.Context, p commerceext.Plugin, opts ...Option) error {
	o := installOpts{logger: commerceext.NopLogger{}}
	for _, opt := range opts {
		opt(&o)
	}
	rt := &commerceext.Runtime{
		Config:  o.config,
		Secrets: o.secrets,
		Logger:  o.logger,
		Events:  hostPublisher{h: h},
	}
	if rt.Config == nil {
		rt.Config = map[string]any{}
	}
	if rt.Secrets == nil {
		rt.Secrets = map[string]string{}
	}
	if err := p.Init(ctx, rt); err != nil {
		return err
	}
	meta := p.Meta()
	reg := commerceext.NewRegistry(meta.ID)
	if err := p.Register(reg); err != nil {
		return err
	}
	for _, hk := range reg.Hooks() {
		entries := h.hooks[hk.HookID]
		h.hooks[hk.HookID] = append(entries, hookEntry{
			pluginID: meta.ID, priority: hk.Priority, seq: len(entries), fn: hk.Fn,
		})
		sortEntries(h.hooks[hk.HookID])
	}
	for _, ev := range reg.Events() {
		h.events = append(h.events, eventEntry{pluginID: meta.ID, handler: wrapType(ev.EventType, ev.Handler)})
	}
	return nil
}

func sortEntries(entries []hookEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].priority != entries[j].priority {
			return entries[i].priority < entries[j].priority
		}
		return entries[i].seq < entries[j].seq
	})
}

// runSafe reproduz o contorno do host real: timeout + recover de panic.
func (h *Host) runSafe(ctx context.Context, fn func(context.Context)) (completed bool) {
	runCtx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()
	done := make(chan bool, 1)
	go func() {
		ok := true
		defer func() {
			if recover() != nil {
				ok = false
			}
			done <- ok
		}()
		fn(runCtx)
	}()
	select {
	case ok := <-done:
		return ok
	case <-runCtx.Done():
		return false
	}
}

func failure(hookID string) commerceext.Outcome {
	return commerceext.Outcome{Allow: false, Code: CodePluginFailure, Message: "plugin failure during " + hookID}
}

// runOutcomeChain executa hooks Outcome em cadeia: primeiro deny vence;
// falha de handler degrada fail-closed com PLUGIN_FAILURE.
func runOutcomeChain[In any](h *Host, ctx context.Context, hookID string, in In) commerceext.Outcome {
	for _, e := range h.hooks[hookID] {
		fn, ok := e.fn.(func(context.Context, In) commerceext.Outcome)
		if !ok {
			continue
		}
		var out commerceext.Outcome
		if !h.runSafe(ctx, func(c context.Context) { out = fn(c, in) }) {
			return failure(hookID)
		}
		if !out.Allow {
			return out
		}
	}
	return commerceext.Allowed()
}

// CheckoutValidate roda a cadeia de checkout.validate.
func (h *Host) CheckoutValidate(ctx context.Context, in commerceext.CheckoutValidateInput) commerceext.Outcome {
	return runOutcomeChain(h, ctx, commerceext.HookCheckoutValidate, in)
}

// RiskAssess roda a cadeia de checkout.risk_assess.
func (h *Host) RiskAssess(ctx context.Context, in commerceext.RiskAssessInput) commerceext.Outcome {
	return runOutcomeChain(h, ctx, commerceext.HookCheckoutRiskAssess, in)
}

// OrderPreConfirm roda a cadeia de order.pre_confirm.
func (h *Host) OrderPreConfirm(ctx context.Context, order commerceext.OrderView) commerceext.Outcome {
	return runOutcomeChain(h, ctx, commerceext.HookOrderPreConfirm, order)
}

// OrderPreCancel roda a cadeia de order.pre_cancel.
func (h *Host) OrderPreCancel(ctx context.Context, order commerceext.OrderView) commerceext.Outcome {
	return runOutcomeChain(h, ctx, commerceext.HookOrderPreCancel, order)
}

// InventoryAllocate roda a cadeia de inventory.allocate.
func (h *Host) InventoryAllocate(ctx context.Context, in commerceext.InventoryAllocateInput) commerceext.Outcome {
	return runOutcomeChain(h, ctx, commerceext.HookInventoryAllocate, in)
}

// MemberEligibility roda a cadeia de member.eligibility.
func (h *Host) MemberEligibility(ctx context.Context, in commerceext.MemberEligibilityInput) commerceext.Outcome {
	return runOutcomeChain(h, ctx, commerceext.HookMemberEligibility, in)
}

// WmsMovementValidate roda a cadeia de wms.movement.validate.
func (h *Host) WmsMovementValidate(ctx context.Context, in commerceext.WmsMovementValidateInput) commerceext.Outcome {
	return runOutcomeChain(h, ctx, commerceext.HookWmsMovementValidate, in)
}

// QuoteAdjust roda checkout.quote_adjust com a semântica de cadeia do core:
// Discount acumula; TotalAmount > 0 sobrescreve; deny bloqueia.
func (h *Host) QuoteAdjust(ctx context.Context, in commerceext.QuoteAdjustInput) commerceext.QuoteAdjustResult {
	result := commerceext.QuoteAdjustResult{TotalAmount: in.Subtotal, Outcome: commerceext.Allowed()}
	for _, e := range h.hooks[commerceext.HookCheckoutQuoteAdjust] {
		fn, ok := e.fn.(func(context.Context, commerceext.QuoteAdjustInput) commerceext.QuoteAdjustResult)
		if !ok {
			continue
		}
		var out commerceext.QuoteAdjustResult
		if !h.runSafe(ctx, func(c context.Context) { out = fn(c, in) }) {
			return commerceext.QuoteAdjustResult{Outcome: failure(commerceext.HookCheckoutQuoteAdjust)}
		}
		if !out.Allow {
			if out.Code == "" {
				out.Code = "quote_blocked"
			}
			return out
		}
		if out.TotalAmount > 0 {
			result.TotalAmount = out.TotalAmount
		}
		result.Discount += out.Discount
	}
	return result
}

// TransformOutbound roda integration.transform_outbound em pipeline de payload.
func (h *Host) TransformOutbound(ctx context.Context, in commerceext.IntegrationTransformInput) commerceext.IntegrationTransformResult {
	return h.transform(ctx, commerceext.HookIntegrationTransformOut, in)
}

// TransformInbound roda integration.transform_inbound em pipeline de payload.
func (h *Host) TransformInbound(ctx context.Context, in commerceext.IntegrationTransformInput) commerceext.IntegrationTransformResult {
	return h.transform(ctx, commerceext.HookIntegrationTransformIn, in)
}

func (h *Host) transform(ctx context.Context, hookID string, in commerceext.IntegrationTransformInput) commerceext.IntegrationTransformResult {
	payload := in.Payload
	for _, e := range h.hooks[hookID] {
		fn, ok := e.fn.(func(context.Context, commerceext.IntegrationTransformInput) commerceext.IntegrationTransformResult)
		if !ok {
			continue
		}
		step := in
		step.Payload = payload
		var out commerceext.IntegrationTransformResult
		if !h.runSafe(ctx, func(c context.Context) { out = fn(c, step) }) {
			return commerceext.IntegrationTransformResult{Outcome: failure(hookID)}
		}
		if !out.Allow {
			return out
		}
		if out.Payload != nil {
			payload = out.Payload
		}
	}
	return commerceext.IntegrationTransformResult{Payload: payload, Outcome: commerceext.Allowed()}
}

// Publish entrega o evento SINCRONAMENTE a todos os inscritos, em ordem de
// inscrição, e devolve os erros retornados (no core eles seriam só logados).
func (h *Host) Publish(ctx context.Context, e commerceext.Event) []error {
	if e.ID == "" {
		e.ID = "evt-plugintest-1"
	}
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}
	var errs []error
	for _, sub := range h.events {
		if err := sub.handler(ctx, e); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// PublishAtLeastOnce entrega o MESMO fato duas vezes com IDs de evento
// DIFERENTES — o cenário real de dupla emissão. Um handler idempotente pela
// chave de negócio processa uma vez; um que deduplica por Event.ID processa
// duas (e o seu teste deve pegar isso).
func (h *Host) PublishAtLeastOnce(ctx context.Context, e commerceext.Event) []error {
	if e.ID == "" {
		e.ID = "evt-plugintest-1"
	}
	errs := h.Publish(ctx, e)
	dup := e
	dup.ID = e.ID + "-redelivery"
	return append(errs, h.Publish(ctx, dup)...)
}
