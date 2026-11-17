package commerceext

import (
	"context"
	"errors"
)

// Erros do medidor. O plugin compara com errors.Is. O preço e o saldo são do
// host: o plugin só diz que o medidor disparou (ADR 0334 D6).
var (
	// ErrInsufficientBalance recusa a chamada quando o saldo não cobre o
	// débito. Nada é lançado.
	ErrInsufficientBalance = errors.New("commerce-ext: saldo insuficiente")
	// ErrUnknownMeter recusa um medidor que a release aceite não declara.
	ErrUnknownMeter = errors.New("commerce-ext: medidor não aceite")
	// ErrUsageUnavailable é o host sem medidor ligado. Pedir cobrança aí é
	// erro de manifesto, não um débito de zero.
	ErrUsageUnavailable = errors.New("commerce-ext: medidor não ligado neste host")
	// ErrEmptyBusinessKey recusa um disparo sem chave de negócio. Sem ela, um
	// evento entregue duas vezes cobrava duas vezes.
	ErrEmptyBusinessKey = errors.New("commerce-ext: chave de negócio vazia")
)

// Usage é como o plugin reporta um medidor. O host debita a wallet da empresa.
// O plugin não escolhe o preço, o tenant nem o saldo.
type Usage interface {
	Add(ctx context.Context, meter string, n int64, businessKey string) error
}

type refusedUsage struct{}

func (refusedUsage) Add(context.Context, string, int64, string) error {
	return ErrUsageUnavailable
}

// Meters devolve o medidor do host, ou um que recusa com ErrUsageUnavailable.
func (r *Runtime) Meters() Usage {
	if r == nil || r.Usage == nil {
		return refusedUsage{}
	}
	return r.Usage
}
