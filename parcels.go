package commerceext

import (
	"context"
	"errors"
	"time"
)

// As encomendas da portaria vistas por uma extensão (ADR 0318 F6, `capabilities.parcels`).
//
// O vocabulário aqui é o do CORE — encomenda, destinatário (uma pessoa ou um grupo da
// portaria), local de guarda, retirada. O da vertical (a unidade, o morador) fica na extensão,
// que traduz o grupo para o nome dela.
//
// O host serve a área de uma PESSOA DA PORTARIA SEM COLABORADOR: a ligada ao comprador da
// sessão (`CustomerCall.CustomerID`), criada por esta extensão pelo host `access`. Três
// garantias são do HOST, nunca da extensão:
//
//   - **a pessoa é a do comprador, e é desta extensão.** O host acha a pessoa pelo comprador e
//     recusa a de outro dono (a tela da portaria, outra extensão) como «não encontrada», sem
//     dizer de quem é. A pessoa ligada a um colaborador retira pelo passe dele, no admin;
//   - **o passe não se guarda.** O QR e o código são derivados na hora de uma chave do cofre do
//     tenant e valem 10 e 5 minutos. A extensão não os grava, não os loga, e o core responde à
//     compradora com `Cache-Control: no-store`;
//   - **a retirada continua na portaria**, pela prova conferida de novo na hora (o QR lido pela
//     câmara, ou o código digitado, com o limite de erros por hora). Nada aqui retira.

var (
	// ErrParcelsUnavailable — as encomendas não estão prontas neste processo (sem banco, sem a
	// chave do cofre, ou o serviço ainda não chegou a este deploy).
	ErrParcelsUnavailable = errors.New("commerceext: encomendas indisponíveis")
	// ErrParcelsDisabled — a empresa não liga o recebimento de encomendas na portaria.
	ErrParcelsDisabled = errors.New("commerceext: recebimento de encomendas desligado")
	// ErrParcelsNoPass — a pessoa do comprador não tem passe agora: suspensa, removida ou sem
	// pessoa ativa desta extensão.
	ErrParcelsNoPass = errors.New("commerceext: sem passe de retirada")
)

// ParcelsHost são as encomendas da portaria para uma extensão. Todas as chamadas são do tenant
// do pedido; nenhuma recebe o tenant como argumento.
type ParcelsHost interface {
	// Mine são as encomendas da pessoa ligada ao comprador e dos grupos dela: as que estão na
	// portaria e as últimas que saíram. Com o recebimento desligado, Enabled = false e as listas
	// vêm vazias (não é erro).
	Mine(ctx context.Context, customerID string) (ParcelsMine, error)
	// Pass é o QR e o código de retirada da pessoa ligada ao comprador, agora.
	Pass(ctx context.Context, customerID string) (ParcelPass, error)
}

// ParcelsMine é a área da pessoa.
type ParcelsMine struct {
	Enabled bool
	Held    []Parcel
	Recent  []Parcel
}

// Tipos de destinatário que a área vê: a própria pessoa ou um grupo dela.
const (
	ParcelRecipientPerson = "person"
	ParcelRecipientGroup  = "group"
)

// Estados da encomenda.
const (
	ParcelHeld     = "held"      // na portaria
	ParcelPickedUp = "picked_up" // retirada
	ParcelReturned = "returned"  // devolvida ao remetente
)

// ParcelRecipient é para quem a encomenda veio. Label é o nome gravado na chegada.
type ParcelRecipient struct {
	Kind  string
	ID    string
	Label string
}

// Parcel é uma encomenda, sem fotos nem o nome lido da etiqueta.
type Parcel struct {
	ID string
	// NumberLabel é o número curto da empresa («#0042»), o que a portaria procura na prateleira.
	NumberLabel  string
	Status       string
	TrackingCode string
	// CarrierLabel é o nome da transportadora; vazio = não identificada.
	CarrierLabel string
	Sender       string
	// Kind: envelope, package, large_box, other.
	Kind         string
	Perishable   bool
	Refrigerated bool
	Fragile      bool
	Damaged      bool
	Recipient    ParcelRecipient
	LocationName string
	LocationSlot string
	ReceivedAt   time.Time
	// HoldUntil é o prazo de guarda; Overdue = na portaria com o prazo vencido.
	HoldUntil  time.Time
	Overdue    bool
	PickedUpAt *time.Time
	// PickedUpBy é quem retirou, como ficou no histórico (a própria pessoa, o terceiro
	// autorizado ou o nome que a portaria escreveu).
	PickedUpBy string
	ReturnedAt *time.Time
}

// ParcelPass é o passe de retirada: o QR (o texto que a câmara da portaria lê), o código de 6
// dígitos, quando o QR vence e quando a tela deve pedir outro.
type ParcelPass struct {
	QR        string
	Code      string
	ExpiresAt time.Time
	RefreshAt time.Time
}

// refusedParcels recusa tudo com ErrNotDeclared: a extensão que não declarou
// `capabilities.parcels` recebe o erro que diz o que falta no manifesto.
type refusedParcels struct{}

func (refusedParcels) Mine(context.Context, string) (ParcelsMine, error) {
	return ParcelsMine{}, ErrNotDeclared
}
func (refusedParcels) Pass(context.Context, string) (ParcelPass, error) {
	return ParcelPass{}, ErrNotDeclared
}

// ParcelDesk devolve as encomendas da portaria, ou um balcão que recusa tudo com
// ErrNotDeclared.
func (r *Runtime) ParcelDesk() ParcelsHost {
	if r == nil || r.Parcels == nil {
		return refusedParcels{}
	}
	return r.Parcels
}
