package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"tinyURL/internal/models"
	"tinyURL/internal/models/repo"
	userrepo "tinyURL/internal/models/userRepo"

	"github.com/jackc/pgx/v5"
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

// WithTx для мока в памяти ничего не меняет: транзакций тут нет,
// а откат в тестах не проверяется.
func (r *memRepo) WithTx(pgx.Tx) userrepo.UserRepo { return r }

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

// addUser кладёт в репозиторий пользователя с захешированным паролем.
// Регистрация живёт в пакете registration, поэтому тестам auth нужен
// собственный способ завести подопытного.
func addUser(t *testing.T, r *memRepo, name, email, password string) *models.User {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := &models.User{Name: name, Email: email, Password: string(hash)}

	id, err := r.CreateUser(context.Background(), user)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	user.ID = id

	return user
}

func TestNewRequiresSecret(t *testing.T) {
	if _, err := New(newMemRepo(), ""); !errors.Is(err, ErrEmptySecretKey) {
		t.Fatalf("got %v, want ErrEmptySecretKey", err)
	}
}

func TestLogin(t *testing.T) {
	repo := newMemRepo()
	s := newTestService(t, repo)
	ctx := context.Background()

	addUser(t, repo, "krost", "krost@example.com", "password123")

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

	user := addUser(t, repo, "krost", "krost@example.com", "password123")

	token, err := s.GenerateToken(user.ID, user.Name)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
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
