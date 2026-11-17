package commerceext

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
)

// Validação das declarações de extensibilidade do manifesto (ADR 0227).
//
// Tudo o que está aqui é recusado no `commerce-ext verify` e no arranque do
// host. A razão de ser rigoroso num ficheiro YAML: o manifesto é a ÚNICA coisa
// que o core lê antes de provisionar schema e menu, e enquanto o runtime for
// in-process (ADR 0227 D5) não há role do Postgres a segurar um erro aqui.

const (
	// SchemaPrefix é o prefixo obrigatório do schema de um plugin (ADR 0004).
	SchemaPrefix = "ext_"
	// MaxSchemaLen deixa margem para `<schema>.goose_db_version` caber nos 63
	// caracteres de um identificador do Postgres.
	MaxSchemaLen = 40
	// ScreenRoutePrefix é a raiz obrigatória de toda tela de extensão.
	ScreenRoutePrefix = "/admin/ext/"
)

var (
	rePluginID = regexp.MustCompile(`^[a-z][a-z0-9-]{1,39}$`)
	reScreenID = regexp.MustCompile(`^[a-z][a-z0-9-]{1,39}$`)
	reActionID = regexp.MustCompile(`^[a-z][a-z0-9-]{1,39}$`)
	// A tela empacotada: caminho relativo dentro de ui/ (ADR 0317 D4).
	reScreenBundle = regexp.MustCompile(`^ui/[a-zA-Z0-9_./-]+\.html$`)
	reScope        = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}:[a-z]{1,16}$`)
)

// CustomerAPIMethods é o conjunto fechado de verbos que uma acção de comprador
// pode declarar. Fora disto o host não abre a porta.
var CustomerAPIMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE"}

// AllDataReads é o domínio FECHADO das leituras do core que um plugin pode
// declarar. Lista curta de propósito: cada entrada é uma superfície que alguém
// tem de desenhar e manter, e «ler o que precisar» não é uma permissão.
var AllDataReads = []string{
	// Identidade da pessoa (nome, e-mail, documento), para um módulo de terceiro
	// que a exige (ADR 0317 D6).
	"customers:identity",
	// Marcações da agenda, por dia, recurso ou cliente (ADR 0317 D6).
	"appointments:read",
	// Ler o cliente de um pedido — o mínimo que a ADR 0227 D9 pede.
	"customers:read",
}

// DerivedSchema devolve o schema que um plugin com este id TEM de declarar.
func DerivedSchema(pluginID string) string {
	return SchemaPrefix + strings.ReplaceAll(strings.TrimSpace(pluginID), "-", "_")
}

// ValidatePluginDatabase recusa uma declaração de persistência que o host não
// consiga provisionar em segurança.
func ValidatePluginDatabase(pluginID string, db PluginDatabase) error {
	schema := strings.TrimSpace(db.Schema)
	migrations := strings.TrimSpace(db.Migrations)
	if schema == "" && migrations == "" {
		return nil // plugin sem estado próprio
	}
	if schema == "" {
		return fmt.Errorf("database.migrations sem database.schema")
	}
	want := DerivedSchema(pluginID)
	if schema != want {
		// O erro diz o valor esperado porque a alternativa é alguém "corrigir"
		// o id do plugin para casar com o schema que escreveu.
		return fmt.Errorf("database.schema deve ser %q (derivado do id %q), não %q", want, pluginID, schema)
	}
	if len(schema) > MaxSchemaLen {
		return fmt.Errorf("database.schema %q acima de %d caracteres", schema, MaxSchemaLen)
	}
	if migrations == "" {
		return fmt.Errorf("database.schema sem database.migrations: um schema vazio não serve a ninguém")
	}
	if err := validateRelativeDir(migrations); err != nil {
		return fmt.Errorf("database.migrations: %w", err)
	}
	return nil
}

// validateRelativeDir recusa caminho absoluto ou a sair da pasta do plugin.
// Sem isto, `migrations: ../../services/api/migrations` fazia o host correr as
// migrações do CORE com a tabela de versão do plugin.
func validateRelativeDir(dir string) error {
	if path.IsAbs(dir) || strings.HasPrefix(dir, "/") || strings.HasPrefix(dir, `\`) {
		return fmt.Errorf("%q é absoluto — use um caminho relativo à pasta do plugin", dir)
	}
	clean := path.Clean(dir)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("%q sai da pasta do plugin", dir)
	}
	if clean != dir {
		return fmt.Errorf("%q não está normalizado (esperado %q)", dir, clean)
	}
	return nil
}

// ValidatePluginUI recusa telas que o admin não consiga montar ou isolar.
func ValidatePluginUI(pluginID string, ui PluginUI) error {
	seen := make(map[string]struct{}, len(ui.Screens))
	for i, sc := range ui.Screens {
		id := strings.TrimSpace(sc.ID)
		if !reScreenID.MatchString(id) {
			return fmt.Errorf("ui.screens[%d].id %q inválido — minúsculas, dígitos e `-` (2 a 40)", i, sc.ID)
		}
		if _, dup := seen[id]; dup {
			return fmt.Errorf("ui.screens[%d].id %q repetido", i, id)
		}
		seen[id] = struct{}{}
		if strings.TrimSpace(sc.Title) == "" {
			return fmt.Errorf("ui.screens[%d].title é obrigatório — é o rótulo no menu", i)
		}
		want := ScreenRoutePrefix + pluginID + "/" + id
		if strings.TrimSpace(sc.Route) != want {
			return fmt.Errorf("ui.screens[%d].route deve ser %q, não %q", i, want, sc.Route)
		}
		// Tela sem scope seria tela visível a toda a equipa. O manifesto tem de
		// dizer o tecto; quem concede é a permissão da pessoa.
		if !reScope.MatchString(strings.TrimSpace(sc.Scope)) {
			return fmt.Errorf("ui.screens[%d].scope %q inválido — esperado `recurso:acção` (ex. customers:write)", i, sc.Scope)
		}
		if err := validateScreenSource(sc); err != nil {
			return fmt.Errorf("ui.screens[%d]: %w", i, err)
		}
	}
	return nil
}

// validateScreenSource exige uma origem só: URL externo ou tela empacotada.
func validateScreenSource(sc PluginScreen) error {
	url, bundle := strings.TrimSpace(sc.URL), strings.TrimSpace(sc.Bundle)
	if url != "" && bundle != "" {
		return fmt.Errorf("url e bundle juntos — a tela vem de um sítio só")
	}
	if bundle == "" {
		if len(sc.AllowedOrigins) > 0 {
			return fmt.Errorf("allowed_origins só vale para tela empacotada")
		}
		if err := validateScreenURL(url); err != nil {
			return fmt.Errorf("url: %w", err)
		}
		return nil
	}
	if !reScreenBundle.MatchString(bundle) || strings.Contains(bundle, "..") {
		return fmt.Errorf("bundle %q inválido — caminho relativo dentro de ui/, terminado em .html", bundle)
	}
	for j, raw := range sc.AllowedOrigins {
		if err := validateScreenOrigin(strings.TrimSpace(raw)); err != nil {
			return fmt.Errorf("allowed_origins[%d]: %w", j, err)
		}
	}
	return nil
}

// validateScreenOrigin aceita só `https://host`, sem caminho: o valor vai para a CSP.
func validateScreenOrigin(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return fmt.Errorf("%q tem de ser uma origem https, sem caminho", raw)
	}
	return nil
}

