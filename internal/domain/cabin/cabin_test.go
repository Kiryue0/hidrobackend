package cabin

import (
	"testing"
	"time"
)

func mustID(t *testing.T, s string) CabinId {
	t.Helper()
	id, err := NewCabinId(s)
	if err != nil {
		t.Fatalf("geçerli id bekleniyordu: %v", err)
	}
	return id
}

func TestNewCabinId(t *testing.T) {
	ok := []string{"CAB-3778C4", "CAB-000000", "CAB-ABCDEF"}
	for _, s := range ok {
		if _, err := NewCabinId(s); err != nil {
			t.Errorf("%q geçerli olmalı: %v", s, err)
		}
	}
	bad := []string{"", "cab-3778c4", "CAB-3778C", "CAB-3778C45", "CAB-37 8C4", "3778C4", "CAB-37G8C4"}
	for _, s := range bad {
		if _, err := NewCabinId(s); err == nil {
			t.Errorf("%q geçersiz olmalı", s)
		}
	}
}

func TestNewCabin_OK_And_Invariant(t *testing.T) {
	c, err := NewCabin(mustID(t, "CAB-3778C4"), "Salon Kabini")
	if err != nil {
		t.Fatalf("kabin oluşmalı: %v", err)
	}
	if len(c.Sensors()) != 4 {
		t.Fatalf("4 zorunlu sensör bekleniyordu, %d", len(c.Sensors()))
	}
	if c.OwnerUserID() != nil {
		t.Fatal("yeni kabin claim'siz olmalı")
	}
}

func TestAssignOwner(t *testing.T) {
	c, _ := NewCabin(mustID(t, "CAB-3778C4"), "")
	if err := c.AssignOwner(7); err != nil {
		t.Fatalf("ilk atama başarılı olmalı: %v", err)
	}
	if !c.IsOwnedBy(7) {
		t.Fatal("sahip 7 olmalı")
	}
	if err := c.AssignOwner(7); err != nil {
		t.Fatal("aynı sahibe yeniden atama idempotent olmalı")
	}
	if err := c.AssignOwner(9); err == nil {
		t.Fatal("farklı sahibe atama reddedilmeli")
	}
}

func TestApplyActuatorState_FromDevice(t *testing.T) {
	c, _ := NewCabin(mustID(t, "CAB-3778C4"), "")
	st, err := NewActuatorState(ActFan1, false, 200, KaynakDecision)
	if err != nil {
		t.Fatalf("durum geçerli olmalı: %v", err)
	}
	if !st.On {
		t.Fatal("speed>0 ise On türetilmeli")
	}
	if err := c.ApplyActuatorState(st); err != nil {
		t.Fatalf("uygulanmalı: %v", err)
	}
	got := c.Actuators()[ActFan1]
	if got.Speed != 200 || got.Source != KaynakDecision {
		t.Fatalf("durum yanlış: %+v", got)
	}
}

func TestMarkOnlineOffline(t *testing.T) {
	c, _ := NewCabin(mustID(t, "CAB-3778C4"), "")
	now := time.Now()
	c.MarkOnline(now)
	if !c.Online() || c.LastSeen() == nil {
		t.Fatal("online + lastSeen set edilmeli")
	}
	c.MarkOffline()
	if c.Online() {
		t.Fatal("offline olmalı")
	}
}

// Erişimcilerin iç pointer'ları sızdırmadığını (aliasing) doğrular.
func TestAccessors_NoAliasLeak(t *testing.T) {
	c, _ := NewCabin(mustID(t, "CAB-3778C4"), "")
	c.MarkOnline(time.Now())
	if err := c.AssignOwner(7); err != nil {
		t.Fatalf("atama: %v", err)
	}

	ls := c.LastSeen()
	*ls = ls.Add(48 * time.Hour)
	if c.LastSeen().Equal(*ls) {
		t.Fatal("LastSeen iç state'i sızdırıyor (aliasing)")
	}

	o := c.OwnerUserID()
	*o = 999
	if c.IsOwnedBy(999) {
		t.Fatal("OwnerUserID iç state'i sızdırıyor (aliasing)")
	}
}

func TestReconstruct_CopiesActuators(t *testing.T) {
	in := map[ActuatorType]ActuatorState{
		ActFan1: {Type: ActFan1, On: true, Speed: 200, Source: KaynakDecision},
	}
	c := Reconstruct(mustID(t, "CAB-3778C4"), nil, "", DefaultThresholds(), DefaultDecisionConfig(), in, true, nil)
	// Çağıranın map'ini değiştir; aggregate etkilenmemeli.
	in[ActFan1] = ActuatorState{Type: ActFan1, On: false, Speed: 0, Source: KaynakButton}
	if c.Actuators()[ActFan1].Speed != 200 {
		t.Fatal("Reconstruct çağıranın map'ini paylaşıyor (aliasing)")
	}
}

func TestUpdateConfig_Validates(t *testing.T) {
	c, _ := NewCabin(mustID(t, "CAB-3778C4"), "")
	bad := DefaultThresholds()
	bad.SicaklikMin = 100
	if err := c.UpdateThresholds(bad); err == nil {
		t.Fatal("geçersiz eşik reddedilmeli")
	}
	if err := c.UpdateThresholds(DefaultThresholds()); err != nil {
		t.Fatalf("geçerli eşik kabul edilmeli: %v", err)
	}
}
