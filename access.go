package commerceext

import (
	"context"
	"errors"
	"time"
)

// A portaria do core vista por uma extensão (ADR 0318 F5, `capabilities.access`).
//
// O vocabulário aqui é o do CORE — local, porta, pessoa, grupo, regra, credencial, visita.
// O da vertical (bloco, unidade, morador, aluno, sócio) fica nas tabelas da extensão, que
// guarda só os ids que o host devolve. Outros usos do mesmo host: academia (sócio com
// cartão e horário), escola (responsável que busca o aluno), coworking (membro por plano),
// hotel (hóspede pela estadia), clínica (paciente da consulta).
//
// Três garantias são do HOST, nunca da extensão:
//
//   - **A extensão só mexe no que ela criou.** Pessoa e grupo nascem com o dono
//     (`ext:<id>`), e qualquer escrita sobre outro registo — o colaborador, a pessoa que a
//     tela da portaria cadastrou, o grupo de outra extensão — recusa com ErrAccessNotManaged.
//     Uma regra ou um cartão só se criam para uma pessoa ou grupo dela, e uma visita só tem
//     anfitrião dela.
//   - **Quem é a pessoa vem do servidor.** A ligação a um comprador (`CustomerID`) é a da
//     sessão `mc_sess` que a extensão recebeu em CustomerCall, e a pessoa ligada a um
//     colaborador nunca é da extensão.
//   - **Abrir passa pela regra e pelos interruptores da portaria**, que nascem desligados
//     (abrir de longe), e o histórico da porta guarda quem pediu e por onde.

var (
	// ErrAccessNotManaged — o registo existe, mas não é desta extensão. O host não diz de
	// quem é.
	ErrAccessNotManaged = errors.New("commerceext: registo da portaria que não é desta extensão")
	// ErrAccessNotFound — o id não é de nenhum registo deste tenant.
	ErrAccessNotFound = errors.New("commerceext: registo da portaria não encontrado")
	// ErrAccessUnavailable — a portaria não está pronta neste processo (sem banco, sem a
	// chave do cofre, ou a abertura por comando ainda não chegou a este deploy).
	ErrAccessUnavailable = errors.New("commerceext: portaria indisponível")
)

// AccessError é a recusa de uma escrita que a extensão pode mostrar a quem pediu: Code é
// curto e estável, Message é a frase em pt-BR.
type AccessError struct {
	Code    string
	Message string
}

func (e *AccessError) Error() string { return "commerceext: " + e.Code + ": " + e.Message }

// Códigos de AccessError.
const (
	AccessErrInvalid          = "invalid"           // o pedido não passa a validação
	AccessErrCredentialTaken  = "credential_taken"  // o cartão já é de outra pessoa
	AccessErrCustomerLinked   = "customer_linked"   // o comprador já é outra pessoa
	AccessErrVisitorDocument  = "visitor_document"  // o documento já tem cadastro
	AccessErrHostNotActive    = "host_not_active"   // o anfitrião está suspenso
	AccessErrVisitCodeBusy    = "visit_code_busy"   // não há código livre agora
	AccessErrPersonNotActive  = "person_not_active" // a pessoa está suspensa ou removida
	AccessErrReferenceMissing = "reference_missing" // local, grupo ou porta não é do tenant
)

