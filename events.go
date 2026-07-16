package commerceext

import "time"

// Core event types (stable v1).
const (
	EventOrderCreated                 = "order.created"
	EventOrderConfirmed               = "order.confirmed"
	EventOrderCancelled               = "order.cancelled"
	EventPaymentAuthorized            = "payment.authorized"
	EventPaymentCaptured              = "payment.captured"
	EventPaymentAttemptFailed         = "payment.attempt.failed"
	EventPaymentAttemptBlocked        = "payment.attempt.blocked"
	EventInventoryReserved            = "inventory.reserved"
	EventInventoryReleased            = "inventory.released"
	EventInventoryMovementRecorded    = "inventory.movement.recorded"
	EventWmsProductDraftReady         = "wms.product_draft.ready"
	EventWmsInboundNfeDraftReady      = "wms.inbound_nfe.draft_ready"
	EventWmsFulfillmentPickingStarted = "wms.fulfillment.picking_started"
	EventWmsFulfillmentDispatched     = "wms.fulfillment.dispatched"
)

// AllCoreEvents lists ship-first events for manifest validation.
var AllCoreEvents = []string{
	EventOrderCreated,
	EventOrderConfirmed,
	EventOrderCancelled,
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
