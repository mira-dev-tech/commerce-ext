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
type RPCCallArgs struct {
	HookID string
	Input  []byte // JSON encoding of the hook's input type
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
