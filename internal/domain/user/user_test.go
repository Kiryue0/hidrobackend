package user

import "testing"

func TestNewUser_OK(t *testing.T) {
	u, err := NewUser("melih", "Melih@Example.com", "hash")
	if err != nil {
		t.Fatalf("geçerli kullanıcı: %v", err)
	}
	if u.Email() != "melih@example.com" {
		t.Fatalf("e-posta küçük harfe normalize edilmeli: %q", u.Email())
	}
}

func TestNewUser_Invalid(t *testing.T) {
	cases := []struct{ name, un, em, ph string }{
		{"kısa kullanıcı", "ab", "a@b.com", "h"},
		{"geçersiz karakter", "me lih", "a@b.com", "h"},
		{"geçersiz email", "melih", "not-an-email", "h"},
		{"boş hash", "melih", "a@b.com", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := NewUser(c.un, c.em, c.ph); err == nil {
				t.Fatal("hata bekleniyordu")
			}
		})
	}
}
