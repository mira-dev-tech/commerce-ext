package plugintest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	commerceext "github.com/mira-dev-tech/commerce-ext"
)

func TestMemoryDBIsolatesTenantAndRefusesCore(t *testing.T) {
	dir := t.TempDir()
	sql := `-- +goose Up
CREATE TABLE IF NOT EXISTS entries (
    tenant_id uuid NOT NULL DEFAULT current_setting('mira.tenant_id', true)::uuid,
    customer_id text NOT NULL,
    form_id text NOT NULL,
    document text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, customer_id, form_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS entries_uq ON entries (tenant_id, customer_id, form_id);
-- +goose Down
DROP TABLE IF EXISTS entries;
`
	if err := os.WriteFile(filepath.Join(dir, "001_entries.sql"), []byte(sql), 0o644); err != nil {
		t.Fatal(err)
	}
	db, err := NewDB("ext_forms", "tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(dir); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := db.Exec(ctx, `UPDATE core.customers SET name = 'x'`); !errors.Is(err, commerceext.ErrForbiddenStatement) {
		t.Fatalf("core: %v", err)
	}
	if _, err := db.Exec(ctx, `CREATE TABLE evil (id int)`); !errors.Is(err, commerceext.ErrForbiddenStatement) {
		t.Fatalf("ddl: %v", err)
	}
	n, err := db.Exec(ctx, `
		INSERT INTO entries (customer_id, form_id, document)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id, customer_id, form_id)
		DO UPDATE SET document = EXCLUDED.document, updated_at = now()`,
		"cust-1", "intake", "primeiro")
	if err != nil || n != 1 {
		t.Fatalf("insert: n=%d err=%v", n, err)
	}
	n, err = db.Exec(ctx, `
		INSERT INTO entries (customer_id, form_id, document)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id, customer_id, form_id)
		DO UPDATE SET document = EXCLUDED.document, updated_at = now()`,
		"cust-1", "intake", "segundo")
	if err != nil || n != 1 {
		t.Fatalf("upsert: n=%d err=%v", n, err)
	}
	var got string
	if err := db.QueryRow(ctx, `SELECT document FROM entries WHERE customer_id = $1 AND form_id = $2`, "cust-1", "intake").Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != "segundo" {
		t.Fatalf("documento = %q", got)
	}
	other := db.WithTenant("tenant-b")
	err = other.QueryRow(ctx, `SELECT document FROM entries WHERE customer_id = $1 AND form_id = $2`, "cust-1", "intake").Scan(&got)
	if !errors.Is(err, commerceext.ErrNoRows) {
		t.Fatalf("outro tenant viu a linha: %v %q", err, got)
	}
	empty := db.WithTenant("")
	if _, err := empty.Exec(ctx, `INSERT INTO entries (customer_id, form_id, document) VALUES ($1, $2, $3)`, "c", "f", "x"); !errors.Is(err, ErrTenantMissing) {
		t.Fatalf("sem tenant: %v", err)
	}
}

func TestWalletDebitsOnceAndRefusesEmpty(t *testing.T) {
	w, err := NewWallet("tenant-a", "forms", "1.0.0", 30, map[string]int64{"lookup": 10})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := w.Require(10); err != nil {
		t.Fatal(err)
	}
	if err := w.Add(ctx, "lookup", 2, "order-1"); err != nil {
		t.Fatal(err)
	}
	if err := w.Add(ctx, "lookup", 2, "order-1"); err != nil {
		t.Fatal(err)
	}
	if w.Balance() != 10 {
		t.Fatalf("saldo = %d, queria 10", w.Balance())
	}
	if len(w.Charges()) != 1 || w.Charges()[0].Millicents != 20 {
		t.Fatalf("lançamentos = %+v", w.Charges())
	}
	if err := w.Add(ctx, "lookup", 2, "order-2"); !errors.Is(err, commerceext.ErrInsufficientBalance) {
		t.Fatalf("sem saldo: %v", err)
	}
	if len(w.Charges()) != 1 {
		t.Fatalf("recusa gravou lançamento: %+v", w.Charges())
	}
	if err := w.Add(ctx, "outro", 1, "order-3"); !errors.Is(err, commerceext.ErrUnknownMeter) {
		t.Fatalf("medidor: %v", err)
	}
	if err := w.Add(ctx, "lookup", 1, ""); !errors.Is(err, commerceext.ErrEmptyBusinessKey) {
		t.Fatalf("chave: %v", err)
	}
	if err := w.Require(11); !errors.Is(err, commerceext.ErrInsufficientBalance) {
		t.Fatalf("require: %v", err)
	}
}

func TestInstallWiresDBAndUsage(t *testing.T) {
	db, err := NewDB("ext_forms", "tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	w, err := NewWallet("tenant-a", "p", "1", 100, map[string]int64{"lookup": 5})
	if err != nil {
		t.Fatal(err)
	}
	var sawDB commerceext.PluginDB
	var sawUsage commerceext.Usage
	p := &probePlugin{onInit: func(rt *commerceext.Runtime) {
		sawDB = rt.Database()
		sawUsage = rt.Meters()
	}}
	h := NewHost()
	if err := h.Install(context.Background(), p, WithDB(db), WithUsage(w)); err != nil {
		t.Fatal(err)
	}
	if sawDB == nil || sawUsage == nil {
		t.Fatal("runtime sem portas")
	}
	if err := sawUsage.Add(context.Background(), "lookup", 1, "k"); err != nil {
		t.Fatal(err)
	}
	if w.Balance() != 95 {
		t.Fatalf("saldo = %d", w.Balance())
	}
}

type probePlugin struct {
	onInit func(*commerceext.Runtime)
}

func (p *probePlugin) Meta() commerceext.Meta {
	return commerceext.Meta{ID: "probe", Version: "1", CompatibleCore: "^0.2.0"}
}
func (p *probePlugin) Init(_ context.Context, rt *commerceext.Runtime) error {
	p.onInit(rt)
	return nil
}
func (p *probePlugin) Register(*commerceext.Registry) error { return nil }
func (p *probePlugin) Shutdown(context.Context) error       { return nil }
