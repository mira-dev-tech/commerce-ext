package commerceext

import (
	"strings"
	"testing"
)

func baseManifest() PluginManifest {
	var m PluginManifest
	m.APIVersion = manifestAPIVersion
	m.Kind = "Plugin"
	m.ID = "alfa-care"
	m.Version = "1.0.0"
	m.CompatibleCore = "^0.2.0"
	m.Runtime.Type = "in-process"
	return m
}

func TestDerivedSchema(t *testing.T) {
	for id, want := range map[string]string{
		"alfa-care": "ext_alfa_care",
		"kit":          "ext_kit",
		"a-b-c":        "ext_a_b_c",
	} {
		if got := DerivedSchema(id); got != want {
			t.Errorf("DerivedSchema(%q) = %q, queria %q", id, got, want)
		}
	}
}

// O manifesto é a fronteira (ADR 0227 D2): um plugin não declara o schema de
// outro, nem o do core, nem um caminho de migrações que saia da pasta dele.
func TestVerifyPluginManifest_Database(t *testing.T) {
	ok := func(mut func(*PluginManifest)) error {
		m := baseManifest()
		mut(&m)
		return VerifyPluginManifest(m, CoreLine)
	}

	if err := ok(func(m *PluginManifest) {
		m.Database.Schema = "ext_alfa_care"
		m.Database.Migrations = "migrations"
	}); err != nil {
		t.Fatalf("declaração válida recusada: %v", err)
	}
	// Sem database nenhum continua a valer — a maioria dos plugins não guarda
	// estado, e exigir-lhes schema quebraria os que já existem.
	if err := ok(func(*PluginManifest) {}); err != nil {
		t.Fatalf("plugin sem database recusado: %v", err)
	}

	for name, mut := range map[string]func(*PluginManifest){
		"schema do core": func(m *PluginManifest) {
			m.Database.Schema = "core"
			m.Database.Migrations = "migrations"
		},
		"schema de outro plugin": func(m *PluginManifest) {
			m.Database.Schema = "ext_outro_plugin"
			m.Database.Migrations = "migrations"
		},
		"schema sem prefixo": func(m *PluginManifest) {
			m.Database.Schema = "alfa_care"
			m.Database.Migrations = "migrations"
		},
		"schema com hifen (nao derivado)": func(m *PluginManifest) {
			m.Database.Schema = "ext_alfa-care"
			m.Database.Migrations = "migrations"
		},
		"schema sem migracoes": func(m *PluginManifest) {
			m.Database.Schema = "ext_alfa_care"
		},
		"migracoes sem schema": func(m *PluginManifest) {
			m.Database.Migrations = "migrations"
		},
		"migracoes absolutas": func(m *PluginManifest) {
			m.Database.Schema = "ext_alfa_care"
			m.Database.Migrations = "/etc/migrations"
		},
		"migracoes a sair da pasta": func(m *PluginManifest) {
			m.Database.Schema = "ext_alfa_care"
			m.Database.Migrations = "../../services/api/migrations"
		},
		"migracoes nao normalizadas": func(m *PluginManifest) {
			m.Database.Schema = "ext_alfa_care"
			m.Database.Migrations = "./migrations"
		},
	} {
		if err := ok(mut); err == nil {
			t.Errorf("%s: queria recusar", name)
		}
	}
}

// `../../services/api/migrations` faria o host correr as migrações do CORE com
// a tabela de versão do plugin. Vale um teste com nome próprio.
func TestVerifyPluginManifest_MigrationsCannotReachCore(t *testing.T) {
	m := baseManifest()
	m.Database.Schema = "ext_alfa_care"
	m.Database.Migrations = "../../services/api/migrations"
	err := VerifyPluginManifest(m, CoreLine)
	if err == nil {
		t.Fatal("queria recusar o caminho que alcança as migrações do core")
	}
	if !strings.Contains(err.Error(), "sai da pasta do plugin") {
		t.Errorf("a mensagem não diz o problema: %v", err)
	}
}

