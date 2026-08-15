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
	userrepo "tinyURL/internal/models/userRepo"
	"tinyURL/internal/services/auth"

	"github.com/jackc/pgx/v5"
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

// WithTx для мока в памяти ничего не меняет: транзакций тут нет.
func (r *memRepo) WithTx(pgx.Tx) userrepo.UserRepo { return r }

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

// newTestServer поднимает роутер поверх моков и возвращает его вместе с
// репозиторием, чтобы тест мог завести пользователя напрямую.
//
// Ручка регистрации здесь не работает: registration.Service требует настоящий
// *pgxpool.Pool для Begin, а транзакцию мок не эмулирует. Регистрация
// проверяется интеграционно, на живой базе.
func newTestServer(t *testing.T) (http.Handler, *memRepo) {
	t.Helper()

	users := newMemRepo()

	service, err := auth.New(users, "test-secret", auth.WithBcryptCost(bcrypt.MinCost))
	if err != nil {
		t.Fatalf("auth.New: %v", err)
	}

	mux := http.NewServeMux()
	NewAuthHandler(service, nil, NewAuth(service)).Routes(mux)

	return mux, users
}

// addUser кладёт пользователя в репозиторий, минуя ручку регистрации.
func addUser(t *testing.T, r *memRepo, name, email, password string) {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	_, err = r.CreateUser(context.Background(), &models.User{
		Name:     name,
		Email:    email,
		Password: string(hash),
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
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

func TestLoginAndMeFlow(t *testing.T) {
	h, users := newTestServer(t)

	addUser(t, users, "krost", "krost@example.com", "password123")

	rec := do(t, h, http.MethodPost, "/api/auth/login",
		`{"login":"krost","password":"password123"}`, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", rec.Code, rec.Body)
	}

	// Ответ не должен содержать пароль ни в каком виде.
	if strings.Contains(rec.Body.String(), "password") {
		t.Errorf("login response mentions password: %s", rec.Body)
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
	h, users := newTestServer(t)

	addUser(t, users, "krost", "krost@example.com", "password123")

	tests := []struct {
		name         string
		method, path string
		body, token  string
		want         int
	}{
		{"malformed json", http.MethodPost, "/api/auth/login", `{"login":`, "", http.StatusBadRequest},
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
	h, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Basic a3Jvc3Q6cGFzcw==")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
