package http

import (
	"testing"

	userrepo "tinyURL/internal/models/userRepo"
	"tinyURL/internal/services/auth"

	"golang.org/x/crypto/bcrypt"
)

func newAuthService(t *testing.T, users userrepo.UserRepo) (*auth.Service, error) {
	t.Helper()
	s, err := auth.New(users, "test-secret", auth.WithBcryptCost(bcrypt.MinCost))
	if err != nil {
		t.Fatalf("auth.New: %v", err)
	}
	return s, nil
}

func mustToken(t *testing.T, s *auth.Service, id int, name string) string {
	t.Helper()
	tok, err := s.GenerateToken(id, name)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	return tok
}
