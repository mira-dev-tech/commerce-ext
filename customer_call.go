package commerceext

import (
	"context"
	"errors"
)

// ErrCustomerActionNotRegistered é a acção declarada no manifesto que o plugin
// carregado não registou. O host responde 409: a instalação diz que a porta
// existe e o binário não a implementa.
var ErrCustomerActionNotRegistered = errors.New("commerceext: acção de comprador não registada")

// CustomerCall é uma chamada da sessão da compradora para uma acção declarada
// da extensão (ADR 0293).
//
// CustomerID é do SERVIDOR (a sessão mc_sess). O corpo não escolhe a pessoa.
// Body é o JSON cru; vazio em GET e DELETE.
type CustomerCall struct {
	Method     string
	Action     string
	CustomerID string
	Body       []byte
}

// CustomerResult é a resposta da extensão. Status 0 vale como 200.
// Body vazio com 204 não escreve corpo. Nos outros 2xx e 4xx o corpo é JSON
// que a extensão define — o core não lhe valida o esquema.
type CustomerResult struct {
	Status int
	Body   []byte
}

// CustomerActionFunc trata uma acção de comprador.
type CustomerActionFunc func(ctx context.Context, call CustomerCall) (CustomerResult, error)

// ErrStaffRecordNotRegistered é o manifesto a anunciar registros para a equipe
// sem o plugin ter registado o handler.
var ErrStaffRecordNotRegistered = errors.New("commerceext: registros da equipe não registados")

// Operações da porta da equipe (ADR 0294). O documento é da extensão; o admin
// só o desenha.
const (
	StaffOpList  = "list"
	StaffOpGet   = "get"
	StaffOpPatch = "patch"
	StaffOpFeed  = "feed"
)

// StaffRecordCall é uma chamada da equipe. CustomerID vem do caminho e o
// serviço já confirmou que a pessoa existe neste tenant. Body é o JSON do
// PATCH (`{"document":{...}}`); vazio em list, get e feed.
type StaffRecordCall struct {
	Op         string
	CustomerID string
	RecordID   string
	Body       []byte
	Limit      int
}

// StaffRecordFunc trata as quatro operações de registro da equipe.
type StaffRecordFunc func(ctx context.Context, call StaffRecordCall) (CustomerResult, error)

// ErrStaffActionNotRegistered é o manifesto a anunciar uma acção da equipe sem o
// plugin ter registado o handler.
var ErrStaffActionNotRegistered = errors.New("commerceext: acção da equipe não registada")

// Actor é a pessoa da equipe por trás de uma chamada (ADR 0317 D2). Vem do
// SERVIDOR, nunca do corpo: autoria e trilha valem por causa disso.
type Actor struct {
	StaffUserID string
	Name        string
}

// StaffActionCall é uma chamada da equipe a uma acção da extensão.
// Query são os parâmetros do URL; Body é o JSON cru, vazio em GET e DELETE.
type StaffActionCall struct {
	Method string
	Action string
	Actor  Actor
	Query  map[string]string
	Body   []byte
}

// StaffActionFunc trata uma acção da equipe.
type StaffActionFunc func(ctx context.Context, call StaffActionCall) (CustomerResult, error)