func TestVerifyPluginManifest_UI(t *testing.T) {
	withScreens := func(screens ...PluginScreen) error {
		m := baseManifest()
		m.UI.Screens = screens
		return VerifyPluginManifest(m, CoreLine)
	}
	good := PluginScreen{
		ID:    "care-recipients",
		Title: "Pessoas autorizadas",
		Route: "/admin/ext/alfa-care/care-recipients",
		Scope: "customers:write",
		URL:   "https://alfa.example.com/secretaria",
	}
	if err := withScreens(good); err != nil {
		t.Fatalf("tela válida recusada: %v", err)
	}

	const okURL = "https://alfa.example.com/secretaria"
	bad := map[string]PluginScreen{
		"rota fora do prefixo": {
			ID: "x", Title: "X", Route: "/admin/customers", Scope: "customers:write", URL: okURL,
		},
		"rota de outro plugin": {
			ID: "x", Title: "X", Route: "/admin/ext/outro-plugin/x", Scope: "customers:write", URL: okURL,
		},
		"rota com screen id diferente": {
			ID: "x", Title: "X", Route: "/admin/ext/alfa-care/y", Scope: "customers:write", URL: okURL,
		},
		"sem titulo": {
			ID: "x", Route: "/admin/ext/alfa-care/x", Scope: "customers:write", URL: okURL,
		},
		"sem scope": {
			ID: "x", Title: "X", Route: "/admin/ext/alfa-care/x", URL: okURL,
		},
		"scope mal formado": {
			ID: "x", Title: "X", Route: "/admin/ext/alfa-care/x", Scope: "customers", URL: okURL,
		},
		"id com maiuscula": {
			ID: "X", Title: "X", Route: "/admin/ext/alfa-care/X", Scope: "customers:write", URL: okURL,
		},
		// A tela vem de outra origem e é enquadrada no admin: http em claro,
		// credenciais no endereço e fragmento (que é do host) não entram.
		"sem url": {
			ID: "x", Title: "X", Route: "/admin/ext/alfa-care/x", Scope: "customers:write",
		},
		"url http": {
			ID: "x", Title: "X", Route: "/admin/ext/alfa-care/x", Scope: "customers:write",
			URL: "http://alfa.example.com/secretaria",
		},
		"url com credenciais": {
			ID: "x", Title: "X", Route: "/admin/ext/alfa-care/x", Scope: "customers:write",
			URL: "https://user:pass@alfa.example.com/secretaria",
		},
		"url com fragmento": {
			ID: "x", Title: "X", Route: "/admin/ext/alfa-care/x", Scope: "customers:write",
			URL: "https://alfa.example.com/secretaria#token",
		},
		"url sem host": {
			ID: "x", Title: "X", Route: "/admin/ext/alfa-care/x", Scope: "customers:write",
			URL: "https:///secretaria",
		},
	}
	for name, sc := range bad {
		if err := withScreens(sc); err == nil {
			t.Errorf("%s: queria recusar", name)
		}
	}

	// Duas telas com o mesmo id dariam duas entradas de menu iguais e uma rota
	// ambígua.
	dup := good
	if err := withScreens(good, dup); err == nil {
		t.Error("id de tela repetido: queria recusar")
	}
}

func TestVerifyPluginManifest_DataReads(t *testing.T) {
	withData := func(reads ...string) error {
		m := baseManifest()
		m.Capabilities.Data = reads
		return VerifyPluginManifest(m, CoreLine)
	}
	if err := withData("customers:read"); err != nil {
		t.Fatalf("leitura declarada e conhecida recusada: %v", err)
	}
	if err := withData(); err != nil {
		t.Fatalf("sem leitura declarada recusado: %v", err)
	}
	for name, reads := range map[string][]string{
		"desconhecida": {"orders:write"},
		"vazia":        {""},
		"repetida":     {"customers:read", "customers:read"},
	} {
		if err := withData(reads...); err == nil {
			t.Errorf("%s: queria recusar", name)
		}
	}
}

