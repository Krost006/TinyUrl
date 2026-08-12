package registration

import (
	"context"
	"errors"
	"tinyURL/internal/models"
	hrefrepo "tinyURL/internal/models/hrefRepo"
	"tinyURL/internal/models/repo"
	userrepo "tinyURL/internal/models/userRepo"
	"tinyURL/internal/services/auth"
	"tinyURL/internal/validate"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// ErrUserExists — имя или email уже заняты.
var ErrUserExists = errors.New("user with this name or email already exists")

type Service struct {
	pool       *pgxpool.Pool // чтобы звать Begin
	users      userrepo.UserRepo
	hrefs      hrefrepo.HrefRepo
	auth       *auth.Service // для выдачи токена
	bcryptCost int
}

// Option настраивает Service при создании.
type Option func(*Service)

// WithBcryptCost задаёт стоимость bcrypt. Нужен в основном тестам: на боевой
// стоимости каждая регистрация обходится примерно в 100 мс.
func WithBcryptCost(cost int) Option {
	return func(s *Service) {
		if cost >= bcrypt.MinCost && cost <= bcrypt.MaxCost {
			s.bcryptCost = cost
		}
	}
}

func New(pool *pgxpool.Pool, users userrepo.UserRepo, hrefs hrefrepo.HrefRepo, auth *auth.Service, opts ...Option) *Service {
	s := &Service{
		pool:       pool,
		users:      users,
		hrefs:      hrefs,
		auth:       auth,
		bcryptCost: bcrypt.DefaultCost,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *Service) Register(ctx context.Context, name, email, password string) (*models.User, string, error) {
	name, err := validate.Name(name)
	if err != nil {
		return nil, "", err
	}

	email, err = validate.Email(email)
	if err != nil {
		return nil, "", err
	}

	if err := validate.Password(password); err != nil {
		return nil, "", err
	}

	// Хешируем до Begin: bcrypt занимает около 100 мс, и держать всё это время
	// открытую транзакцию значит зря занимать соединение из пула.
	pass, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return nil, "", err
	}

	user := models.User{
		Name:     name,
		Password: string(pass),
		Email:    email,
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, "", err
	}

	defer tx.Rollback(ctx)

	id, err := s.users.WithTx(tx).CreateUser(ctx, &user)
	if err != nil {
		if errors.Is(err, repo.ErrAlreadyExists) {
			return nil, "", ErrUserExists
		}
		return nil, "", err
	}

	err = createSlots(ctx, s.hrefs.WithTx(tx), id)
	if err != nil {
		return nil, "", err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, "", err
	}

	token, err := s.auth.GenerateToken(id, name)
	if err != nil {
		return nil, "", err
	}

	user.ID = id
	user.Password = ""

	return &user, token, nil
}
