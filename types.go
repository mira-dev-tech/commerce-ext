package commerceext

import "context"

// OrderView is a stable read model for hooks (no core/internal imports).
//
// JSON tags match the core order wire (snake_case) so the host↔plugin RPC bridge
// round-trips nested fields without a per-hook conversion layer: the host marshals
// the core order (snake_case json) and the plugin decodes it into OrderView.
type OrderView struct {
	ID           string         `json:"id"`
	TenantID     string         `json:"tenant_id"`
	Status       string         `json:"status"`
	TotalAmount  float32        `json:"total_amount"`
	CustomerName string         `json:"customer_name"`
	Metadata     map[string]any `json:"metadata"`
}

// CheckoutValidateInput is passed to checkout.validate handlers.
type CheckoutValidateInput struct {
	SessionID string
	Order     *OrderView
	RequestIP string
}

// RiskAssessInput is passed to checkout.risk_assess handlers.
type RiskAssessInput struct {
	Order         OrderView
	PaymentMethod string
	AttemptCount  int
	RequestIP     string
}

// PaymentRouteInput is passed to payment.route handlers.
type PaymentRouteInput struct {
	Order         OrderView
	PaymentMethod string
}

// PaymentRouteDecision includes gateway routing hints.
type PaymentRouteDecision struct {
	Outcome
	Require3DS bool
}

// PriceResolveInput is passed to catalog.price_resolve handlers.
type PriceResolveInput struct {
	SKU           string
	TenantID      string
	WarehouseID   string
	Channel       string
	UnitPrice     float32
	QuantityAvail int
}

// PriceResolveResult may adjust resolved price.
type PriceResolveResult struct {
	UnitPrice        float32
	PromotionApplied bool
	SubsidyAmount    float32
	Block            bool
	BlockCode        string
	BlockMessage     string
}

// InventoryAllocateInput is passed to inventory.allocate handlers.
type InventoryAllocateInput struct {
	TenantID    string
	WarehouseID string
	SKU         string
	Quantity    int
	OrderID     string
}

// IntegrationTransformInput wraps ERP payload bytes.
type IntegrationTransformInput struct {
	IntegrationID string
	Operation     string
	Payload       []byte
}

// IntegrationTransformResult is the transformed payload.
type IntegrationTransformResult struct {
	Payload []byte
	Outcome
}

// QuoteAdjustLine is a cart line for checkout.quote_adjust.
type QuoteAdjustLine struct {
	SKU        string
	Quantity   int
	UnitPrice  float32
	TotalPrice float32
}

// QuoteAdjustInput is passed to checkout.quote_adjust handlers.
type QuoteAdjustInput struct {
	TenantID    string
	WarehouseID string
	Subtotal    float32
	Items       []QuoteAdjustLine
}

// QuoteAdjustResult may adjust totals before order persist.
type QuoteAdjustResult struct {
	TotalAmount float32
	Discount    float32
	Outcome
}

// MemberEligibilityInput is passed to member.eligibility handlers.
type MemberEligibilityInput struct {
	TenantID string
	Action   string
	Order    OrderView
}

// ShippingQuoteLine is a preliminary shipping option for shipping.quote.
type ShippingQuoteLine struct {
	MethodID      string
	CarrierCode   string
	CarrierName   string
	MethodCode    string
	MethodName    string
	TransportMode string
	Price         float32
	Currency      string
	EtaDaysMin    *int
	EtaDaysMax    *int
	Provider      string
}

// ShippingQuoteInput is passed to shipping.quote handlers.
type ShippingQuoteInput struct {
	Request  map[string]any
	MemberID string
	Options  []ShippingQuoteLine
}

// ShippingQuoteResult may replace shipping options or block.
type ShippingQuoteResult struct {
	Options []ShippingQuoteLine
	Outcome
}

// TaxResolveLine is a tax line breakdown for tax.resolve.
type TaxResolveLine struct {
	SKU             string
	BaseAmount      float32
	IcmsAmount      *float32
	PisCofinsAmount *float32
	IssAmount       *float32
	IbsAmount       *float32
	CbsAmount       *float32
	IsAmount        *float32
	TotalTax        float32
	RuleID          string
	Provider        string
}

// TaxResolveInput is passed to tax.resolve handlers.
type TaxResolveInput struct {
	Request  map[string]any
	MemberID string
	Result   TaxResolveResultBody
}

// TaxResolveResultBody carries preliminary tax quote output.
type TaxResolveResultBody struct {
	ProfileID       string
	RuleID          string
	ReformPhase     string
	Lines           []TaxResolveLine
	IcmsAmount      *float32
	PisCofinsAmount *float32
	IssAmount       *float32
	IbsAmount       *float32
	CbsAmount       *float32
	IsAmount        *float32
	TotalTax        float32
}

// TaxResolveResult may replace tax breakdown or block.
type TaxResolveResult struct {
	Result TaxResolveResultBody
	Outcome
}

// WmsMovementValidateInput is passed to wms.movement.validate before ledger writes.
type WmsMovementValidateInput struct {
	TenantID     string
	WarehouseID  string
	ProductID    string
	MovementType string
	Quantity     int
	Reason       string
	DedupeKey    string
	RefType      string
	RefID        string
}

// WmsEanLookupInput is passed to wms.ean.lookup on cache miss.
type WmsEanLookupInput struct {
	TenantID    string
	EAN         string
	WarehouseID string
}

// WmsEanLookupResult may supply product data from an external catalogue.
type WmsEanLookupResult struct {
	Found       bool
	SKU         string
	Name        string
	EAN         string
	Category    string
	Description string
}

// WmsProductDraftEnrichInput is passed to wms.product_draft.enrich after capture.
type WmsProductDraftEnrichInput struct {
	TenantID    string
	DraftID     string
	EAN         string
	Title       string
	Description string
	Category    string
}

// WmsProductDraftEnrichResult carries LLM/vision normalisation output.
type WmsProductDraftEnrichResult struct {
	Title         string
	Description   string
	Category      string
	SkuSuggestion string
	CostPrice     float32
	MarginPct     float32
	SalePrice     float32
	LocationHint  string
	Ready         bool
	Outcome
}

// WmsInboundNfeResolveItem is one fiscal line from NF-e XML resolution.
type WmsInboundNfeResolveItem struct {
	LineNumber int
	EAN        string
	FiscalName string
	NCM        string
	Quantity   float32
	UnitCost   float32
}

// WmsInboundNfeResolveInput is passed to wms.inbound_nfe.resolve.
type WmsInboundNfeResolveInput struct {
	TenantID    string
	DraftID     string
	AccessKey   string
	WarehouseID string
	XmlSource   string
}

// WmsInboundNfeResolveResult carries parsed NF-e lines.
type WmsInboundNfeResolveResult struct {
	IssuerCNPJ string
	Items      []WmsInboundNfeResolveItem
	Ready      bool
	Outcome
}

// EventHandler processes bus events (must be idempotent).
type EventHandler func(ctx context.Context, event Event) error
