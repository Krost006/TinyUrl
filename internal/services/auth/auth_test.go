package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"tinyURL/internal/models"
	"tinyURL/internal/models/repo"
	userrepo "tinyURL/internal/models/userRepo"

	"golang.org/x/crypto/bcrypt"
)

// memRepo — реализация UserRepo в памяти, повторяющая UNIQUE-ограничения схемы.
type memRepo struct {
	users  map[int]*models.User
	nextID int
	err    error // если задана, возвращается из всех методов
}

func newMemRepo() *memRepo {
	return &memRepo{users: map[int]*models.User{}, nextID: 1}
}

func (r *memRepo) CreateUser(_ context.Context, u *models.User) (int, error) {
	if r.err != nil {
		return 0, r.err
	}

	for _, existing := range r.users {
		if existing.Name == u.Name || existing.Email == u.Email {
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
	if r.err != nil {
		return nil, r.err
	}

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
	if r.err != nil {
		return nil, r.err
	}

	for _, u := range r.users {
		if match(u) {
			copied := *u
			return &copied, nil
		}
	}

	return nil, repo.ErrNotFound
}

func newTestService(t *testing.T, repo userrepo.UserRepo, opts ...Option) *Service {
	t.Helper()

	// MinCost — тесты не должны платить за реальную стоимость bcrypt.
	opts = append([]Option{WithBcryptCost(bcrypt.MinCost)}, opts...)

	s, err := New(repo, "test-secret", opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	return s
}

func TestNewRequiresSecret(t *testing.T) {
	if _, err := New(newMemRepo(), ""); !errors.Is(err, ErrEmptySecretKey) {
		t.Fatalf("got %v, want ErrEmptySecretKey", err)
	}
}

func TestRegisterStoresHashedPassword(t *testing.T) {
	repo := newMemRepo()
	s := newTestService(t, repo)

	user, token, err := s.Register(context.Background(), "krost", "KROST@example.com", "password123")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if user.ID == 0 {
		t.Error("user ID was not set from repo")
	}

	if token == "" {
		t.Error("expected a token")
	}

	if user.Password != "" {
		t.Errorf("password hash leaked in returned user: %q", user.Password)
	}

	// email нормализуется к нижнему регистру
	if user.Email != "krost@example.com" {
		t.Errorf("email = %q, want krost@example.com", user.Email)
	}

	stored := repo.users[user.ID]
	if stored.Password == "password123" {
		t.Fatal("password stored in plaintext")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(stored.Password), []byte("password123")); err != nil {
		t.Fatalf("stored hash does not match original password: %v", err)
	}
}

func TestRegisterValidation(t *testing.T) {
	tests := []struct {
		name        string
		user, email string
		password    string
		want        error
	}{
		{"empty name", "", "a@example.com", "password123", ErrEmptyName},
		{"short name", "ab", "a@example.com", "password123", ErrShortName},
		{"long name", strings.Repeat("a", 33), "a@example.com", "password123", ErrLongName},
		{"bad email", "krost", "not-an-email", "password123", ErrInvalidEmail},
		{"empty email", "krost", "", "password123", ErrInvalidEmail},
		{"short password", "krost", "a@example.com", "short", ErrShortPassword},
		{"long password", "krost", "a@example.com", strings.Repeat("x", 73), ErrLongPassword},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestService(t, newMemRepo())

			_, _, err := s.Register(context.Background(), tt.user, tt.email, tt.password)
			if !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
		})
	}
}

func TestRegisterDuplicate(t *testing.T) {
	s := newTestService(t, newMemRepo())
	ctx := context.Background()

	if _, _, err := s.Register(ctx, "krost", "krost@example.com", "password123"); err != nil {
		t.Fatalf("first Register: %v", err)
	}

	_, _, err := s.Register(ctx, "krost", "other@example.com", "password123")
	if !errors.Is(err, ErrUserExists) {
		t.Fatalf("got %v, want ErrUserExists", err)
	}
}

func TestLogin(t *testing.T) {
	s := newTestService(t, newMemRepo())
	ctx := context.Background()

	if _, _, err := s.Register(ctx, "krost", "krost@example.com", "password123"); err != nil {
		t.Fatalf("Register: %v", err)
	}

	t.Run("by name", func(t *testing.T) {
		user, token, err := s.Login(ctx, "krost", "password123")
		if err != nil {
			t.Fatalf("Login: %v", err)
		}
		if token == "" {
			t.Error("expected a token")
		}
		if user.Password != "" {
			t.Error("password hash leaked in returned user")
		}
	})

	t.Run("by email", func(t *testing.T) {
		if _, _, err := s.Login(ctx, "KROST@example.com", "password123"); err != nil {
			t.Fatalf("Login by email: %v", err)
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		_, _, err := s.Login(ctx, "krost", "wrong-password")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("got %v, want ErrInvalidCredentials", err)
		}
	})

	t.Run("unknown user", func(t *testing.T) {
		_, _, err := s.Login(ctx, "nobody", "password123")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("got %v, want ErrInvalidCredentials", err)
		}
	})

	t.Run("repo failure is not masked as bad credentials", func(t *testing.T) {
		broken := newMemRepo()
		broken.err = errors.New("connection refused")

		_, _, err := newTestService(t, broken).Login(ctx, "krost", "password123")
		if err == nil || errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("got %v, want the underlying repo error", err)
		}
	})
}

