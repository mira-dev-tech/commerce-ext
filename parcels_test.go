package commerceext

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// A área das encomendas é de uma pessoa que a extensão criou pela portaria: o manifesto que pede
// `parcels` sem `access` é recusado no PR, não no primeiro pedido da compradora.
func TestVerifyPluginManifestParcelsNeedsAccess(t *testing.T) {
	m := PluginManifest{ID: "condominio", Version: "1.0.0"}
	m.Capabilities.Parcels = true
	if err := VerifyPluginManifest(m, CoreLine); err == nil || !strings.Contains(err.Error(), "capabilities.access") {
		t.Fatalf("parcels sem access = %v", err)
	}
	m.Capabilities.Access = true
	if err := VerifyPluginManifest(m, CoreLine); err != nil {
		t.Fatal(err)
	}
}

// Sem a capacidade declarada, o balcão recusa tudo com ErrNotDeclared: a extensão diz o que falta
// no manifesto em vez de estourar.
func TestParcelDeskUndeclared(t *testing.T) {
	for _, rt := range []*Runtime{nil, {}} {
		desk := rt.ParcelDesk()
		if _, err := desk.Mine(context.Background(), "c"); !errors.Is(err, ErrNotDeclared) {
			t.Fatalf("Mine sem declarar = %v", err)
		}
		if _, err := desk.Pass(context.Background(), "c"); !errors.Is(err, ErrNotDeclared) {
			t.Fatalf("Pass sem declarar = %v", err)
		}
	}
}
