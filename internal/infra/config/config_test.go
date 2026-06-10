package config

import "testing"

// trimQuotes: panel kopyala-yapıştır kalıntıları (boşluk, satır sonu, tırnak)
// URL parse'ı sessizce bozuyordu (paho "no servers defined to connect to").
func TestTrimQuotes(t *testing.T) {
	cases := map[string]string{
		"tls://h:8883":         "tls://h:8883",
		" tls://h:8883 ":       "tls://h:8883",
		"\"tls://h:8883\"":     "tls://h:8883",
		"'tls://h:8883'":       "tls://h:8883",
		" \"tls://h:8883\" ":   "tls://h:8883",
		"\" 'tls://h:8883' \"": "tls://h:8883",
		"tls://h:8883\n":       "tls://h:8883",
		"":                     "",
		"\"":                   "\"",
		"\"\"":                 "",
	}
	for in, want := range cases {
		if got := trimQuotes(in); got != want {
			t.Errorf("trimQuotes(%q) = %q, beklenen %q", in, got, want)
		}
	}
}
