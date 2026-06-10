package mqtt

import "testing"

func TestParseUpTopic(t *testing.T) {
	cases := []struct {
		topic    string
		wantID   string
		wantType string
		wantOK   bool
	}{
		{"cabin/CAB-3778C4/up/sensors", "CAB-3778C4", "sensors", true},
		{"cabin/CAB-3778C4/up/state", "CAB-3778C4", "state", true},
		{"cabin/CAB-3778C4/up/heartbeat", "CAB-3778C4", "heartbeat", true},
		{"cabin/CAB-3778C4/up/status", "CAB-3778C4", "status", true},
		{"cabin/CAB-3778C4/up/alert", "CAB-3778C4", "alert", true},
		// kenar durumları
		{"cabin/CAB-3778C4/down/command", "", "", false},     // down yönü
		{"cabin/CAB-3778C4/up", "", "", false},               // eksik tip
		{"cabin/CAB-3778C4/up/sensors/extra", "", "", false}, // fazla segment
		{"cabin//up/sensors", "", "sensors", true},           // boş id (parse geçerli; use case reddeder)
		{"", "", "", false},
		{"foo/bar/baz/qux", "", "", false}, // yanlış prefix
	}
	for _, c := range cases {
		id, mt, ok := parseUpTopic(c.topic)
		if ok != c.wantOK {
			t.Errorf("%q: ok=%v, beklenen %v", c.topic, ok, c.wantOK)
			continue
		}
		if ok && (id != c.wantID || mt != c.wantType) {
			t.Errorf("%q: (%q,%q), beklenen (%q,%q)", c.topic, id, mt, c.wantID, c.wantType)
		}
	}
}

func TestTsOrNow(t *testing.T) {
	if got := tsOrNow(0); got.IsZero() {
		t.Fatal("ts=0 alış zamanına düşmeli (zero olmamalı)")
	}
	if got := tsOrNow(-5); got.IsZero() {
		t.Fatal("ts<0 alış zamanına düşmeli")
	}
	if got := tsOrNow(1718000000); got.Unix() != 1718000000 {
		t.Fatalf("ts>0 korunmalı, %d", got.Unix())
	}
}
