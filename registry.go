package commerceext

import "context"

type hookRegistration struct {
	HookID   string
	Priority int
	Fn       any
}

type eventRegistration struct {
	EventType string
	Handler   EventHandler
}

// HookRegistration describes a registered hook handler.
type HookRegistration = hookRegistration

// EventRegistration describes a registered event handler.
type EventRegistration = eventRegistration

// Registry collects hook, event and integration-adapter registrations from a plugin.
type Registry struct {
	pluginID            string
	hooks               []hookRegistration
	events              []eventRegistration
	adapters            []IntegrationAdapter
	marketplaceAdapters []MarketplaceAdapter
}

// NewRegistry creates a registry scoped to pluginID.
func NewRegistry(pluginID string) *Registry {
	return &Registry{pluginID: pluginID}
}

// PluginID returns the owning plugin identifier.
func (r *Registry) PluginID() string {
	if r == nil {
		return ""
	}
	return r.pluginID
}

// Hooks returns registered hooks (for bridge into core host).
func (r *Registry) Hooks() []HookRegistration {
	if r == nil {
		return nil
	}
	return append([]HookRegistration(nil), r.hooks...)
}

// Events returns registered event subscriptions.
func (r *Registry) Events() []EventRegistration {
	if r == nil {
		return nil
	}
	return append([]EventRegistration(nil), r.events...)
}

// RegisterIntegrationAdapter registers an ERP integration adapter (sync/push).
func (r *Registry) RegisterIntegrationAdapter(adapter IntegrationAdapter) {
	if r == nil || adapter == nil {
		return
	}
	r.adapters = append(r.adapters, adapter)
}

// Adapters returns registered integration adapters (for bridge into core host).
func (r *Registry) Adapters() []IntegrationAdapter {
	if r == nil {
		return nil
	}
	return append([]IntegrationAdapter(nil), r.adapters...)
}

// RegisterMarketplaceAdapter registers a marketplace channel adapter.
func (r *Registry) RegisterMarketplaceAdapter(adapter MarketplaceAdapter) {
	if r == nil || adapter == nil {
		return
	}
	r.marketplaceAdapters = append(r.marketplaceAdapters, adapter)
}

// MarketplaceAdapters returns registered marketplace adapters (for bridge into core host).
func (r *Registry) MarketplaceAdapters() []MarketplaceAdapter {
	if r == nil {
		return nil
	}
	return append([]MarketplaceAdapter(nil), r.marketplaceAdapters...)
}

// OnCheckoutValidate registers checkout.validate (priority: lower runs first).
func (r *Registry) OnCheckoutValidate(priority int, fn func(context.Context, CheckoutValidateInput) Outcome) {
	r.register(HookCheckoutValidate, priority, fn)
}

// OnCheckoutRiskAssess registers checkout.risk_assess.
func (r *Registry) OnCheckoutRiskAssess(priority int, fn func(context.Context, RiskAssessInput) Outcome) {
	r.register(HookCheckoutRiskAssess, priority, fn)
}

// OnOrderPreConfirm registers order.pre_confirm.
func (r *Registry) OnOrderPreConfirm(priority int, fn func(context.Context, OrderView) Outcome) {
	r.register(HookOrderPreConfirm, priority, fn)
}

// OnOrderPreCancel registers order.pre_cancel.
func (r *Registry) OnOrderPreCancel(priority int, fn func(context.Context, OrderView) Outcome) {
	r.register(HookOrderPreCancel, priority, fn)
}

// OnPaymentRoute registers payment.route.
func (r *Registry) OnPaymentRoute(priority int, fn func(context.Context, PaymentRouteInput) PaymentRouteDecision) {
	r.register(HookPaymentRoute, priority, fn)
}

// OnCatalogPriceResolve registers catalog.price_resolve.
func (r *Registry) OnCatalogPriceResolve(priority int, fn func(context.Context, PriceResolveInput) PriceResolveResult) {
	r.register(HookCatalogPriceResolve, priority, fn)
}

// OnInventoryAllocate registers inventory.allocate.
func (r *Registry) OnInventoryAllocate(priority int, fn func(context.Context, InventoryAllocateInput) Outcome) {
	r.register(HookInventoryAllocate, priority, fn)
}

// OnCheckoutQuoteAdjust registers checkout.quote_adjust.
func (r *Registry) OnCheckoutQuoteAdjust(priority int, fn func(context.Context, QuoteAdjustInput) QuoteAdjustResult) {
	r.register(HookCheckoutQuoteAdjust, priority, fn)
}

// OnMemberEligibility registers member.eligibility.
func (r *Registry) OnMemberEligibility(priority int, fn func(context.Context, MemberEligibilityInput) Outcome) {
	r.register(HookMemberEligibility, priority, fn)
}

// OnIntegrationTransformOutbound registers integration.transform_outbound.
func (r *Registry) OnIntegrationTransformOutbound(priority int, fn func(context.Context, IntegrationTransformInput) IntegrationTransformResult) {
	r.register(HookIntegrationTransformOut, priority, fn)
}

// OnIntegrationTransformInbound registers integration.transform_inbound.
func (r *Registry) OnIntegrationTransformInbound(priority int, fn func(context.Context, IntegrationTransformInput) IntegrationTransformResult) {
	r.register(HookIntegrationTransformIn, priority, fn)
}

// OnWmsMovementValidate registers wms.movement.validate.
func (r *Registry) OnWmsMovementValidate(priority int, fn func(context.Context, WmsMovementValidateInput) Outcome) {
	r.register(HookWmsMovementValidate, priority, fn)
}

// OnWmsEanLookup registers wms.ean.lookup.
func (r *Registry) OnWmsEanLookup(priority int, fn func(context.Context, WmsEanLookupInput) WmsEanLookupResult) {
	r.register(HookWmsEanLookup, priority, fn)
}

// OnWmsProductDraftEnrich registers wms.product_draft.enrich.
func (r *Registry) OnWmsProductDraftEnrich(priority int, fn func(context.Context, WmsProductDraftEnrichInput) WmsProductDraftEnrichResult) {
	r.register(HookWmsProductDraftEnrich, priority, fn)
}

// OnWmsInboundNfeResolve registers wms.inbound_nfe.resolve.
func (r *Registry) OnWmsInboundNfeResolve(priority int, fn func(context.Context, WmsInboundNfeResolveInput) WmsInboundNfeResolveResult) {
	r.register(HookWmsInboundNfeResolve, priority, fn)
}

// OnShippingQuote registers shipping.quote.
func (r *Registry) OnShippingQuote(priority int, fn func(context.Context, ShippingQuoteInput) ShippingQuoteResult) {
	r.register(HookShippingQuote, priority, fn)
}

// OnTaxResolve registers tax.resolve.
func (r *Registry) OnTaxResolve(priority int, fn func(context.Context, TaxResolveInput) TaxResolveResult) {
	r.register(HookTaxResolve, priority, fn)
}

// OnEvent registers an async event handler (idempotent required).
func (r *Registry) OnEvent(eventType string, handler EventHandler) {
	if r == nil || handler == nil {
		return
	}
	r.events = append(r.events, eventRegistration{EventType: eventType, Handler: handler})
}

func (r *Registry) register(hookID string, priority int, fn any) {
	if r == nil || fn == nil {
		return
	}
	r.hooks = append(r.hooks, hookRegistration{HookID: hookID, Priority: priority, Fn: fn})
}