// AccessHost é a portaria do core para uma extensão. Todas as chamadas são do tenant do
// pedido; nenhuma recebe o tenant como argumento.
type AccessHost interface {
	// Places é a árvore de locais da portaria. Os locais são da tela da portaria: a
	// extensão escolhe entre eles, não os cria.
	Places(ctx context.Context) ([]AccessPlace, error)

	// CreateGroup cria um grupo desta extensão.
	CreateGroup(ctx context.Context, name string) (AccessGroup, error)
	RenameGroup(ctx context.Context, id, name string) (AccessGroup, error)
	// DeleteGroup apaga o grupo, quem está nele e as regras dele. Idempotente.
	DeleteGroup(ctx context.Context, id string) error

	// CreatePerson cria uma pessoa desta extensão.
	CreatePerson(ctx context.Context, in AccessPersonInput) (AccessPerson, error)
	// UpdatePerson troca nome, estado, nota, grupos e o comprador. Um grupo que não é desta
	// extensão recusa.
	UpdatePerson(ctx context.Context, id string, in AccessPersonInput) (AccessPerson, error)
	// RemovePerson marca a pessoa removida: revoga os cartões e apaga as regras dela.
	// Idempotente.
	RemovePerson(ctx context.Context, id string, by Actor) error
	// Person lê uma pessoa desta extensão, removida incluída. Outra: ErrAccessNotManaged.
	Person(ctx context.Context, id string) (AccessPerson, error)
	// PersonByCustomer é a pessoa desta extensão ligada ao comprador. found = false: não há,
	// ou é de outro dono (o host não o diz).
	PersonByCustomer(ctx context.Context, customerID string) (AccessPerson, bool, error)

	// GrantRule cria uma regra para uma pessoa ou grupo desta extensão.
	GrantRule(ctx context.Context, in AccessRuleInput, by Actor) (AccessRule, error)
	// Rules são as regras destes sujeitos (pessoas e grupos desta extensão).
	Rules(ctx context.Context, subjects []AccessSubject) ([]AccessRule, error)
	// DeleteRule apaga uma regra de um sujeito desta extensão. Idempotente.
	DeleteRule(ctx context.Context, id string) error

	// AddCredential dá um cartão a uma pessoa desta extensão.
	AddCredential(ctx context.Context, in AccessCredentialInput, by Actor) (AccessCredential, error)
	// Credentials são os cartões destas pessoas, revogados incluídos.
	Credentials(ctx context.Context, personIDs []string) ([]AccessCredential, error)
	RevokeCredential(ctx context.Context, id string, by Actor) error

	// LookupVisitor procura o cadastro pelo documento (só o HMAC é guardado).
	LookupVisitor(ctx context.Context, documentKind, documentNumber string) (AccessVisitor, bool, error)
	// CreateVisitor cadastra quem vem. O documento já cadastrado recusa com
	// AccessErrVisitorDocument; quem chama usa LookupVisitor.
	CreateVisitor(ctx context.Context, in AccessVisitorInput, by Actor) (AccessVisitor, error)
	// IssueVisit marca a visita, com anfitrião desta extensão, e devolve o código. O código
	// sai só aqui: no banco fica o HMAC.
	IssueVisit(ctx context.Context, in AccessVisitInput, by Actor) (AccessVisit, string, error)
	// Visits são as visitas destes anfitriões (desta extensão), das mais novas.
	Visits(ctx context.Context, hostPersonIDs []string) ([]AccessVisit, error)
	// CancelVisit cancela uma visita de um anfitrião desta extensão. Idempotente.
	CancelVisit(ctx context.Context, id string, by Actor) error

	// OpenableDoors são as portas que a pessoa pode abrir AGORA de longe: a regra dela na
	// porta e na hora, e o interruptor de abrir de longe da plataforma, da empresa e da
	// porta. Sem a abertura por comando neste deploy: ErrAccessUnavailable.
	OpenableDoors(ctx context.Context, personID string) ([]AccessDoor, error)
	// OpenDoor abre de longe pela regra da pessoa. Espera a confirmação do aparelho.
	OpenDoor(ctx context.Context, in AccessOpenRequest) (AccessOpenResult, error)

	// AllowDoorAnswer é o degrau da extensão para a pessoa atender a chamada da porta pela
	// área dela (ADR 0318, emenda da #4542). Nasce desligado. Mesmo ligado, a pessoa só
	// recebe com o interruptor da empresa e da porta, a escolha dela na área e a regra dela
	// naquela porta e hora. Só numa pessoa desta extensão; repetir não é erro.
	AllowDoorAnswer(ctx context.Context, personID string, allowed bool) error
}

// AccessPlace é um nó da árvore de locais. Uma regra num local vale nas portas dele e dos
// de baixo.
type AccessPlace struct {
	ID       string
	ParentID string
	Name     string
}

// AccessGroup é um grupo desta extensão.
type AccessGroup struct {
	ID          string
	Name        string
	MemberCount int
}

