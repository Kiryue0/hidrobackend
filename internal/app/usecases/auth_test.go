package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/kiryue0/hidrobackend/internal/app/apperr"
	"github.com/kiryue0/hidrobackend/internal/domain/user"
)

// --- fakes ---

type fakeUserRepo struct {
	byUsername map[string]*user.User
	byEmail    map[string]*user.User
	nextID     int64
	conflict   bool
}

func newFakeRepo() *fakeUserRepo {
	return &fakeUserRepo{byUsername: map[string]*user.User{}, byEmail: map[string]*user.User{}}
}

func (f *fakeUserRepo) Create(_ context.Context, u *user.User) error {
	if f.conflict {
		return apperr.ErrConflict
	}
	f.nextID++
	u.SetID(f.nextID)
	f.byUsername[u.Username()] = u
	f.byEmail[u.Email()] = u
	return nil
}
func (f *fakeUserRepo) GetByID(_ context.Context, id int64) (*user.User, error) {
	for _, u := range f.byUsername {
		if u.ID() == id {
			return u, nil
		}
	}
	return nil, apperr.ErrNotFound
}
func (f *fakeUserRepo) GetByUsername(_ context.Context, un string) (*user.User, error) {
	if u, ok := f.byUsername[un]; ok {
		return u, nil
	}
	return nil, apperr.ErrNotFound
}
func (f *fakeUserRepo) GetByEmail(_ context.Context, em string) (*user.User, error) {
	if u, ok := f.byEmail[em]; ok {
		return u, nil
	}
	return nil, apperr.ErrNotFound
}
func (f *fakeUserRepo) Delete(_ context.Context, id int64) (int64, error) {
	for k, u := range f.byUsername {
		if u.ID() == id {
			delete(f.byUsername, k)
			delete(f.byEmail, u.Email())
			return 1, nil
		}
	}
	return 0, apperr.ErrNotFound
}

// plain-text hasher (test): "hash:"+pw
type fakeHasher struct{}

func (fakeHasher) Hash(p string) (string, error) { return "hash:" + p, nil }
func (fakeHasher) Compare(h, p string) bool      { return h == "hash:"+p }

type fakeTokens struct{}

func (fakeTokens) Issue(id int64) (string, error) { return "tok", nil }
func (fakeTokens) Parse(string) (int64, error)    { return 0, nil }

func newService(repo *fakeUserRepo) *AuthService {
	return NewAuthService(repo, newFakeCabinRepo(), fakeHasher{}, fakeTokens{})
}

// --- tests ---

func TestRegister_OK(t *testing.T) {
	repo := newFakeRepo()
	s := newService(repo)
	u, err := s.Register(context.Background(), RegisterInput{Username: "melih", Email: "m@x.com", Password: "secret12"})
	if err != nil {
		t.Fatalf("kayıt başarılı olmalı: %v", err)
	}
	if u.ID() == 0 {
		t.Fatal("ID atanmalı")
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	s := newService(newFakeRepo())
	_, err := s.Register(context.Background(), RegisterInput{Username: "melih", Email: "m@x.com", Password: "short"})
	if !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("validation hatası bekleniyordu: %v", err)
	}
}

func TestRegister_Conflict(t *testing.T) {
	repo := newFakeRepo()
	repo.conflict = true
	s := newService(repo)
	_, err := s.Register(context.Background(), RegisterInput{Username: "melih", Email: "m@x.com", Password: "secret12"})
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("conflict bekleniyordu: %v", err)
	}
}

func TestLogin_OK_ByUsername(t *testing.T) {
	repo := newFakeRepo()
	s := newService(repo)
	_, _ = s.Register(context.Background(), RegisterInput{Username: "melih", Email: "m@x.com", Password: "secret12"})

	tok, err := s.Login(context.Background(), LoginInput{Identifier: "melih", Password: "secret12"})
	if err != nil || tok != "tok" {
		t.Fatalf("giriş başarılı olmalı: tok=%q err=%v", tok, err)
	}
}

func TestLogin_OK_ByEmail(t *testing.T) {
	repo := newFakeRepo()
	s := newService(repo)
	_, _ = s.Register(context.Background(), RegisterInput{Username: "melih", Email: "m@x.com", Password: "secret12"})

	if _, err := s.Login(context.Background(), LoginInput{Identifier: "M@X.com", Password: "secret12"}); err != nil {
		t.Fatalf("e-posta ile giriş başarılı olmalı: %v", err)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	repo := newFakeRepo()
	s := newService(repo)
	_, _ = s.Register(context.Background(), RegisterInput{Username: "melih", Email: "m@x.com", Password: "secret12"})

	_, err := s.Login(context.Background(), LoginInput{Identifier: "melih", Password: "yanlis123"})
	if !errors.Is(err, apperr.ErrInvalidCredentials) {
		t.Fatalf("geçersiz kimlik bekleniyordu: %v", err)
	}
}

func TestLogin_UnknownUser_NoLeak(t *testing.T) {
	s := newService(newFakeRepo())
	_, err := s.Login(context.Background(), LoginInput{Identifier: "yok", Password: "secret12"})
	if !errors.Is(err, apperr.ErrInvalidCredentials) {
		t.Fatalf("bilinmeyen kullanıcı ErrInvalidCredentials dönmeli (sızıntı yok): %v", err)
	}
}
