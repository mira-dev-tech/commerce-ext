package commerceext

import "testing"

func TestValidateCustomerAPI_RejectsUnknownMethod(t *testing.T) {
	err := ValidateCustomerAPI([]PluginCustomerAction{{
		ID: "journal", Methods: []string{"POST", "HEAD"},
	}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateCustomerAPI_AcceptsDeclaredVerbs(t *testing.T) {
	err := ValidateCustomerAPI([]PluginCustomerAction{{
		ID: "journal", Methods: []string{"get", "PATCH"},
	}})
	if err != nil {
		t.Fatal(err)
	}
}
