package commerceext

import (
	"strings"
	"testing"
)

func customerScreen(id string) PluginCustomerScreen {
	return PluginCustomerScreen{ID: id, Title: "Área", Bundle: "customer-ui/index.html"}
}

func TestValidatePluginCustomerUI_AcceptsBundledScreen(t *testing.T) {
	ui := PluginCustomerUI{Screens: []PluginCustomerScreen{{
		ID: "morador", Title: "Área do morador", Description: "Visitas e portas",
		Bundle: "customer-ui/index.html", AllowedOrigins: []string{"https://cdn.example.com"},
	}}}
	if err := ValidatePluginCustomerUI(ui, true); err != nil {
		t.Fatal(err)
	}
}

func TestValidatePluginCustomerUI_EmptyIsFine(t *testing.T) {
	if err := ValidatePluginCustomerUI(PluginCustomerUI{}, false); err != nil {
		t.Fatal(err)
	}
}

func TestValidatePluginCustomerUI_Rejects(t *testing.T) {
	long := strings.Repeat("a", maxCustomerScreenTitle+1)
	cases := map[string]struct {
		ui     PluginCustomerUI
		hasAPI bool
		want   string
	}{
		// Sem customer_api a tela não tem com o que falar.
		"sem customer_api": {PluginCustomerUI{Screens: []PluginCustomerScreen{customerScreen("area")}}, false, "sem customer_api"},
		"id inválido":      {PluginCustomerUI{Screens: []PluginCustomerScreen{customerScreen("Área")}}, true, "id"},
		"id repetido":      {PluginCustomerUI{Screens: []PluginCustomerScreen{customerScreen("area"), customerScreen("area")}}, true, "repetido"},
		"sem título": {PluginCustomerUI{Screens: []PluginCustomerScreen{{
			ID: "area", Bundle: "customer-ui/index.html"}}}, true, "title"},
		"título longo": {PluginCustomerUI{Screens: []PluginCustomerScreen{{
			ID: "area", Title: long, Bundle: "customer-ui/index.html"}}}, true, "title"},
		// A pasta da equipe não serve à compradora: CSP e prefixo são outros.
		"bundle em ui/": {PluginCustomerUI{Screens: []PluginCustomerScreen{{
			ID: "area", Title: "Área", Bundle: "ui/index.html"}}}, true, "bundle"},
		"bundle sai da pasta": {PluginCustomerUI{Screens: []PluginCustomerScreen{{
			ID: "area", Title: "Área", Bundle: "customer-ui/../ui/index.html"}}}, true, "bundle"},
		"bundle sem html": {PluginCustomerUI{Screens: []PluginCustomerScreen{{
			ID: "area", Title: "Área", Bundle: "customer-ui/app.js"}}}, true, "bundle"},
		"origem http": {PluginCustomerUI{Screens: []PluginCustomerScreen{{
			ID: "area", Title: "Área", Bundle: "customer-ui/index.html", AllowedOrigins: []string{"http://cdn.example.com"}}}}, true, "allowed_origins"},
		"origem com caminho": {PluginCustomerUI{Screens: []PluginCustomerScreen{{
			ID: "area", Title: "Área", Bundle: "customer-ui/index.html", AllowedOrigins: []string{"https://cdn.example.com/x"}}}}, true, "allowed_origins"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := ValidatePluginCustomerUI(tc.ui, tc.hasAPI)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("esperava recusa com %q, veio %v", tc.want, err)
			}
		})
	}
}

func TestValidatePluginCustomerUI_RejectsTooMany(t *testing.T) {
	var screens []PluginCustomerScreen
	for i := 0; i <= MaxCustomerScreens; i++ {
		screens = append(screens, customerScreen("tela-"+string(rune('a'+i))))
	}
	if err := ValidatePluginCustomerUI(PluginCustomerUI{Screens: screens}, true); err == nil {
		t.Fatal("esperava recusa acima do tecto")
	}
}

func TestVerifyPluginManifest_ChecksCustomerUI(t *testing.T) {
	m := PluginManifest{ID: "area-teste", Version: "1.0.0"}
	m.CustomerUI.Screens = []PluginCustomerScreen{customerScreen("area")}
	if err := VerifyPluginManifest(m, ""); err == nil {
		t.Fatal("customer_ui sem customer_api tinha de ser recusado")
	}
	m.CustomerAPI = []PluginCustomerAction{{ID: "eu", Methods: []string{"GET"}}}
	if err := VerifyPluginManifest(m, ""); err != nil {
		t.Fatal(err)
	}
}
