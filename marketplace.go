package commerceext

import "context"

// MarketplaceMeta describes a marketplace adapter.
type MarketplaceMeta struct {
	ID           string // e.g. "mercado-livre"
	Version      string
	ChannelCode  string   // matches SalesChannel.Code
	Regions      []string // e.g. ["BR"]
	Capabilities []string // e.g. ["listings","orders","shipping"]
}

// MarketplaceSyncProduct is a pre-fetched snapshot of a product for marketplace sync.
// The core assembles this from the catalog, pricing, and inventory stores so the
// adapter does not need to reach back into the host.
type MarketplaceSyncProduct struct {
	SKU            string
	ExternalItemID string // empty if not yet listed on this marketplace
	Title          string
	Description    string
	Category       string
	EAN            string
	Price          float64
	Currency       string
	Stock          int
	Images         []string
	Attributes     map[string]string
}

// MarketplaceSyncListingsRequest is passed to MarketplaceAdapter.SyncListings.
type MarketplaceSyncListingsRequest struct {
	TenantID  string
	ChannelID string
	SKUFilter []string // empty = all active products
	Products  []MarketplaceSyncProduct
}

// MarketplaceListingError reports a per-SKU error during SyncListings.
type MarketplaceListingError struct {
	SKU     string
	Message string
}

// MarketplaceSyncListingsResult carries the outcome of a SyncListings call.
type MarketplaceSyncListingsResult struct {
	Created int
	Updated int
	Errors  []MarketplaceListingError
}

// MarketplaceUpdateStockRequest pushes updated stock to a marketplace listing.
type MarketplaceUpdateStockRequest struct {
	TenantID       string
	ExternalItemID string
	SKU            string
	Quantity       int
}

// MarketplaceUpdatePriceRequest pushes an updated price to a marketplace listing.
type MarketplaceUpdatePriceRequest struct {
	TenantID       string
	ExternalItemID string
	SKU            string
	Price          float64
	Currency       string // e.g. "BRL"
}

// MarketplaceAddress is a simplified shipping / buyer address.
type MarketplaceAddress struct {
	Street     string
	City       string
	State      string
	PostalCode string
	Country    string
}

// MarketplaceLineItem is a single line in an order received from a marketplace.
type MarketplaceLineItem struct {
	ExternalItemID string
	SKU            string
	Title          string
	Quantity       int
	UnitPrice      float64
	Currency       string
}

// MarketplaceOrder is the marketplace-side view of an order.
type MarketplaceOrder struct {
	ExternalID         string
	ExternalShipmentID string // marketplace shipment/logistic ID (used for AckOrderShipped)
	SellerID           string
	BuyerName          string
	BuyerEmail         string
	LineItems          []MarketplaceLineItem
	TotalAmount        float64
	Currency           string
	PaymentStatus      string
	ShippingAddress    MarketplaceAddress
}

// MarketplaceNotification is a single notification entry (thin payload from marketplace).
type MarketplaceNotification struct {
	Topic      string
	ResourceID string
	Raw        []byte
}

// MarketplaceNotificationAck acknowledges processing of a notification.
type MarketplaceNotificationAck struct {
	Topic      string
	ResourceID string
	Processed  bool
}

// MarketplaceAdapter is the marketplace channel protocol.
// Implementations live in extensions/{marketplace}/ and are registered via Registry.
type MarketplaceAdapter interface {
	Meta() MarketplaceMeta

	// Health verifies connectivity to the marketplace API.
	Health(ctx context.Context) error

	// ValidateCredentials checks that the stored credentials for tenantID are valid.
	ValidateCredentials(ctx context.Context, tenantID string) error

	// SyncListings pushes the tenant's active catalog as marketplace listings.
	SyncListings(ctx context.Context, req MarketplaceSyncListingsRequest) (*MarketplaceSyncListingsResult, error)

	// PauseListings pauses marketplace listings for the given SKUs.
	PauseListings(ctx context.Context, tenantID string, skus []string) error

	// ResumeListings reactivates paused marketplace listings for the given SKUs.
	ResumeListings(ctx context.Context, tenantID string, skus []string) error

	// UpdateStock pushes a stock quantity update to a marketplace listing.
	UpdateStock(ctx context.Context, req MarketplaceUpdateStockRequest) error

	// UpdatePrice pushes a price update to a marketplace listing.
	UpdatePrice(ctx context.Context, req MarketplaceUpdatePriceRequest) error

	// FetchOrder retrieves a full order from the marketplace by its external ID.
	FetchOrder(ctx context.Context, tenantID, externalOrderID string) (*MarketplaceOrder, error)

	// AckOrderShipped notifies the marketplace that an order has shipped.
	AckOrderShipped(ctx context.Context, tenantID, externalOrderID, trackingCode string) error

	// HandleNotification processes an inbound webhook notification payload.
	// Must be fast: the HTTP handler already replied 200 before calling this.
	HandleNotification(ctx context.Context, tenantID string, payload []byte) (*MarketplaceNotificationAck, error)

	// FetchMissedNotifications retrieves notifications the marketplace marked as
	// lost (exceeded retry window) so the core can replay them.
	FetchMissedNotifications(ctx context.Context, tenantID string) ([]MarketplaceNotification, error)

	// OAuthBeginURL returns the marketplace authorization URL for the tenant.
	// The state parameter (anti-CSRF) is generated internally and stored for later validation.
	// connectionID is embedded in the state so the callback can recover which connection to activate.
	OAuthBeginURL(ctx context.Context, tenantID, connectionID string) (authorizeURL string, err error)

	// OAuthComplete handles the marketplace authorization callback.
	// It validates the state, exchanges the code, stores the tokens, and returns the seller ID.
	OAuthComplete(ctx context.Context, tenantID, code, state string) (sellerID, connectionID string, err error)
}
