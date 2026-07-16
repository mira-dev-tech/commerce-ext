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
}