// validateScreenURL recusa a origem que o admin não deve carregar.
//
// Não é defesa contra SSRF — quem busca o URL é o BROWSER, não o servidor. É
// defesa contra enquadrar no admin uma página que viaja em claro ou que traz
// credenciais no próprio endereço (que iriam para o histórico, para o Referer
// e para os logs de quem estiver no caminho).
func validateScreenURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("é obrigatório — é de onde a tela vem")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%q não é um URL válido", raw)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("%q tem de ser https (a tela corre dentro do admin)", raw)
	}
	if u.Host == "" {
		return fmt.Errorf("%q sem host", raw)
	}
	if u.User != nil {
		return fmt.Errorf("URL com credenciais não entra — elas acabariam no histórico e nos logs")
	}
	if u.Fragment != "" {
		// O fragmento é nosso: é por ali que o host passa o contexto da tela.
		return fmt.Errorf("URL com fragmento (`#…`) não entra — o fragmento é do host")
	}
	return nil
}

// ValidatePluginDataReads recusa leitura do core fora do domínio fechado.
func ValidatePluginDataReads(reads []string) error {
	seen := make(map[string]struct{}, len(reads))
	for i, r := range reads {
		r = strings.TrimSpace(r)
		if r == "" {
			return fmt.Errorf("capabilities.data[%d] vazio", i)
		}
		if _, dup := seen[r]; dup {
			return fmt.Errorf("capabilities.data[%d] %q repetido", i, r)
		}
		seen[r] = struct{}{}
		if !knownDataRead(r) {
			return fmt.Errorf("capabilities.data[%d] %q desconhecido (disponíveis: %s)",
				i, r, strings.Join(AllDataReads, ", "))
		}
	}
	return nil
}

// PluginCustomerAction é uma acção que a sessão da compradora pode chamar
// (ADR 0293). O id é o segmento do caminho; methods é o que o host deixa passar.
type PluginCustomerAction struct {
	ID      string   `yaml:"id"`
	Methods []string `yaml:"methods"`
}

