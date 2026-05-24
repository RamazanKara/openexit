package inventory

import (
	"encoding/json"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestInventoryRoundTripJSONAndYAML(t *testing.T) {
	inv := New("demo", "datadog", "datadoghq.eu", "test", time.Unix(0, 0))
	inv.Assets.Dashboards = []Dashboard{{
		ID:          "dash",
		Title:       "Dashboard",
		Widgets:     WidgetSummary{Total: 1},
		EvidenceRef: "evidence://datadog/dashboard/dash",
	}}
	inv.RecomputeSummary()
	jsonData, err := json.Marshal(inv)
	if err != nil {
		t.Fatal(err)
	}
	var fromJSON Inventory
	if err := json.Unmarshal(jsonData, &fromJSON); err != nil {
		t.Fatal(err)
	}
	yamlData, err := yaml.Marshal(&fromJSON)
	if err != nil {
		t.Fatal(err)
	}
	var fromYAML Inventory
	if err := yaml.Unmarshal(yamlData, &fromYAML); err != nil {
		t.Fatal(err)
	}
	if err := Validate(&fromYAML); err != nil {
		t.Fatal(err)
	}
	if fromYAML.Summary.Dashboards != 1 {
		t.Fatalf("summary lost in round trip: %+v", fromYAML.Summary)
	}
}

func TestRedactSecretLikeValues(t *testing.T) {
	out := RedactString(`{"api_key":"dd123456789012345678901234567890"}`)
	if ContainsSecret(out) {
		t.Fatalf("redaction left secret-like value: %s", out)
	}
}
