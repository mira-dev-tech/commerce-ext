package commerceext

// Hook identifiers (ARCH Peça 1 — catálogo v1).
const (
	HookCheckoutValidate          = "checkout.validate"
	HookCheckoutQuoteAdjust       = "checkout.quote_adjust"
	HookCheckoutRiskAssess        = "checkout.risk_assess"
	HookOrderPreConfirm           = "order.pre_confirm"
	HookOrderPreCancel            = "order.pre_cancel"
	HookPaymentRoute              = "payment.route"
	HookPaymentInstallmentResolve = "payment.installment_resolve"
	HookCatalogPriceResolve       = "catalog.price_resolve"
	HookInventoryAllocate         = "inventory.allocate"
	HookMemberEligibility         = "member.eligibility"
	HookIntegrationTransformOut   = "integration.transform_outbound"
	HookIntegrationTransformIn    = "integration.transform_inbound"
	HookWmsEanLookup              = "wms.ean.lookup"
	HookWmsInboundNfeResolve      = "wms.inbound_nfe.resolve"
	HookWmsProductDraftEnrich     = "wms.product_draft.enrich"
	HookWmsMovementValidate       = "wms.movement.validate"
	HookShippingQuote             = "shipping.quote"
	HookTaxResolve                = "tax.resolve"
	HookRentalQuoteAdjust         = "rental.quote_adjust"
	HookRentalAvailability        = "rental.availability"
)

// AllHooks lists ship-first hook IDs for manifest validation.
var AllHooks = []string{
	HookCheckoutValidate,
	HookCheckoutQuoteAdjust,
	HookCheckoutRiskAssess,
	HookOrderPreConfirm,
	HookOrderPreCancel,
	HookPaymentRoute,
	HookPaymentInstallmentResolve,
	HookCatalogPriceResolve,
	HookInventoryAllocate,
	HookMemberEligibility,
	HookIntegrationTransformOut,
	HookIntegrationTransformIn,
	HookWmsEanLookup,
	HookWmsInboundNfeResolve,
	HookWmsProductDraftEnrich,
	HookWmsMovementValidate,
	HookShippingQuote,
	HookTaxResolve,
	// HookRentalQuoteAdjust e HookRentalAvailability estão FORA desta lista de
	// propósito: os IDs existem e estão reservados (ADR 0049), mas não há método
	// Registry.On* nem dispatch no core — aceitá-los num manifesto seria vender
	// um hook que nenhum plugin consegue registar (o "ghost hook" que
	// TestAllHooksHaveRegistryMethod guarda). Entram aqui no mesmo PR que os
	// wire, não antes.
}