// ValidateCustomerAPI recusa acções que o host não consiga expor com segurança.
func ValidateCustomerAPI(actions []PluginCustomerAction) error {
	seen := make(map[string]struct{}, len(actions))
	for i, a := range actions {
		id := strings.TrimSpace(a.ID)
		if id != "*" && !reActionID.MatchString(id) {
			return fmt.Errorf("customer_api[%d].id %q inválido — minúsculas, dígitos e `-` (2 a 40), ou `*` para qualquer acção", i, a.ID)
		}
		if _, dup := seen[id]; dup {
			return fmt.Errorf("customer_api[%d].id %q repetido", i, id)
		}
		seen[id] = struct{}{}
		if len(a.Methods) == 0 {
			return fmt.Errorf("customer_api[%d].methods vazio — declare pelo menos um verbo", i)
		}
		methods := make(map[string]struct{}, len(a.Methods))
		for j, m := range a.Methods {
			method := strings.ToUpper(strings.TrimSpace(m))
			if !knownCustomerMethod(method) {
				return fmt.Errorf("customer_api[%d].methods[%d] %q inválido (disponíveis: %s)",
					i, j, m, strings.Join(CustomerAPIMethods, ", "))
			}
			if _, dup := methods[method]; dup {
				return fmt.Errorf("customer_api[%d].methods[%d] %q repetido", i, j, method)
			}
			methods[method] = struct{}{}
		}
	}
	return nil
}

// StaffAPIMethods é o conjunto fechado de verbos da porta da equipe.
var StaffAPIMethods = []string{"GET", "POST", "PATCH", "DELETE"}

// PluginStaffAction é uma acção que a equipe chama com a pessoa identificada
// (ADR 0317 D2). Scope é o que a pessoa tem de ter, conferido no acto.
type PluginStaffAction struct {
	ID      string   `yaml:"id"`
	Methods []string `yaml:"methods"`
	Scope   string   `yaml:"scope"`
}

// ValidateStaffAPI recusa acção sem scope: seria uma porta aberta a toda a equipe.
func ValidateStaffAPI(actions []PluginStaffAction) error {
	seen := make(map[string]struct{}, len(actions))
	for i, a := range actions {
		id := strings.TrimSpace(a.ID)
		if !reActionID.MatchString(id) {
			return fmt.Errorf("staff_api[%d].id %q inválido — minúsculas, dígitos e `-` (2 a 40)", i, a.ID)
		}
		if _, dup := seen[id]; dup {
			return fmt.Errorf("staff_api[%d].id %q repetido", i, id)
		}
		seen[id] = struct{}{}
		if !reScope.MatchString(strings.TrimSpace(a.Scope)) {
			return fmt.Errorf("staff_api[%d].scope %q inválido — esperado `recurso:acção`", i, a.Scope)
		}
		if len(a.Methods) == 0 {
			return fmt.Errorf("staff_api[%d].methods vazio — declare pelo menos um verbo", i)
		}
		methods := make(map[string]struct{}, len(a.Methods))
		for j, m := range a.Methods {
			method := strings.ToUpper(strings.TrimSpace(m))
			known := false
			for _, ok := range StaffAPIMethods {
				known = known || ok == method
			}
			if !known {
				return fmt.Errorf("staff_api[%d].methods[%d] %q inválido (disponíveis: %s)", i, j, m, strings.Join(StaffAPIMethods, ", "))
			}
			if _, dup := methods[method]; dup {
				return fmt.Errorf("staff_api[%d].methods[%d] %q repetido", i, j, method)
			}
			methods[method] = struct{}{}
		}
	}
	return nil
}

// PluginStaffRecords abre os registros da extensão para a equipe (ADR 0294).
// Title é o rótulo da aba e do menu. Methods é o que o host deixa passar:
// GET lê, PATCH grava o documento inteiro que a tela editou.
type PluginStaffRecords struct {
	Title   string   `yaml:"title"`
	Methods []string `yaml:"methods"`
}

// ValidateStaffRecords aceita o bloco ausente. Presente, exige título e pelo
// menos GET. PATCH é o que torna a tela editável.
func ValidateStaffRecords(s PluginStaffRecords) error {
	if strings.TrimSpace(s.Title) == "" && len(s.Methods) == 0 {
		return nil
	}
	if strings.TrimSpace(s.Title) == "" {
		return fmt.Errorf("staff_records.title vazio — é o rótulo no admin")
	}
	if len(s.Methods) == 0 {
		return fmt.Errorf("staff_records.methods vazio — declare GET")
	}
	seen := map[string]struct{}{}
	hasGet := false
	for i, m := range s.Methods {
		method := strings.ToUpper(strings.TrimSpace(m))
		if method != "GET" && method != "PATCH" {
			return fmt.Errorf("staff_records.methods[%d] %q inválido (disponíveis: GET, PATCH)", i, m)
		}
		if _, dup := seen[method]; dup {
			return fmt.Errorf("staff_records.methods[%d] %q repetido", i, method)
		}
		seen[method] = struct{}{}
		if method == "GET" {
			hasGet = true
		}
	}
	if !hasGet {
		return fmt.Errorf("staff_records.methods sem GET — a equipe não teria o que abrir")
	}
	return nil
}