// O id entra no nome do schema e no prefixo da rota — texto arbitrário ali é
// um identificador do Postgres e um caminho de URL.
func TestVerifyPluginManifest_PluginID(t *testing.T) {
	for _, id := range []string{"A-maiuscula", "com espaco", "1comeca-com-digito", "x", "com_underscore", "com.ponto", ""} {
		m := baseManifest()
		m.ID = id
		if err := VerifyPluginManifest(m, CoreLine); err == nil {
			t.Errorf("id %q: queria recusar", id)
		}
	}
	for _, id := range []string{"kit", "alfa-care", "reference-antifraud"} {
		m := baseManifest()
		m.ID = id
		if err := VerifyPluginManifest(m, CoreLine); err != nil {
			t.Errorf("id %q: queria aceitar, veio %v", id, err)
		}
	}
}

func screenManifest(screen PluginScreen) PluginManifest {
	screen.ID, screen.Title, screen.Route, screen.Scope = "board", "Quadro", ScreenRoutePrefix+"forms/board", "records:read"
	return PluginManifest{ID: "forms", Version: "1.0.0", UI: PluginUI{Screens: []PluginScreen{screen}}}
}

func TestScreenSource(t *testing.T) {
	cases := []struct {
		name   string
		screen PluginScreen
		ok     bool
	}{
		{"url", PluginScreen{URL: "https://ext.example.com/a"}, true},
		{"bundle", PluginScreen{Bundle: "ui/index.html"}, true},
		{"bundle com origem", PluginScreen{Bundle: "ui/a/b.html", AllowedOrigins: []string{"https://x.example.com"}}, true},
		{"os dois", PluginScreen{URL: "https://ext.example.com/a", Bundle: "ui/index.html"}, false},
		{"nenhum", PluginScreen{}, false},
		{"bundle fora de ui", PluginScreen{Bundle: "plugin.go"}, false},
		{"bundle a subir", PluginScreen{Bundle: "ui/../segredo.html"}, false},
		{"bundle absoluto", PluginScreen{Bundle: "/etc/passwd.html"}, false},
		{"origem http", PluginScreen{Bundle: "ui/index.html", AllowedOrigins: []string{"http://x.example.com"}}, false},
		{"origem com caminho", PluginScreen{Bundle: "ui/index.html", AllowedOrigins: []string{"https://x.example.com/a"}}, false},
		{"origem curinga", PluginScreen{Bundle: "ui/index.html", AllowedOrigins: []string{"*"}}, false},
		{"origem em tela de url", PluginScreen{URL: "https://ext.example.com/a", AllowedOrigins: []string{"https://x.example.com"}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifyPluginManifest(screenManifest(tc.screen), "0.2.0")
			if (err == nil) != tc.ok {
				t.Fatalf("err=%v, ok=%v", err, tc.ok)
			}
		})
	}
}

func TestValidateStaffAPI(t *testing.T) {
	cases := []struct {
		name string
		a    []PluginStaffAction
		ok   bool
	}{
		{"vazio", nil, true},
		{"certo", []PluginStaffAction{{ID: "notes", Methods: []string{"GET", "post"}, Scope: "records:read"}}, true},
		{"sem scope", []PluginStaffAction{{ID: "notes", Methods: []string{"GET"}}}, false},
		{"sem verbo", []PluginStaffAction{{ID: "notes", Scope: "records:read"}}, false},
		{"verbo fora", []PluginStaffAction{{ID: "notes", Methods: []string{"PUT"}, Scope: "records:read"}}, false},
		{"id curinga", []PluginStaffAction{{ID: "*", Methods: []string{"GET"}, Scope: "records:read"}}, false},
		{"repetida", []PluginStaffAction{{ID: "a1", Methods: []string{"GET"}, Scope: "r:a"}, {ID: "a1", Methods: []string{"GET"}, Scope: "r:a"}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateStaffAPI(tc.a); (err == nil) != tc.ok {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
