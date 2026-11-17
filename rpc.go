package commerceext

// RPCArgs and RPCReply types for net/rpc over Unix/TCP socket.
// Plugin binaries implement RPCService; core calls via net/rpc.Client.

// RPCPingArgs / RPCPingReply — liveness check.
type RPCPingArgs struct{ Msg string }
type RPCPingReply struct{ Msg string }

// RPCHooksArgs / RPCHooksReply — enumerate which hooks this plugin implements.
type RPCHooksArgs struct{}
type RPCHooksReply struct {
	HookIDs []string // subset of AllHooks
}

// RPCCallArgs carries one hook invocation (JSON-encoded input).
// CallID is set by the partner host. The plugin sends it back on Usage.Add
// so a charge belongs to this call and a failed call can be refunded.
type RPCCallArgs struct {
	HookID string
	CallID string
	Input  []byte // JSON encoding of the hook's input type
}

// HostService is the net/rpc name of the partner host (ADR 0334 D3).
// The plugin calls Add. The API calls Dispatch and Probe.
const HostService = "PluginHost"

// HostAddArgs is one meter tick. The plugin does not send a price.
type HostAddArgs struct {
	CallID      string
	Meter       string
	N           int64
	BusinessKey string
}

// HostAddReply.Code is empty on success. Otherwise it is one of the
// insufficient_balance, unknown_meter, empty_key, invalid, suspended,
// not_active, unavailable codes. net/rpc does not carry typed errors.
type HostAddReply struct {
	Code string
}

// HostDispatchArgs is one hook call from the API. Token is the host secret.
type HostDispatchArgs struct {
	Token    string
	MemberID string
	HookID   string
	Input    []byte
}

// HostProbeArgs asks whether this process's database role respects RLS.
type HostProbeArgs struct {
	Token string
}

// HostProbeReply.RoleOK is true only when the probe ran and RLS applies.
type HostProbeReply struct {
	RoleOK bool
}

// RPCCallReply carries the hook's output (JSON-encoded result + optional outcome).
type RPCCallReply struct {
	Output []byte // JSON encoding of the hook's result type
	Allow  bool
	Code   string
	Msg    string
}

// ServiceName is the net/rpc service name exposed by plugin binaries.
const ServiceName = "CommerceExt"
