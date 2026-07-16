package commerceext

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/rpc"
	"os"
)

// Serve is the entrypoint a go-plugin binary calls from main(). It handles the
// two protocol invocations the core Extension Host uses:
//
//	plugin --commerce-ext-handshake          → prints "commerce-ext-ok" and exits
//	plugin --commerce-ext-rpc 127.0.0.1:PORT → serves net/rpc on PORT until killed
//
// Plugin authors implement the Plugin interface and import only this module —
// they never touch mira-commerce-core internals. Example:
//
//	func main() { commerceext.Serve(myplugin.New()) }
func Serve(p Plugin) {
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--commerce-ext-handshake":
			fmt.Println("commerce-ext-ok")
			return
		case "--commerce-ext-rpc":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "commerce-ext: --commerce-ext-rpc requires an address")
				os.Exit(2)
			}
			if err := serveRPC(p, args[i+1]); err != nil {
				fmt.Fprintf(os.Stderr, "commerce-ext: serve rpc: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}
	fmt.Fprintln(os.Stderr, "commerce-ext: no protocol flag (expected --commerce-ext-handshake or --commerce-ext-rpc)")
	os.Exit(2)
}

// serveRPC initialises the plugin, builds its registry and serves CommerceExt
// over net/rpc on addr. It blocks until the connection is closed (core kills the
// process on shutdown).
func serveRPC(p Plugin, addr string) error {
	ctx := context.Background()
	if err := p.Init(ctx, &Runtime{Logger: NopLogger{}}); err != nil {
		return fmt.Errorf("plugin init: %w", err)
	}
	reg := NewRegistry(p.Meta().ID)
	if err := p.Register(reg); err != nil {
		return fmt.Errorf("plugin register: %w", err)
	}

	svc := &rpcService{plugin: p, registry: reg}
	server := rpc.NewServer()
	if err := server.RegisterName(ServiceName, svc); err != nil {
		return fmt.Errorf("register rpc service: %w", err)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	defer ln.Close()
	server.Accept(ln) // blocks serving connections until the listener closes
	return nil
}

// rpcService is the net/rpc receiver the core Extension Host dials. Method
// signatures follow net/rpc conventions (exported, two args, error return).
type rpcService struct {
	plugin   Plugin
	registry *Registry
}

// Ping answers the host liveness probe.
func (s *rpcService) Ping(_ RPCPingArgs, reply *RPCPingReply) error {
	reply.Msg = "pong"
	return nil
}

// Hooks enumerates the hook IDs this plugin implements (host registers each).
func (s *rpcService) Hooks(_ RPCHooksArgs, reply *RPCHooksReply) error {
	seen := make(map[string]bool)
	for _, h := range s.registry.Hooks() {
		if seen[h.HookID] {
			continue
		}
		seen[h.HookID] = true
		reply.HookIDs = append(reply.HookIDs, h.HookID)
	}
	return nil
}

// Call dispatches one hook invocation to the registered handler.
func (s *rpcService) Call(args RPCCallArgs, reply *RPCCallReply) error {
	for _, h := range s.registry.Hooks() {
		if h.HookID != args.HookID {
			continue
		}
		return dispatchHook(h, args.Input, reply)
	}
	return fmt.Errorf("no handler for hook %q", args.HookID)
}

// dispatchHook decodes the JSON input for the hook's typed handler, invokes it
// and encodes the result into reply. Outcome-only hooks fill Allow/Code/Msg;
// result hooks JSON-encode the whole result into Output (matching the host-side
// RPC adapters in internal/ext/goplugin).
//
//nolint:cyclop // one case per hook type — exhaustive by design.
func dispatchHook(h HookRegistration, input []byte, reply *RPCCallReply) error {
	ctx := context.Background()
	switch fn := h.Fn.(type) {
	// ── Outcome-only hooks ──────────────────────────────────────────────
	case func(context.Context, CheckoutValidateInput) Outcome:
		var in CheckoutValidateInput
		if err := json.Unmarshal(input, &in); err != nil {
			return err
		}
		setOutcome(reply, fn(ctx, in))
	case func(context.Context, RiskAssessInput) Outcome:
		var in RiskAssessInput
		if err := json.Unmarshal(input, &in); err != nil {
			return err
		}
		setOutcome(reply, fn(ctx, in))
	case func(context.Context, OrderView) Outcome:
		var in OrderView
		if err := json.Unmarshal(input, &in); err != nil {
			return err
		}
		setOutcome(reply, fn(ctx, in))
	case func(context.Context, InventoryAllocateInput) Outcome:
		var in InventoryAllocateInput
		if err := json.Unmarshal(input, &in); err != nil {
			return err
		}
		setOutcome(reply, fn(ctx, in))
	case func(context.Context, MemberEligibilityInput) Outcome:
		var in MemberEligibilityInput
		if err := json.Unmarshal(input, &in); err != nil {
			return err
		}
		setOutcome(reply, fn(ctx, in))
	case func(context.Context, WmsMovementValidateInput) Outcome:
		var in WmsMovementValidateInput
		if err := json.Unmarshal(input, &in); err != nil {
			return err
		}
		setOutcome(reply, fn(ctx, in))

	// ── payment.route (Outcome + require3ds) ────────────────────────────
	case func(context.Context, PaymentRouteInput) PaymentRouteDecision:
		var in PaymentRouteInput
		if err := json.Unmarshal(input, &in); err != nil {
			return err
		}
		dec := fn(ctx, in)
		setOutcome(reply, dec.Outcome)
		out, err := json.Marshal(map[string]bool{"require3ds": dec.Require3DS})
		if err != nil {
			return err
		}
		reply.Output = out

	// ── Result hooks (whole result JSON-encoded into Output) ────────────
	case func(context.Context, PriceResolveInput) PriceResolveResult:
		return resultHook(input, reply, fn)
	case func(context.Context, QuoteAdjustInput) QuoteAdjustResult:
		return resultHook(input, reply, fn)
	case func(context.Context, ShippingQuoteInput) ShippingQuoteResult:
		return resultHook(input, reply, fn)
	case func(context.Context, TaxResolveInput) TaxResolveResult:
		return resultHook(input, reply, fn)
	case func(context.Context, IntegrationTransformInput) IntegrationTransformResult:
		return resultHook(input, reply, fn)
	case func(context.Context, WmsEanLookupInput) WmsEanLookupResult:
		return resultHook(input, reply, fn)
	case func(context.Context, WmsProductDraftEnrichInput) WmsProductDraftEnrichResult:
		return resultHook(input, reply, fn)
	case func(context.Context, WmsInboundNfeResolveInput) WmsInboundNfeResolveResult:
		return resultHook(input, reply, fn)

	default:
		return fmt.Errorf("hook %q has unsupported handler signature %T", h.HookID, h.Fn)
	}
	return nil
}

// resultHook decodes In, invokes fn and JSON-encodes Out into reply.Output.
func resultHook[In any, Out any](input []byte, reply *RPCCallReply, fn func(context.Context, In) Out) error {
	var in In
	if err := json.Unmarshal(input, &in); err != nil {
		return err
	}
	out, err := json.Marshal(fn(context.Background(), in))
	if err != nil {
		return err
	}
	reply.Output = out
	return nil
}

func setOutcome(reply *RPCCallReply, o Outcome) {
	reply.Allow = o.Allow
	reply.Code = o.Code
	reply.Msg = o.Message
}
