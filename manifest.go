package commerceext

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const manifestAPIVersion = "commerce.mira.dev/v1"

// ClientManifest is the deploy-level manifest (repo cliente).
type ClientManifest struct {
	APIVersion string `yaml:"apiVersion"`
	Core       struct {
		Image string `yaml:"image"`
	} `yaml:"core"`
	Modules  map[string]bool `yaml:"modules"`
	Plugins  []PluginRef     `yaml:"plugins"`
	Vertical struct {
		Preset string `yaml:"preset"`
	} `yaml:"vertical"`
}

// PluginManifest is kind: Plugin (per extension).
type PluginManifest struct {
	APIVersion     string `yaml:"apiVersion"`
	Kind           string `yaml:"kind"`
	ID             string `yaml:"id"`
	Version        string `yaml:"version"`
	CompatibleCore string `yaml:"compatibleCore"`
	Runtime        struct {
		Type   string `yaml:"type"`
		Binary string `yaml:"binary"`
	} `yaml:"runtime"`
	// Database é a persistência PRÓPRIA do plugin (ADR 0004 / ADR 0227 D1).
	// O plugin declara; o host cria o schema e corre estas migrações por banco
	// de tenant. Vazio = plugin sem estado próprio.
	Database PluginDatabase `yaml:"database"`
	// UI são as telas que o ADMIN renderiza para este plugin (ADR 0227 D7).
	UI PluginUI `yaml:"ui"`
	// CustomerAPI são as acções que a VITRINE chama com a sessão da compradora
	// (ADR 0293). O id entra no caminho; os métodos são o que o host aceita.
	// Sem esta lista a extensão não tem porta de escrita para o browser.
	CustomerAPI []PluginCustomerAction `yaml:"customer_api"`
	// CustomerUI são as telas que a COMPRADORA abre (ADR 0327). O core serve-as no
	// checkout do tenant, numa moldura neutra e em origem separada; a tela não recebe
	// credencial nenhuma e fala só com as acções de `customer_api`, pelo host.
	CustomerUI PluginCustomerUI `yaml:"customer_ui"`
	// StaffRecords é a porta da EQUIPE (ADR 0294). O admin lê e edita o
	// documento na ficha da pessoa. Vazio = a extensão não aparece lá.
	StaffRecords PluginStaffRecords `yaml:"staff_records"`
	// StaffAPI são as acções que a EQUIPE chama com a pessoa identificada
	// (ADR 0317 D2). Cada uma declara o scope que exige.
	StaffAPI     []PluginStaffAction `yaml:"staff_api"`
	Capabilities struct {
		Hooks  []string `yaml:"hooks"`
		Events struct {
			Subscribe []string `yaml:"subscribe"`
			Publish   []string `yaml:"publish"`
		} `yaml:"events"`
		Integrations []string `yaml:"integrations"`
		// Data são as leituras do CORE que este plugin declara precisar
		// (ADR 0227 D9). Sem declaração, `Runtime.Core` devolve negado — a
		// leitura é concedida pelo manifesto revisto em PR, nunca pedida em
		// runtime.
		Data []string `yaml:"data"`
		// CustomerDocuments deixa a extensão entregar documento ao cliente
		// pela mesma regra da equipe (ADR 0309 D6).
		CustomerDocuments bool `yaml:"customer_documents"`
		// Signing deixa a extensão pedir a assinatura da pessoa que age, com o
		// certificado dela (ADR 0317 D5).
		Signing bool `yaml:"signing"`
		// Access deixa a extensão gerir pessoas, grupos, regras, cartões e visitas da
		// portaria — só os que ela criou — e abrir pela regra da pessoa (ADR 0318 F5).
		Access bool `yaml:"access"`
		// Parcels deixa a extensão mostrar à pessoa do comprador as encomendas dela na
		// portaria e o passe de retirada (ADR 0318 F6). A pessoa é uma das que a extensão criou
		// pelo host access, por isso exige `access`.
		Parcels bool `yaml:"parcels"`
	} `yaml:"capabilities"`
}

// PluginDatabase declara o schema próprio do plugin.
type PluginDatabase struct {
	// Schema é `ext_<id>` com `-` → `_`. É DERIVADO do id: escreve-se aqui
	// para o manifesto ser legível, e o Verify recusa qualquer outro valor —
	// um plugin a declarar `core` ou o schema de outro plugin seria escrita
	// no schema alheio (ADR 0227 D2).
	Schema string `yaml:"schema"`
	// Migrations é a pasta (RELATIVA à pasta do manifesto) com as migrações
	// goose do plugin. Corridas com tabela de versão própria,
	// `<schema>.goose_db_version`, nunca a do core.
	Migrations string `yaml:"migrations"`
}

// PluginUI declara as telas do plugin no admin.
type PluginUI struct {
	Screens []PluginScreen `yaml:"screens"`
}

// PluginScreen é uma tela do plugin renderizada pelo admin.
type PluginScreen struct {
	ID    string `yaml:"id"`
	Title string `yaml:"title"`
	Icon  string `yaml:"icon"`
	// Route é obrigatoriamente `/admin/ext/<plugin-id>/<screen-id>`. O
	// prefixo não é cosmético: é o que permite uma política de origem só para
	// telas de extensão.
	Route string `yaml:"route"`
	// URL é DE ONDE a tela vem — a página que o admin carrega em origem
	// separada. `https://` obrigatório e sem credenciais no URL.
	//
	// A tela ser de outra origem é o desenho, não uma limitação: assim ela não
	// lê a sessão do admin nem o DOM dele, e o que recebe é só o token
	// recortado que o core lhe envia por `postMessage` (ADR 0227 D7).
	URL string `yaml:"url"`
	// Bundle é a alternativa ao URL (ADR 0317 D4): o ficheiro de entrada da tela
	// empacotada em `ui/` na pasta da extensão, servido pelo core na origem da API.
	Bundle string `yaml:"bundle"`
	// AllowedOrigins são as origens https que a tela empacotada pode alcançar
	// além da API, como um módulo de terceiro. Vão para a CSP.
	AllowedOrigins []string `yaml:"allowed_origins"`
	// Scope é o TECTO de permissão da tela, não uma concessão: o token que ela
	// recebe leva os scopes daqui ∩ os da pessoa logada.
	Scope string `yaml:"scope"`
}