func TestAuthenticate(t *testing.T) {
	repo := newMemRepo()
	s := newTestService(t, repo)
	ctx := context.Background()

	user, token, err := s.Register(ctx, "krost", "krost@example.com", "password123")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	t.Run("valid token", func(t *testing.T) {
		got, err := s.Authenticate(ctx, token)
		if err != nil {
			t.Fatalf("Authenticate: %v", err)
		}
		if got.ID != user.ID {
			t.Errorf("ID = %d, want %d", got.ID, user.ID)
		}
		if got.Password != "" {
			t.Error("password hash leaked in returned user")
		}
	})

	t.Run("garbage token", func(t *testing.T) {
		if _, err := s.Authenticate(ctx, "not.a.token"); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("got %v, want ErrInvalidToken", err)
		}
	})

	t.Run("token signed with another key", func(t *testing.T) {
		other := newTestService(t, repo)
		other.secretKey = []byte("different-secret")

		foreign, err := other.GenerateToken(user.ID, user.Name)
		if err != nil {
			t.Fatalf("GenerateToken: %v", err)
		}

		if _, err := s.Authenticate(ctx, foreign); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("got %v, want ErrInvalidToken", err)
		}
	})

	t.Run("expired token", func(t *testing.T) {
		past := time.Now().Add(-2 * time.Hour)
		expiring := newTestService(t, repo,
			WithTokenTTL(time.Hour),
			WithClock(func() time.Time { return past }),
		)

		stale, err := expiring.GenerateToken(user.ID, user.Name)
		if err != nil {
			t.Fatalf("GenerateToken: %v", err)
		}

		if _, err := s.Authenticate(ctx, stale); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("got %v, want ErrInvalidToken", err)
		}
	})

	t.Run("deleted user", func(t *testing.T) {
		delete(repo.users, user.ID)
		defer func() { repo.users[user.ID] = &models.User{ID: user.ID, Name: user.Name} }()

		if _, err := s.Authenticate(ctx, token); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("got %v, want ErrInvalidToken", err)
		}
	})
}

func TestParseTokenRejectsNoneAlgorithm(t *testing.T) {
	s := newTestService(t, newMemRepo())

	// Заголовок {"alg":"none","typ":"JWT"} и claims с subject=1, без подписи.
	const unsigned = "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." +
		"eyJzdWIiOiIxIiwiZXhwIjo0MTAyNDQ0ODAwfQ."

	if _, err := s.ParseToken(unsigned); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("got %v, want ErrInvalidToken", err)
	}
}
