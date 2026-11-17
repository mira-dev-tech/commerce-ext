package commerceext

import (
	"context"
	"net/rpc"
	"os"
)

// EnvHost is the address of the partner host. Set only on the plugin
// process. Empty means this binary has no meter (local harness, in-process).
const EnvHost = "COMMERCE_EXT_HOST"

type callCtxKey struct{}

// WithCall attaches the host's call id so Usage.Add can name the charge.
func WithCall(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, callCtxKey{}, id)
}

// CallFrom returns the call id, or "" when the hook is in-process.
func CallFrom(ctx context.Context) string {
	id, _ := ctx.Value(callCtxKey{}).(string)
	return id
}

// DialUsage dials the partner host and returns a meter that reports ticks
// there. The plugin still does not choose the price.
func DialUsage(addr string) (Usage, error) {
	client, err := rpc.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return hostUsage{client: client}, nil
}

type hostUsage struct {
	client *rpc.Client
}

func (u hostUsage) Add(ctx context.Context, meter string, n int64, businessKey string) error {
	var reply HostAddReply
	err := u.client.Call(HostService+".Add", HostAddArgs{
		CallID: CallFrom(ctx), Meter: meter, N: n, BusinessKey: businessKey,
	}, &reply)
	if err != nil {
		return err
	}
	return codeToError(reply.Code)
}

func codeToError(code string) error {
	switch code {
	case "", "ok":
		return nil
	case "insufficient_balance":
		return ErrInsufficientBalance
	case "unknown_meter":
		return ErrUnknownMeter
	case "empty_key":
		return ErrEmptyBusinessKey
	default:
		return ErrUsageUnavailable
	}
}

// RefuseAPIEnv reports whether this process was given the API's database
// credentials. The partner binary and the partner host both refuse to start
// with them. The API process itself is not this check.
func RefuseAPIEnv() error {
	for _, key := range []string{"DATABASE_URL", "TENANT_DATABASES", "TENANT_MEMBERS"} {
		if os.Getenv(key) != "" {
			return errAPIEnv(key)
		}
	}
	return nil
}

type apiEnvError string

func errAPIEnv(key string) error { return apiEnvError(key) }

func (e apiEnvError) Error() string {
	return "commerce-ext: " + string(e) + " não entra no processo do plugin"
}