// PluginRef is a plugin pin in the client manifest.
type PluginRef struct {
	ID      string `yaml:"id"`
	Version string `yaml:"version"`
}

// LoadClientManifest reads and parses a client manifest YAML file.
func LoadClientManifest(path string) (ClientManifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ClientManifest{}, err
	}
	var m ClientManifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return ClientManifest{}, err
	}
	return m, nil
}

// LoadPluginManifest reads a plugin manifest YAML file.
func LoadPluginManifest(path string) (PluginManifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return PluginManifest{}, err
	}
	var m PluginManifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return PluginManifest{}, err
	}
	return m, nil
}

// VerifyClientManifest validates client manifest against core version.
func VerifyClientManifest(m ClientManifest, coreVersion string) error {
	if m.APIVersion != "" && m.APIVersion != manifestAPIVersion {
		return fmt.Errorf("apiVersion %q unsupported (want %s)", m.APIVersion, manifestAPIVersion)
	}
	if m.Core.Image == "" {
		return fmt.Errorf("core.image is required")
	}
	if len(m.Plugins) == 0 {
		return fmt.Errorf("at least one plugin entry required for verify (or use --skip-plugins)")
	}
	for _, p := range m.Plugins {
		if strings.TrimSpace(p.ID) == "" {
			return fmt.Errorf("plugin id is required")
		}
	}
	_ = coreVersion
	return nil
}

// VerifyPluginManifest validates plugin manifest hooks/events against catalog.
func VerifyPluginManifest(m PluginManifest, coreVersion string) error {
	if m.Kind != "" && m.Kind != "Plugin" {
		return fmt.Errorf("kind must be Plugin")
	}
	if m.APIVersion != "" && m.APIVersion != manifestAPIVersion {
		return fmt.Errorf("apiVersion %q unsupported", m.APIVersion)
	}
	if m.ID == "" {
		return fmt.Errorf("id is required")
	}
	// O id entra no NOME do schema e no prefixo da rota da tela — deixá-lo
	// livre punha texto arbitrário num identificador do Postgres e num caminho
	// de URL.
	if !rePluginID.MatchString(m.ID) {
		return fmt.Errorf("id %q inválido — minúsculas, dígitos e `-` (2 a 40 caracteres, começa com letra)", m.ID)
	}
	if m.Version == "" {
		return fmt.Errorf("version is required")
	}
	if m.CompatibleCore != "" && !compatibleCoreMatches(m.CompatibleCore, coreVersion) {
		return fmt.Errorf("compatibleCore %q does not match core %q", m.CompatibleCore, coreVersion)
	}
	for _, h := range m.Capabilities.Hooks {
		if !knownHook(h) {
			return fmt.Errorf("unknown hook %q", h)
		}
	}
	for _, e := range m.Capabilities.Events.Subscribe {
		if !knownEvent(e) {
			return fmt.Errorf("unknown subscribe event %q", e)
		}
	}
	// Extensibilidade (ADR 0227): schema próprio, telas e leituras do core.
	if err := ValidatePluginDatabase(m.ID, m.Database); err != nil {
		return err
	}
	if err := ValidatePluginUI(m.ID, m.UI); err != nil {
		return err
	}
	if err := ValidatePluginDataReads(m.Capabilities.Data); err != nil {
		return err
	}
	if err := ValidateCustomerAPI(m.CustomerAPI); err != nil {
		return err
	}
	if err := ValidatePluginCustomerUI(m.CustomerUI, len(m.CustomerAPI) > 0); err != nil {
		return err
	}
	if err := ValidateStaffRecords(m.StaffRecords); err != nil {
		return err
	}
	if err := ValidateStaffAPI(m.StaffAPI); err != nil {
		return err
	}
	// A área das encomendas é de uma pessoa que a extensão criou pela portaria: sem `access`
	// não há pessoa, e o host recusaria cada chamada sem dizer porquê.
	if m.Capabilities.Parcels && !m.Capabilities.Access {
		return fmt.Errorf("capabilities.parcels exige capabilities.access (a pessoa é da portaria)")
	}
	return nil
}

func knownHook(id string) bool {
	for _, h := range AllHooks {
		if h == id {
			return true
		}
	}
	return false
}

func knownEvent(id string) bool {
	for _, e := range AllCoreEvents {
		if e == id {
			return true
		}
	}
	// namespaced plugin events: vendor.action
	return strings.Contains(id, ".")
}

func compatibleCoreMatches(rangeExpr, coreVersion string) bool {
	rangeExpr = strings.TrimSpace(rangeExpr)
	coreVersion = strings.TrimSpace(coreVersion)
	if rangeExpr == "" || coreVersion == "" {
		return true
	}
	if strings.HasPrefix(rangeExpr, "^") {
		prefix := strings.TrimPrefix(rangeExpr, "^")
		major := strings.Split(prefix, ".")[0]
		coreMajor := strings.Split(coreVersion, ".")[0]
		return major == coreMajor
	}
	return rangeExpr == coreVersion
}
