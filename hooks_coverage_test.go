package commerceext

import (
	"context"
	"testing"
)

// registerAllHooks wires one handler per public hook through the Registry API a
// plugin author actually uses. A hook missing here cannot be registered by any
// plugin — which is the point of the guards below.
func registerAllHooks(reg *Registry) {
	reg.OnCheckoutValidate(0, func(context.Context, CheckoutValidateInput) Outcome { return Allowed() })
	reg.OnCheckoutRiskAssess(0, func(context.Context, RiskAssessInput) Outcome { return Allowed() })
	reg.OnCheckoutQuoteAdjust(0, func(context.Context, QuoteAdjustInput) QuoteAdjustResult {
		return QuoteAdjustResult{Outcome: Allowed()}
	})
	reg.OnOrderPreConfirm(0, func(context.Context, OrderView) Outcome { return Allowed() })
	reg.OnOrderPreCancel(0, func(context.Context, OrderView) Outcome { return Allowed() })
	reg.OnPaymentRoute(0, func(context.Context, PaymentRouteInput) PaymentRouteDecision {
		return PaymentRouteDecision{Outcome: Allowed()}
	})
	reg.OnPaymentInstallmentResolve(0, func(context.Context, PaymentInstallmentInput) PaymentInstallmentResult {
		return PaymentInstallmentResult{Outcome: Allowed()}
	})
	reg.OnCatalogPriceResolve(0, func(context.Context, PriceResolveInput) PriceResolveResult {
		return PriceResolveResult{}
	})
	reg.OnInventoryAllocate(0, func(context.Context, InventoryAllocateInput) Outcome { return Allowed() })
	reg.OnMemberEligibility(0, func(context.Context, MemberEligibilityInput) Outcome { return Allowed() })
	reg.OnIntegrationTransformOutbound(0, func(context.Context, IntegrationTransformInput) IntegrationTransformResult {
		return IntegrationTransformResult{Outcome: Allowed()}
	})
	reg.OnIntegrationTransformInbound(0, func(context.Context, IntegrationTransformInput) IntegrationTransformResult {
		return IntegrationTransformResult{Outcome: Allowed()}
	})
	reg.OnWmsMovementValidate(0, func(context.Context, WmsMovementValidateInput) Outcome { return Allowed() })
	reg.OnWmsEanLookup(0, func(context.Context, WmsEanLookupInput) WmsEanLookupResult {
		return WmsEanLookupResult{}
	})
	reg.OnWmsProductDraftEnrich(0, func(context.Context, WmsProductDraftEnrichInput) WmsProductDraftEnrichResult {
		return WmsProductDraftEnrichResult{Outcome: Allowed()}
	})
	reg.OnWmsInboundNfeResolve(0, func(context.Context, WmsInboundNfeResolveInput) WmsInboundNfeResolveResult {
		return WmsInboundNfeResolveResult{Outcome: Allowed()}
	})
	reg.OnShippingQuote(0, func(context.Context, ShippingQuoteInput) ShippingQuoteResult {
		return ShippingQuoteResult{Outcome: Allowed()}
	})
	reg.OnTaxResolve(0, func(context.Context, TaxResolveInput) TaxResolveResult {
		return TaxResolveResult{Outcome: Allowed()}
	})
}

// TestAllHooksHaveRegistryMethod guards against "ghost hooks": an ID that
// VerifyPluginManifest accepts (because it is in AllHooks) but that no plugin
// can actually register. payment.installment_resolve was exactly this.
func TestAllHooksHaveRegistryMethod(t *testing.T) {
	reg := NewRegistry("guard")
	registerAllHooks(reg)

	registered := make(map[string]bool)
	for _, h := range reg.Hooks() {
		registered[h.HookID] = true
	}
	for _, hookID := range AllHooks {
		if !registered[hookID] {
			t.Errorf("hook %q is in AllHooks (so VerifyPluginManifest accepts it in a manifest) "+
				"but no Registry.On* method registers it — either wire it or drop it from AllHooks", hookID)
		}
	}
}

// TestAllHooksDispatch guards the other half: a registered handler that Serve
// cannot dispatch fails at runtime, in the plugin subprocess, on a live request.
func TestAllHooksDispatch(t *testing.T) {
	reg := NewRegistry("guard")
	registerAllHooks(reg)

	for _, h := range reg.Hooks() {
		var reply RPCCallReply
		if err := dispatchHook(h, "", []byte(`{}`), &reply); err != nil {
			t.Errorf("dispatchHook(%s): %v — add a case in serve.go for this handler signature", h.HookID, err)
		}
	}
}
