package commerceext

import "context"

// IntegrationMeta describes an ERP/integration adapter.
type IntegrationMeta struct {
	ID      string
	Version string
	Kind    string // e.g. commerce.mira.dev/integration/erp/v1
}

// SyncInventoryRequest is passed to IntegrationAdapter.SyncInventory.
type SyncInventoryRequest struct {
	TenantID    string
	WarehouseID string
	Since       string
}

// InventoryRow is a single stock line fetched from the ERP for the core to apply.
type InventoryRow struct {
	SKU         string
	Quantity    int
	WarehouseID string
}

// PriceRow is a single price line fetched from the ERP for the core to apply.
type PriceRow struct {
	SKU   string
	Price float32
}

// SyncInventoryResult carries the stock rows pulled from the ERP. The adapter
// performs the I/O; the core owns persistence into its catalog.
type SyncInventoryResult struct {
	UpdatedSKUs int
	Items       []InventoryRow
	Errors      []string
}

// SyncPricesRequest is passed to IntegrationAdapter.SyncPrices.
type SyncPricesRequest struct {
	TenantID string
	Since    string
}

// SyncPricesResult carries the price rows pulled from the ERP.
type SyncPricesResult struct {
	UpdatedSKUs int
	Prices      []PriceRow
	Errors      []string
}

// OrderOutboundView is the ERP-safe order projection.
type OrderOutboundView struct {
	ID        string
	TenantID  string
	Status    string
	Total     float32
	LineItems []OrderLineView
}

// OrderLineView is a single order line for ERP push.
type OrderLineView struct {
	SKU       string
	Quantity  int
	UnitPrice float32
}

// OrderStatusUpdate is pushed to ERP on status change.
type OrderStatusUpdate struct {
	OrderID string
	Status  string
}

// PushAck acknowledges order push to ERP.
type PushAck struct {
	ExternalID string
}

// IntegrationAdapter is the ERP integration protocol (ARCH Peça 1).
type IntegrationAdapter interface {
	Meta() IntegrationMeta
	Health(ctx context.Context) error
	SyncInventory(ctx context.Context, req SyncInventoryRequest) (*SyncInventoryResult, error)
	SyncPrices(ctx context.Context, req SyncPricesRequest) (*SyncPricesResult, error)
	PushOrder(ctx context.Context, order OrderOutboundView) (*PushAck, error)
	PushOrderStatus(ctx context.Context, update OrderStatusUpdate) error
}
