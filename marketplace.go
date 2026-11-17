package commerceext

import "context"

// Canonical requirement keys (ADR 0132) — UI labels via CanonicalRequirementLabel.
const (
	ReqKeyCategory           = "category"
	ReqKeyAttributes         = "attributes"
	ReqKeyBrand              = "brand"
	ReqKeyIdentity           = "identity"
	ReqKeyShipping           = "shipping"
	ReqKeyPricing            = "pricing"
	ReqKeyStock              = "stock"
	ReqKeyContentTitle       = "content.title"
	ReqKeyContentDescription = "content.description"
	ReqKeyContentMedia       = "content.media"
	ReqKeyPromotions         = "promotions"
	ReqKeyListingStatus      = "listing_status"
)

// ChannelRequirementSeverity is the obligation level for a canonical requirement.
type ChannelRequirementSeverity string

const (
	ChannelRequirementRequired    ChannelRequirementSeverity = "required"
	ChannelRequirementRecommended ChannelRequirementSeverity = "recommended"
	ChannelRequirementOptional    ChannelRequirementSeverity = "optional"
)

// ChannelRequirementSpec declares one marketplace requirement in Mira canonical space (ADR 0132).
// UI always shows CanonicalRequirementLabel(CanonicalKey); ChannelKey/ChannelLabel are tooltip/debug only.
type ChannelRequirementSpec struct {
	CanonicalKey string                     // e.g. "category", "attributes"
	Severity     ChannelRequirementSeverity // required | recommended | optional
	ChannelKey   string                     // portal jargão (ex. GTIN) — só tooltip
	ChannelLabel string                     // opcional, tooltip
}

// CanonicalRequirementLabel returns the stable PT UI label for a canonical key.
func CanonicalRequirementLabel(canonicalKey string) string {
	switch canonicalKey {
	case ReqKeyCategory:
		return "Categoria"
	case ReqKeyAttributes:
		return "Características"
	case ReqKeyBrand:
		return "Marca"
	case ReqKeyIdentity:
		return "Identificação"
	case ReqKeyShipping:
		return "Entrega"
	case ReqKeyPricing:
		return "Preço do canal"
	case ReqKeyStock:
		return "Estoque publicado"
	case ReqKeyContentTitle:
		return "Título do anúncio"
	case ReqKeyContentDescription:
		return "Descrição"
	case ReqKeyContentMedia:
		return "Imagens"
	case ReqKeyPromotions:
		return "Promoções"
	case ReqKeyListingStatus:
		return "Estado do anúncio"
	default:
		return canonicalKey
	}
}

// MinimalScaffoldRequirements is the default RequirementSpec list for scaffold channels.
func MinimalScaffoldRequirements() []ChannelRequirementSpec {
	return []ChannelRequirementSpec{
		{CanonicalKey: ReqKeyCategory, Severity: ChannelRequirementRequired, ChannelKey: "category_id"},
		{CanonicalKey: ReqKeyIdentity, Severity: ChannelRequirementRecommended, ChannelKey: "gtin"},
		{CanonicalKey: ReqKeyContentTitle, Severity: ChannelRequirementRequired, ChannelKey: "title"},
		{CanonicalKey: ReqKeyPricing, Severity: ChannelRequirementRequired, ChannelKey: "price"},
		{CanonicalKey: ReqKeyStock, Severity: ChannelRequirementRequired, ChannelKey: "stock"},
		{CanonicalKey: ReqKeyListingStatus, Severity: ChannelRequirementOptional, ChannelKey: "status"},
	}
}

// MercadoLivreRequirements declares ML obligation levels (ADR 0132).
func MercadoLivreRequirements() []ChannelRequirementSpec {
	return []ChannelRequirementSpec{
		{CanonicalKey: ReqKeyCategory, Severity: ChannelRequirementRequired, ChannelKey: "category_id", ChannelLabel: "Categoria ML"},
		{CanonicalKey: ReqKeyAttributes, Severity: ChannelRequirementRequired, ChannelKey: "attributes", ChannelLabel: "Atributos da categoria"},
		{CanonicalKey: ReqKeyBrand, Severity: ChannelRequirementRecommended, ChannelKey: "BRAND", ChannelLabel: "Marca"},
		{CanonicalKey: ReqKeyIdentity, Severity: ChannelRequirementRecommended, ChannelKey: "GTIN", ChannelLabel: "GTIN/EAN"},
		{CanonicalKey: ReqKeyShipping, Severity: ChannelRequirementRecommended, ChannelKey: "shipping.mode", ChannelLabel: "Modo de envio"},
		{CanonicalKey: ReqKeyPricing, Severity: ChannelRequirementRequired, ChannelKey: "price"},
		{CanonicalKey: ReqKeyStock, Severity: ChannelRequirementRequired, ChannelKey: "available_quantity"},
		{CanonicalKey: ReqKeyContentTitle, Severity: ChannelRequirementRequired, ChannelKey: "title"},
		{CanonicalKey: ReqKeyContentDescription, Severity: ChannelRequirementRecommended, ChannelKey: "description"},
		{CanonicalKey: ReqKeyContentMedia, Severity: ChannelRequirementRequired, ChannelKey: "pictures"},
		{CanonicalKey: ReqKeyPromotions, Severity: ChannelRequirementOptional, ChannelKey: "promotions"},
		{CanonicalKey: ReqKeyListingStatus, Severity: ChannelRequirementOptional, ChannelKey: "status"},
	}
}

