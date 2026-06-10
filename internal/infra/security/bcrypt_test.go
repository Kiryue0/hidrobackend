package security

import "testing"

func TestBcrypt_HashCompare(t *testing.T) {
	h := NewBcryptHasher(0) // DefaultCost
	hash, err := h.Hash("secret12")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash == "secret12" {
		t.Fatal("hash plaintext'e eşit olmamalı")
	}
	if !h.Compare(hash, "secret12") {
		t.Fatal("doğru parola eşleşmeliydi")
	}
	if h.Compare(hash, "yanlis123") {
		t.Fatal("yanlış parola eşleşmemeliydi")
	}
	if h.Compare("bozuk-hash", "secret12") {
		t.Fatal("geçersiz hash false dönmeliydi (panik değil)")
	}
}
