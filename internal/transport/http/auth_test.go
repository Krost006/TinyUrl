package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tinyURL/internal/models"
	"tinyURL/internal/models/repo"
	"tinyURL/internal/services/auth"

	"golang.org/x/crypto/bcrypt"
)

type memRepo struct {
	users  map[int]*models.User
	nextID int
}

func newMemRepo() *memRepo {
	return &memRepo{users: map[int]*models.User{}, nextID: 1}
}

func (r *memRepo) CreateUser(_ context.Context, u *models.User) (int, error) {
	for _, e := range r.users {
		if e.Name == u.Name || e.Email == u.Email {
			return 0, repo.ErrAlreadyExists
		}
	}

	id := r.nextID
	r.nextID++

	stored := *u
	stored.ID = id
	r.users[id] = &stored

	return id, nil
}

func (r *memRepo) GetUserById(_ context.Context, id int) (*models.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, repo.ErrNotFound
	}

	copied := *u

	return &copied, nil
}

func (r *memRepo) GetUserByName(_ context.Context, name string) (*models.User, error) {
	return r.find(func(u *models.User) bool { return u.Name == name })
}

func (r *memRepo) GetUserByEmail(_ context.Context, email string) (*models.User, error) {
	return r.find(func(u *models.User) bool { return u.Email == email })
}

func (r *memRepo) find(match func(*models.User) bool) (*models.User, error) {
	for _, u := range r.users {
		if match(u) {
			copied := *u
			return &copied, nil
		}
	}

	return nil, repo.ErrNotFound
}

func newTestServer(t *testing.T) http.Handler {
	t.Helper()

	service, err := auth.New(newMemRepo(), "test-secret", auth.WithBcryptCost(bcrypt.MinCost))
	if err != nil {
		t.Fatalf("auth.New: %v", err)
	}

	mux := http.NewServeMux()
	NewAuthHandler(service).Routes(mux)

	return mux
}

func do(t *testing.T, h http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func TestRegisterAndLoginFlow(t *testing.T) {
	h := newTestServer(t)

	rec := do(t, h, http.MethodPost, "/api/auth/register",
		`{"name":"krost","email":"krost@example.com","password":"password123"}`, "")

	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body = %s", rec.Code, rec.Body)
	}

	var registered authResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &registered); err != nil {
		t.Fatalf("decode register response: %v", err)
	}

	if registered.Token == "" {
		t.Fatal("register returned no token")
	}

	// Ответ не должен содержать пароль ни в каком виде.
	if strings.Contains(rec.Body.String(), "password") {
		t.Errorf("register response mentions password: %s", rec.Body)
	}

	rec = do(t, h, http.MethodPost, "/api/auth/login",
		`{"login":"krost","password":"password123"}`, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", rec.Code, rec.Body)
	}

	var loggedIn authResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &loggedIn); err != nil {
		t.Fatalf("decode login response: %v", err)
	}

	rec = do(t, h, http.MethodGet, "/api/auth/me", "", loggedIn.Token)
	if rec.Code != http.StatusOK {
		t.Fatalf("me status = %d, body = %s", rec.Code, rec.Body)
	}

	var me userResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &me); err != nil {
		t.Fatalf("decode me response: %v", err)
	}

	if me.Name != "krost" || me.Email != "krost@example.com" {
		t.Errorf("me = %+v", me)
	}
}

func TestStatusCodes(t *testing.T) {
	h := newTestServer(t)

	// Готовим существующего пользователя.
	do(t, h, http.MethodPost, "/api/auth/register",
		`{"name":"krost","email":"krost@example.com","password":"password123"}`, "")

	tests := []struct {
		name         string
		method, path string
		body, token  string
		want         int
	}{
		{"duplicate user", http.MethodPost, "/api/auth/register",
			`{"name":"krost","email":"krost@example.com","password":"password123"}`, "", http.StatusConflict},
		{"invalid email", http.MethodPost, "/api/auth/register",
			`{"name":"other","email":"nope","password":"password123"}`, "", http.StatusBadRequest},
		{"short password", http.MethodPost, "/api/auth/register",
			`{"name":"other","email":"o@example.com","password":"x"}`, "", http.StatusBadRequest},
		{"malformed json", http.MethodPost, "/api/auth/register", `{"name":`, "", http.StatusBadRequest},
		{"unknown field", http.MethodPost, "/api/auth/login",
			`{"login":"krost","password":"password123","admin":true}`, "", http.StatusBadRequest},
		{"wrong password", http.MethodPost, "/api/auth/login",
			`{"login":"krost","password":"nope-nope-nope"}`, "", http.StatusUnauthorized},
		{"unknown user", http.MethodPost, "/api/auth/login",
			`{"login":"ghost","password":"password123"}`, "", http.StatusUnauthorized},
		{"me without token", http.MethodGet, "/api/auth/me", "", "", http.StatusUnauthorized},
		{"me with garbage token", http.MethodGet, "/api/auth/me", "", "abc.def.ghi", http.StatusUnauthorized},
		{"wrong method", http.MethodGet, "/api/auth/login", "", "", http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, h, tt.method, tt.path, tt.body, tt.token)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tt.want, rec.Body)
			}
		})
	}
}

func TestRequireAuthRejectsNonBearerScheme(t *testing.T) {
	h := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Basic a3Jvc3Q6cGFzcw==")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
