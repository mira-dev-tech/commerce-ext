package plugintest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	commerceext "github.com/mira-dev-tech/commerce-ext"
	"github.com/mira-dev-tech/commerce-ext/plugintest"
)

// fakePlugin cobre as superfícies do mini-host nos testes.
type fakePlugin struct {
	id        string
	prio      int
	cap       float32
	discount  float32
	processed map[string]struct{}
	points    int
	panics    bool
	sleep     time.Duration
}

func (p *fakePlugin) Meta() commerceext.Meta {
	return commerceext.Meta{ID: p.id, Version: "1.0.0", CompatibleCore: "^0.2.0"}
}
func (p *fakePlugin) Init(context.Context, *commerceext.Runtime) error {
	p.processed = map[string]struct{}{}
	return nil
}
func (p *fakePlugin) Shutdown(context.Context) error { return nil }

func (p *fakePlugin) Register(reg *commerceext.Registry) error {
	reg.OnCheckoutQuoteAdjust(p.prio, func(_ context.Context, in commerceext.QuoteAdjustInput) commerceext.QuoteAdjustResult {
		if p.panics {
			panic("boom")
		}
		if p.sleep > 0 {
			time.Sleep(p.sleep)
		}
		if p.discount <= 0 {
			return commerceext.QuoteAdjustResult{Outcome: commerceext.Allowed()}
		}
		d := in.Subtotal * p.discount / 100
		return commerceext.QuoteAdjustResult{TotalAmount: in.Subtotal - d, Discount: d, Outcome: commerceext.Allowed()}
	})
	reg.OnOrderPreConfirm(p.prio, func(_ context.Context, o commerceext.OrderView) commerceext.Outcome {
		if p.cap > 0 && o.TotalAmount > p.cap {
			return commerceext.Denied("CAP", "acima do teto")
		}
		return commerceext.Allowed()
	})
	reg.OnEvent(commerceext.EventOrderConfirmed, func(_ context.Context, e commerceext.Event) error {
		orderID, _ := e.Data["order_id"].(string)
		if orderID == "" {
			return errors.New("sem order_id")
		}
		if _, done := p.processed[orderID]; done {
			return nil
		}
		p.processed[orderID] = struct{}{}
		p.points++
		return nil
	})
	return nil
}

func TestQuoteAdjustChainSemantics(t *testing.T) {
	host := plugintest.NewHost()
	ctx := context.Background()
	// Dois plugins no mesmo hook: descontos ACUMULAM na cadeia.
	if err := host.Install(ctx, &fakePlugin{id: "a", prio: 100, discount: 5}); err != nil {
		t.Fatal(err)
	}
	if err := host.Install(ctx, &fakePlugin{id: "b", prio: 200, discount: 10}); err != nil {
		t.Fatal(err)
	}
	out := host.QuoteAdjust(ctx, commerceext.QuoteAdjustInput{Subtotal: 100})
	if !out.Allow || out.Discount != 15 {
		t.Fatalf("expected discount acumulado 15, got %+v", out)
	}
	// TotalAmount do último que ajustou sobrescreve (b devolve 90).
	if out.TotalAmount != 90 {
		t.Fatalf("expected total 90 (override do último), got %+v", out)
	}
}

func TestPreConfirmDenyStopsChain(t *testing.T) {
	host := plugintest.NewHost()
	ctx := context.Background()
	_ = host.Install(ctx, &fakePlugin{id: "a", prio: 100, cap: 50})
	out := host.OrderPreConfirm(ctx, commerceext.OrderView{TotalAmount: 100})
	if out.Allow || out.Code != "CAP" {
		t.Fatalf("expected CAP deny, got %+v", out)
	}
}

func TestPanicBecomesPluginFailure(t *testing.T) {
	host := plugintest.NewHost()
	ctx := context.Background()
	_ = host.Install(ctx, &fakePlugin{id: "a", prio: 100, panics: true})
	out := host.QuoteAdjust(ctx, commerceext.QuoteAdjustInput{Subtotal: 100})
	if out.Allow || out.Code != plugintest.CodePluginFailure {
		t.Fatalf("expected PLUGIN_FAILURE em panic, got %+v", out)
	}
}

func TestTimeoutBecomesPluginFailure(t *testing.T) {
	host := plugintest.NewHost()
	host.SetTimeout(20 * time.Millisecond)
	ctx := context.Background()
	_ = host.Install(ctx, &fakePlugin{id: "a", prio: 100, discount: 5, sleep: 200 * time.Millisecond})
	out := host.QuoteAdjust(ctx, commerceext.QuoteAdjustInput{Subtotal: 100})
	if out.Allow || out.Code != plugintest.CodePluginFailure {
		t.Fatalf("expected PLUGIN_FAILURE em timeout, got %+v", out)
	}
}

