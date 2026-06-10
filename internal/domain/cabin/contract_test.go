package cabin

import (
	"encoding/json"
	"testing"
)

// Bu test, Thresholds ve DecisionConfig'in JSON alan adlarının firmware
// down/config kontratıyla (backend_plan.md Bölüm 2.3/2.4) BİREBİR eşleştiğini
// garanti eder. Bir alan adı kazara değişirse firmware'i bozmadan önce burada yakalanır.
func TestThresholdsJSONContract(t *testing.T) {
	b, err := json.Marshal(DefaultThresholds())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := []string{
		"sicaklikMin", "sicaklikMax", "sicaklikKritikMax",
		"nemMin", "nemMax", "nemKritikMax",
		"tdsMin", "tdsMax", "phMin", "phMax",
	}
	assertExactKeys(t, "thresholds", m, want)
}

func TestDecisionConfigJSONContract(t *testing.T) {
	b, err := json.Marshal(DefaultDecisionConfig())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := []string{
		"otomatikMod",
		"fanOrtaEsik", "fanTamEsik", "ledKisEsik", "ledKapatEsik",
		"fanBazHiz", "fanOrtaHiz", "fanTamHiz", "ledKisikDuty",
		"histerezisC", "histerezisNem",
		"havaMotoruMod", "havaAcikSn", "havaKapaliSn",
	}
	assertExactKeys(t, "decision", m, want)
}

func assertExactKeys(t *testing.T, name string, got map[string]json.RawMessage, want []string) {
	t.Helper()
	wantSet := make(map[string]bool, len(want))
	for _, k := range want {
		wantSet[k] = true
		if _, ok := got[k]; !ok {
			t.Errorf("%s: zorunlu JSON alanı eksik: %q", name, k)
		}
	}
	for k := range got {
		if !wantSet[k] {
			t.Errorf("%s: beklenmeyen JSON alanı (kontrat dışı): %q", name, k)
		}
	}
}
