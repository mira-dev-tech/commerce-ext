package commerceext

import "time"

// Core event types (stable v1).
const (
	EventOrderCreated                   = "order.created"
	EventOrderConfirmed                 = "order.confirmed"
	EventOrderCancelled                 = "order.cancelled"
	EventOrderChanged                   = "order.changed"
	EventPaymentAuthorized              = "payment.authorized"
	EventPaymentCaptured                = "payment.captured"
	EventPaymentAttemptFailed           = "payment.attempt.failed"
	EventPaymentAttemptBlocked          = "payment.attempt.blocked"
	EventInventoryReserved              = "inventory.reserved"
	EventInventoryReleased              = "inventory.released"
	EventInventoryMovementRecorded      = "inventory.movement.recorded"
	EventWmsProductDraftReady           = "wms.product_draft.ready"
	EventWmsInboundNfeDraftReady        = "wms.inbound_nfe.draft_ready"
	EventWmsFulfillmentPickingStarted   = "wms.fulfillment.picking_started"
	EventWmsFulfillmentDispatched       = "wms.fulfillment.dispatched"
	EventWmsFulfillmentOrderAmended     = "wms.fulfillment.order_amended"
	EventRentalHeld                     = "rental.held"
	EventDepositAuthorized              = "deposit.authorized"
	EventDepositCaptured                = "deposit.captured"
	EventMarketplaceConnectionActivated = "marketplace.connection.activated"
)

// Eventos do SAC e da Ouvidoria (ADR 0224, payload versão 1): ids, códigos e
// números — nunca texto do cliente nem contacto. O detalhe lê-se pela API
// (`GET /cases/{id}`) com a credencial do plugin.
const (
	EventCaseCreated       = "case.created"
	EventCaseUpdated       = "case.updated"
	EventCaseStatusChanged = "case.status_changed"
	EventCaseMessageAdded  = "case.message_added"
	EventCaseEscalated     = "case.escalated"
	EventCaseResolved      = "case.resolved"
	EventCaseSLABreaching  = "case.sla_breaching"
	EventCaseOfferAccepted = "case.offer_accepted"
)

// AllCoreEvents lists ship-first events for manifest validation.
var AllCoreEvents = []string{
	EventOrderCreated,
	EventOrderConfirmed,
	EventOrderCancelled,
	EventOrderChanged,
	EventPaymentAuthorized,
	EventPaymentCaptured,
	EventPaymentAttemptFailed,
	EventPaymentAttemptBlocked,
	EventInventoryReserved,
	EventInventoryReleased,
	EventInventoryMovementRecorded,
	EventWmsProductDraftReady,
	EventWmsInboundNfeDraftReady,
	EventWmsFulfillmentPickingStarted,
	EventWmsFulfillmentDispatched,
	EventWmsFulfillmentOrderAmended,
	EventRentalHeld,
	EventDepositAuthorized,
	EventDepositCaptured,
	EventMarketplaceConnectionActivated,
	EventCaseCreated,
	EventCaseUpdated,
	EventCaseStatusChanged,
	EventCaseMessageAdded,
	EventCaseEscalated,
	EventCaseResolved,
	EventCaseSLABreaching,
	EventCaseOfferAccepted,
}

// Event is a CloudEvents-like envelope (at-least-once delivery).
type Event struct {
	ID       string
	Type     string
	Source   string
	Time     time.Time
	TenantID string
	Data     map[string]any
}
