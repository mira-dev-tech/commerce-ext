package commerceext

import (
	"context"
	"errors"
	"time"
)

// O que um plugin recebe para ler e escrever dados (ADR 0227 D3/D4/D9).
//
// Tudo aqui é INTERFACE, e o módulo `commerce-ext` continua sem driver de
// banco: se a assinatura arrastasse `pgx`, todo plugin passava a compilar
// contra o driver do core e a fronteira deixava de existir no tipo.
//
// O plugin nunca recebe DSN, credencial nem a possibilidade de DDL. O que ele
// tem é esta porta, e a porta já vem com o tenant e o schema dentro.

// ErrNotDeclared é devolvido quando o plugin pede uma leitura do core que não
// declarou em `capabilities.data`. Concede-se no manifesto, revisto em PR —
// nunca em runtime.
var ErrNotDeclared = errors.New("commerce-ext: leitura não declarada em capabilities.data")

// ErrNotProvisioned é devolvido quando o plugin usa `Runtime.DB` sem ter
// declarado `database.schema`. Pedir banco sem o declarar é erro de manifesto,
// não falha de infra — por isso é um erro nomeado e não um nil pointer.
var ErrNotProvisioned = errors.New("commerce-ext: plugin sem database.schema no manifesto")

// ErrNoRows é o Scan/QueryRow quando a consulta não devolve linha. O plugin
// compara com errors.Is — não importa o driver.
var ErrNoRows = errors.New("commerce-ext: nenhuma linha")

// ErrForbiddenStatement é devolvido quando a instrução sai do schema do plugin
// ou tenta DDL. Ver ADR 0227 D3.
var ErrForbiddenStatement = errors.New("commerce-ext: instrução fora do schema do plugin")

// Rows é o resultado de uma consulta, sem expor o driver.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

// Row é uma linha única (o `QueryRow` clássico).
type Row interface {
	Scan(dest ...any) error
}

// PluginDB é a porta para o schema PRÓPRIO do plugin.
//
// Duas coisas que o chamador não controla, e é isso que a torna segura:
//
//   - o `search_path` está fixo no schema do plugin, portanto uma tabela sem
//     qualificação nunca é a do core;
//   - o `tenant_id` é injectado pelo servidor a partir do contexto. O plugin
//     não o passa como argumento e não o consegue escolher — a mesma regra do
//     `x-mira-member-id` nos surfaces compartilhados.
//
// Por isso o SQL do plugin escreve as suas tabelas SEM a coluna `tenant_id` nos
// INSERT e SEM a comparar nos WHERE: quem a põe e quem a filtra é o acessor.
type PluginDB interface {
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) Row
	Exec(ctx context.Context, sql string, args ...any) (int64, error)
	// Schema devolve o schema deste plugin (útil em log e mensagem de erro).
	Schema() string
}

// CustomerView é o que um plugin vê de um cliente com `customers:read`
// declarado. Deliberadamente magro: nome e documento são PII, e um plugin que
// precise de mais pede ampliação por ADR, não por um campo novo em silêncio.
//
// `Metadata` vem porque é o sítio onde a extensão guarda o que é dela (#2867);
// `Anonymized` vem porque um plugin que decida sobre um cliente esquecido tem
// de poder parar em vez de tratar o registo redigido como dado real.
type CustomerView struct {
	ID         string         `json:"id"`
	Metadata   map[string]any `json:"metadata"`
	Anonymized bool           `json:"anonymized"`
}

// CoreReader são as leituras do CORE que o plugin declarou. Chamada não
// declarada devolve ErrNotDeclared — nunca um zero value, que o chamador leria
// como «cliente sem dados».
type CoreReader interface {
	Customer(ctx context.Context, customerID string) (CustomerView, error)
	// CustomerIdentity exige `customers:identity` em capabilities.data.
	CustomerIdentity(ctx context.Context, customerID string) (CustomerIdentity, error)
	// SearchCustomers exige `customers:identity` em capabilities.data.
	SearchCustomers(ctx context.Context, q string, limit int) ([]CustomerIdentity, error)
	// Appointments exige `appointments:read` em capabilities.data.
	Appointments(ctx context.Context, q AppointmentQuery) ([]Appointment, error)
}

// CustomerIdentity é a identidade da pessoa (ADR 0317 D6).
type CustomerIdentity struct {
	ID             string
	Name           string
	Email          string
	Phone          string
	DocumentNumber string
	Anonymized     bool
}

// AppointmentQuery recorta as marcações. From e To são obrigatórios.
type AppointmentQuery struct {
	From       time.Time
	To         time.Time
	CustomerID string
	ResourceID string
}

// Appointment é uma marcação da agenda, na vista que uma extensão precisa.
type Appointment struct {
	ID           string
	CustomerID   string
	CustomerName string
	Phone        string
	ResourceID   string
	ResourceName string
	Status       string
	ScheduledFor time.Time
	EndsAt       time.Time
}

// ── Acessores de recusa ───────────────────────────────────────────────────
//
// Devolvidos por Runtime.Database()/CoreReads() quando o plugin não declarou a
// capacidade. Recusam com o erro NOMEADO em vez de serem nil: um plugin que
// esqueça a verificação recebe um erro que diz o que falta no manifesto, não um
// panic num caminho que ninguém exercitou.

type unprovisionedDB struct{}

func (unprovisionedDB) Query(context.Context, string, ...any) (Rows, error) {
	return nil, ErrNotProvisioned
}

func (unprovisionedDB) QueryRow(context.Context, string, ...any) Row {
	return errRow{ErrNotProvisioned}
}

func (unprovisionedDB) Exec(context.Context, string, ...any) (int64, error) {
	return 0, ErrNotProvisioned
}

func (unprovisionedDB) Schema() string { return "" }

type undeclaredCore struct{}

func (undeclaredCore) CustomerIdentity(context.Context, string) (CustomerIdentity, error) {
	return CustomerIdentity{}, ErrNotDeclared
}

func (undeclaredCore) SearchCustomers(context.Context, string, int) ([]CustomerIdentity, error) {
	return nil, ErrNotDeclared
}

func (undeclaredCore) Appointments(context.Context, AppointmentQuery) ([]Appointment, error) {
	return nil, ErrNotDeclared
}

func (undeclaredCore) Customer(context.Context, string) (CustomerView, error) {
	return CustomerView{}, ErrNotDeclared
}

// errRow devolve sempre o mesmo erro no Scan — é a forma de um QueryRow falhar
// sem o chamador ter de distinguir «linha vazia» de «porta fechada».
type errRow struct{ err error }

func (r errRow) Scan(...any) error { return r.err }