// IFoodFOODRequirements declares iFood restaurant (FOOD) obligation levels (ADR 0132).
func IFoodFOODRequirements() []ChannelRequirementSpec {
	return []ChannelRequirementSpec{
		{CanonicalKey: ReqKeyCategory, Severity: ChannelRequirementRequired, ChannelKey: "category", ChannelLabel: "Grupo/categoria do catálogo"},
		{CanonicalKey: ReqKeyContentTitle, Severity: ChannelRequirementRequired, ChannelKey: "name", ChannelLabel: "Nome do item"},
		{CanonicalKey: ReqKeyContentDescription, Severity: ChannelRequirementRecommended, ChannelKey: "description"},
		{CanonicalKey: ReqKeyContentMedia, Severity: ChannelRequirementRecommended, ChannelKey: "image"},
		{CanonicalKey: ReqKeyPricing, Severity: ChannelRequirementRequired, ChannelKey: "price"},
		{CanonicalKey: ReqKeyStock, Severity: ChannelRequirementRecommended, ChannelKey: "availability", ChannelLabel: "Disponibilidade"},
		{CanonicalKey: ReqKeyIdentity, Severity: ChannelRequirementOptional, ChannelKey: "externalCode", ChannelLabel: "Código externo"},
		{CanonicalKey: ReqKeyListingStatus, Severity: ChannelRequirementOptional, ChannelKey: "status"},
	}
}

// MarketplaceMeta describes a marketplace adapter.
type MarketplaceMeta struct {
	ID           string // e.g. "mercado-livre"
	Version      string
	ChannelCode  string   // matches SalesChannel.Code
	Regions      []string // e.g. ["BR"]
	Capabilities []string // e.g. ["listings","orders","shipping","user_code_oauth"]
	// Requirements lists canonical obligation levels for readiness/UI (ADR 0132).
	// Empty = caller may fall back to MinimalScaffoldRequirements.
	Requirements []ChannelRequirementSpec
}

// RequirementSpecsForMeta returns declared requirements or the scaffold minimum.
func RequirementSpecsForMeta(meta MarketplaceMeta) []ChannelRequirementSpec {
	if len(meta.Requirements) > 0 {
		return meta.Requirements
	}
	return MinimalScaffoldRequirements()
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
	Price          float64 // sale / published price
	ListPrice      float64 // optional list (de/por); 0 = omit
	Currency       string
	Stock          int
	Images         []string
	Attributes     map[string]string
	// ShippingMethodMap is copied from connection config when adapters need it.
	ShippingMethodMap map[string]string
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

// MarketplaceListingUpsert — resultado por SKU de um SyncListings, para o core
// PERSISTIR o external_item_id (auditoria 2026-08-03, R2 do relatório de
// marketplace: sem persistência, cada tick re-criava TODOS os anúncios).
type MarketplaceListingUpsert struct {
	SKU            string
	ExternalItemID string
	Status         string // active | paused | error
}

// MarketplaceSyncListingsResult carries the outcome of a SyncListings call.
type MarketplaceSyncListingsResult struct {
	Created  int
	Updated  int
	Errors   []MarketplaceListingError
	Listings []MarketplaceListingUpsert
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
	ListPrice      float64 // optional; 0 = omit
	Currency       string  // e.g. "BRL"
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

// MarketplaceCategoryNode is one node in a marketplace category tree (ADR 0102).
type MarketplaceCategoryNode struct {
	ID       string
	Name     string
	Path     string
	ParentID string
	Leaf     bool
}

// MarketplaceAttributeDef describes a required/optional attribute for a category.
type MarketplaceAttributeDef struct {
	ID       string
	Name     string
	Required bool
	Values   []string // empty = free text / open enum
}

// MarketplaceTaxonomy is an optional capability (type-assert on MarketplaceAdapter).
// Adapters that expose category/attribute schemas implement this for AI mapping (ADR 0102).
type MarketplaceTaxonomy interface {
	FetchCategoryTree(ctx context.Context, tenantID string) ([]MarketplaceCategoryNode, error)
	FetchAttributeSchema(ctx context.Context, tenantID, externalCategoryID string) ([]MarketplaceAttributeDef, error)
}

// MarketplaceWebhookVerifier is an optional capability for HMAC/signature checks
// before the core enqueues the webhook payload (ADR 0103).
type MarketplaceWebhookVerifier interface {
	VerifyWebhook(ctx context.Context, tenantID string, headers map[string]string, payload []byte) error
}

// MarketplaceUserCodeChallenge is the distributed OAuth userCode payload (ADR 0132 / iFood).
type MarketplaceUserCodeChallenge struct {
	UserCode         string
	VerificationURL  string
	IntervalSeconds  int
	ExpiresInSeconds int
	State            string // opaque; pass back to OAuthComplete as state
}

// MarketplaceUserCodeOAuth is an optional capability for channels that use
// userCode (device) OAuth instead of browser redirect (capability: user_code_oauth).
type MarketplaceUserCodeOAuth interface {
	OAuthBeginUserCode(ctx context.Context, tenantID, connectionID string) (*MarketplaceUserCodeChallenge, error)
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