func knownCustomerMethod(method string) bool {
	for _, m := range CustomerAPIMethods {
		if m == method {
			return true
		}
	}
	return false
}

func knownDataRead(id string) bool {
	for _, d := range AllDataReads {
		if d == id {
			return true
		}
	}
	return false
}

// ── Tela da compradora (ADR 0327) ───────────────────────────────────────────

const (
	// CustomerScreenBundleDir é a pasta da tela da compradora, separada de `ui/`
	// (a da equipe): o core serve-a noutro prefixo, com a CSP dela, e uma não
	// carrega os ficheiros da outra.
	CustomerScreenBundleDir = "customer-ui"
	// MaxCustomerScreens é o tecto de telas da compradora por extensão.
	MaxCustomerScreens = 8
	// maxCustomerScreenTitle é o rótulo na moldura e na lista da área.
	maxCustomerScreenTitle = 60
	// maxCustomerScreenDescription é a linha de apoio na lista da área.
	maxCustomerScreenDescription = 200
)

var reCustomerScreenBundle = regexp.MustCompile(`^customer-ui/[a-zA-Z0-9_./-]+\.html$`)

// PluginCustomerUI declara as telas da compradora.
type PluginCustomerUI struct {
	Screens []PluginCustomerScreen `yaml:"screens"`
}

// PluginCustomerScreen é uma tela que a pessoa da sessão abre no checkout do
// tenant. Só empacotada: um URL externo poria uma página de fora diante da
// compradora, com a moldura da plataforma a dar-lhe credibilidade.
type PluginCustomerScreen struct {
	ID          string `yaml:"id"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	// Bundle é o ficheiro de entrada em `customer-ui/`, servido pelo core na
	// origem da API.
	Bundle string `yaml:"bundle"`
	// AllowedOrigins são as origens https que a tela pode alcançar além da
	// própria. Vão para a CSP dela, e só dela.
	AllowedOrigins []string `yaml:"allowed_origins"`
}

// ValidatePluginCustomerUI recusa a tela que o core não consiga servir isolada.
// hasCustomerAPI: a tela só fala com `customer_api`; sem acções, não teria o que
// mostrar.
func ValidatePluginCustomerUI(ui PluginCustomerUI, hasCustomerAPI bool) error {
	if len(ui.Screens) == 0 {
		return nil
	}
	if len(ui.Screens) > MaxCustomerScreens {
		return fmt.Errorf("customer_ui.screens acima de %d telas", MaxCustomerScreens)
	}
	if !hasCustomerAPI {
		return fmt.Errorf("customer_ui sem customer_api — a tela da compradora só fala com as acções declaradas ali")
	}
	seen := make(map[string]struct{}, len(ui.Screens))
	for i, sc := range ui.Screens {
		id := strings.TrimSpace(sc.ID)
		if !reScreenID.MatchString(id) {
			return fmt.Errorf("customer_ui.screens[%d].id %q inválido — minúsculas, dígitos e `-` (2 a 40)", i, sc.ID)
		}
		if _, dup := seen[id]; dup {
			return fmt.Errorf("customer_ui.screens[%d].id %q repetido", i, id)
		}
		seen[id] = struct{}{}
		title := strings.TrimSpace(sc.Title)
		if title == "" || len([]rune(title)) > maxCustomerScreenTitle {
			return fmt.Errorf("customer_ui.screens[%d].title é obrigatório, até %d caracteres", i, maxCustomerScreenTitle)
		}
		if len([]rune(strings.TrimSpace(sc.Description))) > maxCustomerScreenDescription {
			return fmt.Errorf("customer_ui.screens[%d].description acima de %d caracteres", i, maxCustomerScreenDescription)
		}
		bundle := strings.TrimSpace(sc.Bundle)
		if !reCustomerScreenBundle.MatchString(bundle) || strings.Contains(bundle, "..") {
			return fmt.Errorf("customer_ui.screens[%d].bundle %q inválido — caminho relativo dentro de %s/, terminado em .html",
				i, sc.Bundle, CustomerScreenBundleDir)
		}
		for j, raw := range sc.AllowedOrigins {
			if err := validateScreenOrigin(strings.TrimSpace(raw)); err != nil {
				return fmt.Errorf("customer_ui.screens[%d].allowed_origins[%d]: %w", i, j, err)
			}
		}
	}
	return nil
}