// Estado da pessoa.
const (
	AccessPersonActive    = "active"
	AccessPersonSuspended = "suspended"
)

// AccessPersonInput é o que a extensão escreve numa pessoa. CustomerID vazio = sem comprador;
// Status vazio = active.
type AccessPersonInput struct {
	Name       string
	Status     string
	Note       string
	CustomerID string
	GroupIDs   []string
}

// AccessPerson é uma pessoa desta extensão.
type AccessPerson struct {
	ID         string
	Name       string
	Status     string
	Note       string
	CustomerID string
	GroupIDs   []string
	RemovedAt  *time.Time
}

// A quem uma regra vale.
const (
	AccessSubjectPerson = "person"
	AccessSubjectGroup  = "group"
)

// AccessSubject é uma pessoa ou um grupo.
type AccessSubject struct {
	Kind string
	ID   string
}

// AccessRuleInput é uma regra de porta e hora. Weekdays vazio = todos (1 = segunda … 7 =
// domingo); StartTime e EndTime vazios = o dia inteiro, em HH:MM no fuso da empresa.
type AccessRuleInput struct {
	PlaceID    string
	Subject    AccessSubject
	Weekdays   []int
	StartTime  string
	EndTime    string
	ValidFrom  *time.Time
	ValidUntil *time.Time
}

// AccessRule é uma regra gravada.
type AccessRule struct {
	ID         string
	PlaceID    string
	Subject    AccessSubject
	Weekdays   []int
	StartTime  string
	EndTime    string
	ValidFrom  *time.Time
	ValidUntil *time.Time
}

// Tipo de credencial que a extensão dá.
const AccessCredentialCard = "card"

// AccessCredentialInput é um cartão. O número é normalizado pelo host.
type AccessCredentialInput struct {
	PersonID string
	Kind     string
	Value    string
	Label    string
}

// AccessCredential é um cartão gravado.
type AccessCredential struct {
	ID        string
	PersonID  string
	Kind      string
	Value     string
	Label     string
	RevokedAt *time.Time
}

// AccessVisitorInput é o cadastro de quem vem: nome e documento obrigatórios.
type AccessVisitorInput struct {
	Name           string
	DocumentKind   string // cpf, rg, cnh, passport, other
	DocumentNumber string
	Company        string
	Phone          string
}

// AccessVisitor é o cadastro, com o documento só mascarado.
type AccessVisitor struct {
	ID             string
	Name           string
	DocumentKind   string
	DocumentMasked string
	Company        string
}

// AccessVisitInput é a visita. Uma de DeviceID ou PlaceID. A avulsa (Preauthorized false)
// dura até 24 horas e vale uma entrada; a pré-autorizada vale MaxEntries entradas nos dias e
// no horário, até o teto de dias da empresa.
type AccessVisitInput struct {
	VisitorID            string
	HostPersonID         string
	GroupID              string
	DeviceID             string
	PlaceID              string
	StartsAt             time.Time
	EndsAt               time.Time
	Note                 string
	ReleaseWithoutNotice bool
	Preauthorized        bool
	MaxEntries           int
	Weekdays             []int
	StartTime            string
	EndTime              string
}

// AccessVisit é a visita gravada, sem o código.
type AccessVisit struct {
	ID                   string
	VisitorID            string
	VisitorName          string
	HostPersonID         string
	GroupID              string
	DeviceID             string
	PlaceID              string
	StartsAt             time.Time
	EndsAt               time.Time
	Status               string // scheduled, used, cancelled, expired
	Preauthorized        bool
	MaxEntries           int
	EntriesUsed          int
	ReleaseWithoutNotice bool
}

// AccessDoor é uma porta (o aparelho de porta) que a pessoa pode abrir.
type AccessDoor struct {
	DeviceID string
	Name     string
}

// AccessOpenRequest é o pedido de abrir de longe. PersonID é uma pessoa desta extensão; Via
// diz por onde a pessoa pediu, para o histórico (ex.: "customer_session").
type AccessOpenRequest struct {
	PersonID string
	DeviceID string
	Via      string
}

