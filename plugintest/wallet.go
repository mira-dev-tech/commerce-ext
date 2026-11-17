package plugintest

import (
	"context"
	"fmt"
	"strings"
	"sync"

	commerceext "github.com/mira-dev-tech/commerce-ext"
)

// Charge é um débito que a wallet falsa registou.
type Charge struct {
	Meter      string
	N          int64
	Millicents int64
	Key        string
}

// Wallet é o saldo da empresa no teste (ADR 0334 D6). O preço vem do mapa
// aceite no construtor. O plugin não o escreve.
type Wallet struct {
	mu       sync.Mutex
	tenant   string
	pluginID string
	version  string
	balance  int64
	prices   map[string]int64
	charges  []Charge
	seen     map[string]struct{}
}

// NewWallet cria a wallet com o saldo e os preços aceites, em millicents.
// tenant, plugin e versão entram na chave de idempotência, como o host fará.
func NewWallet(tenant, pluginID, version string, balance int64, prices map[string]int64) (*Wallet, error) {
	if strings.TrimSpace(tenant) == "" || strings.TrimSpace(pluginID) == "" || strings.TrimSpace(version) == "" {
		return nil, fmt.Errorf("plugintest: wallet sem tenant, plugin ou versão")
	}
	if balance < 0 {
		return nil, fmt.Errorf("plugintest: saldo inicial negativo")
	}
	copied := make(map[string]int64, len(prices))
	for k, v := range prices {
		if strings.TrimSpace(k) == "" || v < 0 {
			return nil, fmt.Errorf("plugintest: preço inválido %q", k)
		}
		copied[k] = v
	}
	return &Wallet{
		tenant: tenant, pluginID: pluginID, version: version,
		balance: balance, prices: copied, seen: map[string]struct{}{},
	}, nil
}

// Balance é o disponível, em millicents.
func (w *Wallet) Balance() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.balance
}

// Charges devolve uma cópia dos débitos, por ordem.
func (w *Wallet) Charges() []Charge {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]Charge, len(w.charges))
	copy(out, w.charges)
	return out
}

// Require é o que o host faz antes de correr um hook pago. Sem saldo, o hook
// não corre. Não debita.
func (w *Wallet) Require(millicents int64) error {
	if millicents < 0 {
		return fmt.Errorf("plugintest: valor a exigir negativo")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.balance < millicents {
		return commerceext.ErrInsufficientBalance
	}
	return nil
}

// Add implementa commerceext.Usage. Debita preço×n. A mesma chave de negócio
// não debita outra vez. Sem saldo, não grava lançamento.
func (w *Wallet) Add(_ context.Context, meter string, n int64, businessKey string) error {
	meter = strings.TrimSpace(meter)
	businessKey = strings.TrimSpace(businessKey)
	if meter == "" {
		return commerceext.ErrUnknownMeter
	}
	if businessKey == "" {
		return commerceext.ErrEmptyBusinessKey
	}
	if n <= 0 {
		return fmt.Errorf("plugintest: medidor %q com quantidade %d", meter, n)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	price, ok := w.prices[meter]
	if !ok {
		return commerceext.ErrUnknownMeter
	}
	key := w.tenant + "\x00" + w.pluginID + "\x00" + w.version + "\x00" + meter + "\x00" + businessKey
	if _, done := w.seen[key]; done {
		return nil
	}
	if price > 0 && n > (1<<63-1)/price {
		return fmt.Errorf("plugintest: débito de %q transborda", meter)
	}
	cost := price * n
	if w.balance < cost {
		return commerceext.ErrInsufficientBalance
	}
	w.balance -= cost
	w.seen[key] = struct{}{}
	w.charges = append(w.charges, Charge{Meter: meter, N: n, Millicents: cost, Key: businessKey})
	return nil
}