func TestPublishAtLeastOnceExposesBadDedupe(t *testing.T) {
	host := plugintest.NewHost()
	ctx := context.Background()
	p := &fakePlugin{id: "a", prio: 100}
	_ = host.Install(ctx, p)
	errs := host.PublishAtLeastOnce(ctx, commerceext.Event{
		Type: commerceext.EventOrderConfirmed,
		Data: map[string]any{"order_id": "o1"},
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected handler errors: %v", errs)
	}
	// Handler idempotente por order_id credita UMA vez mesmo com 2 entregas.
	if p.points != 1 {
		t.Fatalf("expected 1 credit (dedupe por chave de negócio), got %d", p.points)
	}
}

func TestEventTypeFiltering(t *testing.T) {
	host := plugintest.NewHost()
	ctx := context.Background()
	p := &fakePlugin{id: "a", prio: 100}
	_ = host.Install(ctx, p)
	// Evento de outro tipo não aciona o handler de order.confirmed.
	host.Publish(ctx, commerceext.Event{Type: commerceext.EventOrderCancelled, Data: map[string]any{"order_id": "x"}})
	if p.points != 0 {
		t.Fatalf("handler não deveria ter rodado para outro tipo, points=%d", p.points)
	}
}

type routePlugin struct {
	id      string
	prio    int
	teto3ds float32
	blocked string
	freeGte float32
}

func (p *routePlugin) Meta() commerceext.Meta {
	return commerceext.Meta{ID: p.id, Version: "1.0.0", CompatibleCore: "^0.2.0"}
}
func (p *routePlugin) Init(context.Context, *commerceext.Runtime) error { return nil }
func (p *routePlugin) Shutdown(context.Context) error                   { return nil }

func (p *routePlugin) Register(reg *commerceext.Registry) error {
	reg.OnPaymentRoute(p.prio, func(_ context.Context, in commerceext.PaymentRouteInput) commerceext.PaymentRouteDecision {
		if p.blocked != "" && in.PaymentMethod == p.blocked {
			return commerceext.PaymentRouteDecision{Outcome: commerceext.Denied("METHOD_OFF", "método indisponível")}
		}
		return commerceext.PaymentRouteDecision{
			Outcome:    commerceext.Allowed(),
			Require3DS: p.teto3ds > 0 && in.Order.TotalAmount > p.teto3ds,
		}
	})
	reg.OnShippingQuote(p.prio, func(_ context.Context, in commerceext.ShippingQuoteInput) commerceext.ShippingQuoteResult {
		if p.freeGte <= 0 {
			return commerceext.ShippingQuoteResult{Outcome: commerceext.Allowed()}
		}
		out := make([]commerceext.ShippingQuoteLine, len(in.Options))
		copy(out, in.Options)
		for i := range out {
			out[i].Price = 0
		}
		return commerceext.ShippingQuoteResult{Options: out, Outcome: commerceext.Allowed()}
	})
	return nil
}

func TestPaymentRouteRequire3DSAccumulates(t *testing.T) {
	host := plugintest.NewHost()
	ctx := context.Background()
	_ = host.Install(ctx, &routePlugin{id: "a", prio: 100, teto3ds: 1000})
	_ = host.Install(ctx, &routePlugin{id: "b", prio: 200})
	dec := host.PaymentRoute(ctx, commerceext.PaymentRouteInput{
		Order: commerceext.OrderView{TotalAmount: 1500}, PaymentMethod: "card",
	})
	if !dec.Allow || !dec.Require3DS {
		t.Fatalf("expected allow com Require3DS acumulado, got %+v", dec)
	}
	dec = host.PaymentRoute(ctx, commerceext.PaymentRouteInput{
		Order: commerceext.OrderView{TotalAmount: 500}, PaymentMethod: "card",
	})
	if dec.Require3DS {
		t.Fatalf("abaixo do teto não exige 3DS, got %+v", dec)
	}
}

func TestPaymentRouteDenyStops(t *testing.T) {
	host := plugintest.NewHost()
	ctx := context.Background()
	_ = host.Install(ctx, &routePlugin{id: "a", prio: 100, blocked: "boleto"})
	dec := host.PaymentRoute(ctx, commerceext.PaymentRouteInput{PaymentMethod: "boleto"})
	if dec.Allow || dec.Code != "METHOD_OFF" {
		t.Fatalf("expected METHOD_OFF deny, got %+v", dec)
	}
}

func TestShippingQuoteLastAdjusterWins(t *testing.T) {
	host := plugintest.NewHost()
	ctx := context.Background()
	_ = host.Install(ctx, &routePlugin{id: "a", prio: 100, freeGte: 1})
	in := commerceext.ShippingQuoteInput{Options: []commerceext.ShippingQuoteLine{
		{MethodID: "m1", CarrierCode: "sedex", Price: 39.9},
	}}
	out := host.ShippingQuote(ctx, in)
	if !out.Allow || len(out.Options) != 1 || out.Options[0].Price != 0 {
		t.Fatalf("expected frete zerado pelo plugin, got %+v", out)
	}
	if in.Options[0].Price != 39.9 {
		t.Fatalf("input original não pode ser mutado, got %+v", in.Options[0])
	}
}