// Desfecho de abrir.
const (
	AccessOpened      = "opened"      // o aparelho confirmou o relé
	AccessUnconfirmed = "unconfirmed" // mandou abrir e o aparelho não confirmou
	AccessOpenFailed  = "failed"      // não conseguiu mandar abrir
	AccessOpenRefused = "refused"     // a regra ou um interruptor não deixa
)

// AccessOpenResult é o desfecho. Reason é o motivo curto da recusa ou da falha.
type AccessOpenResult struct {
	Status string
	Reason string
}

// refusedAccess recusa tudo com ErrNotDeclared: a extensão que não declarou
// `capabilities.access` recebe o erro que diz o que falta no manifesto.
type refusedAccess struct{}

func (refusedAccess) Places(context.Context) ([]AccessPlace, error) { return nil, ErrNotDeclared }
func (refusedAccess) CreateGroup(context.Context, string) (AccessGroup, error) {
	return AccessGroup{}, ErrNotDeclared
}
func (refusedAccess) RenameGroup(context.Context, string, string) (AccessGroup, error) {
	return AccessGroup{}, ErrNotDeclared
}
func (refusedAccess) DeleteGroup(context.Context, string) error { return ErrNotDeclared }
func (refusedAccess) CreatePerson(context.Context, AccessPersonInput) (AccessPerson, error) {
	return AccessPerson{}, ErrNotDeclared
}
func (refusedAccess) UpdatePerson(context.Context, string, AccessPersonInput) (AccessPerson, error) {
	return AccessPerson{}, ErrNotDeclared
}
func (refusedAccess) RemovePerson(context.Context, string, Actor) error { return ErrNotDeclared }
func (refusedAccess) Person(context.Context, string) (AccessPerson, error) {
	return AccessPerson{}, ErrNotDeclared
}
func (refusedAccess) PersonByCustomer(context.Context, string) (AccessPerson, bool, error) {
	return AccessPerson{}, false, ErrNotDeclared
}
func (refusedAccess) GrantRule(context.Context, AccessRuleInput, Actor) (AccessRule, error) {
	return AccessRule{}, ErrNotDeclared
}
func (refusedAccess) Rules(context.Context, []AccessSubject) ([]AccessRule, error) {
	return nil, ErrNotDeclared
}
func (refusedAccess) DeleteRule(context.Context, string) error { return ErrNotDeclared }
func (refusedAccess) AddCredential(context.Context, AccessCredentialInput, Actor) (AccessCredential, error) {
	return AccessCredential{}, ErrNotDeclared
}
func (refusedAccess) Credentials(context.Context, []string) ([]AccessCredential, error) {
	return nil, ErrNotDeclared
}
func (refusedAccess) RevokeCredential(context.Context, string, Actor) error { return ErrNotDeclared }
func (refusedAccess) LookupVisitor(context.Context, string, string) (AccessVisitor, bool, error) {
	return AccessVisitor{}, false, ErrNotDeclared
}
func (refusedAccess) CreateVisitor(context.Context, AccessVisitorInput, Actor) (AccessVisitor, error) {
	return AccessVisitor{}, ErrNotDeclared
}
func (refusedAccess) IssueVisit(context.Context, AccessVisitInput, Actor) (AccessVisit, string, error) {
	return AccessVisit{}, "", ErrNotDeclared
}
func (refusedAccess) Visits(context.Context, []string) ([]AccessVisit, error) {
	return nil, ErrNotDeclared
}
func (refusedAccess) CancelVisit(context.Context, string, Actor) error { return ErrNotDeclared }
func (refusedAccess) OpenableDoors(context.Context, string) ([]AccessDoor, error) {
	return nil, ErrNotDeclared
}
func (refusedAccess) OpenDoor(context.Context, AccessOpenRequest) (AccessOpenResult, error) {
	return AccessOpenResult{}, ErrNotDeclared
}
func (refusedAccess) AllowDoorAnswer(context.Context, string, bool) error { return ErrNotDeclared }

// AccessControl devolve a portaria, ou uma que recusa tudo com ErrNotDeclared.
func (r *Runtime) AccessControl() AccessHost {
	if r == nil || r.Access == nil {
		return refusedAccess{}
	}
	return r.Access
}
