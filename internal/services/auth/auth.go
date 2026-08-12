// Package auth отвечает за авторизацию и аутентификацию: проверяет пароли
// и выдаёт JWT-токены. Создание пользователя живёт в пакете registration.
package auth

import (
	"context"
	"errors"
	"time"

	"tinyURL/internal/models"
	"tinyURL/internal/models/repo"
	userrepo "tinyURL/internal/models/userRepo"
	"tinyURL/internal/validate"

	"golang.org/x/crypto/bcrypt"
)

const defaultTokenTTL = 24 * time.Hour

// Service — сервис пользователей.
type Service struct {
	repo       userrepo.UserRepo
	secretKey  []byte
	tokenTTL   time.Duration
	bcryptCost int
	now        func() time.Time // подменяется в тестах
}

// Option настраивает Service при создании.
type Option func(*Service)

// WithTokenTTL задаёт срок жизни токена вместо суток по умолчанию.
func WithTokenTTL(ttl time.Duration) Option {
	return func(s *Service) {
		if ttl > 0 {
			s.tokenTTL = ttl
		}
	}
}

// WithBcryptCost задаёт стоимость bcrypt. Нужен в основном тестам,
// чтобы не тратить на хеширование сотни миллисекунд.
func WithBcryptCost(cost int) Option {
	return func(s *Service) {
		if cost >= bcrypt.MinCost && cost <= bcrypt.MaxCost {
			s.bcryptCost = cost
		}
	}
}

// WithClock подменяет источник времени.
func WithClock(now func() time.Time) Option {
	return func(s *Service) {
		if now != nil {
			s.now = now
		}
	}
}

// New создаёт сервис пользователей. Пустой secretKey — ошибка конфигурации:
// без него подпись токенов не имеет смысла.
func New(repo userrepo.UserRepo, secretKey string, opts ...Option) (*Service, error) {
	if secretKey == "" {
		return nil, ErrEmptySecretKey
	}

	s := &Service{
		repo:       repo,
		secretKey:  []byte(secretKey),
		tokenTTL:   defaultTokenTTL,
		bcryptCost: bcrypt.DefaultCost,
		now:        time.Now,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

// Login проверяет учётные данные и возвращает токен. В качестве логина
// принимается имя пользователя или email.
func (s *Service) Login(ctx context.Context, login, password string) (*models.User, string, error) {
	user, err := s.findByLogin(ctx, login)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			// Всё равно считаем bcrypt, чтобы время ответа не выдавало,
			// существует ли такой пользователь.
			bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
			return nil, "", ErrInvalidCredentials
		}
		return nil, "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, "", ErrInvalidCredentials
	}

	token, err := s.GenerateToken(user.ID, user.Name)
	if err != nil {
		return nil, "", err
	}

	user.Password = ""

	return user, token, nil
}

// Authenticate проверяет токен и возвращает пользователя, которому он выдан.
// Используется middleware при каждом запросе к защищённым ручкам.
func (s *Service) Authenticate(ctx context.Context, token string) (*models.User, error) {
	claims, err := s.ParseToken(token)
	if err != nil {
		return nil, err
	}

	id, err := claims.UserID()
	if err != nil {
		return nil, ErrInvalidToken
	}

	user, err := s.repo.GetUserById(ctx, id)
	if err != nil {
		// Пользователь мог быть удалён уже после выдачи токена.
		if errors.Is(err, repo.ErrNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}

	user.Password = ""

	return user, nil
}

// findByLogin ищет пользователя по email, если логин похож на адрес, иначе по имени.
func (s *Service) findByLogin(ctx context.Context, login string) (*models.User, error) {
	if email, err := validate.Email(login); err == nil {
		return s.repo.GetUserByEmail(ctx, email)
	}

	name, err := validate.Name(login)
	if err != nil {
		// Логин не проходит даже базовую проверку — такого пользователя
		// заведомо нет, в базу идти незачем.
		return nil, repo.ErrNotFound
	}

	return s.repo.GetUserByName(ctx, name)
}

// dummyHash — валидный bcrypt-хеш от случайной строки. Нужен только для того,
// чтобы сравнение занимало столько же времени, сколько и для реального пользователя.
var dummyHash = []byte("$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy")
