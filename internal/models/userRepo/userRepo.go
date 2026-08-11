package userrepo

import (
	"context"
	"errors"

	"tinyURL/internal/models"
	"tinyURL/internal/models/repo"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// uniqueViolation — код ошибки PostgreSQL при нарушении UNIQUE.
const uniqueViolation = "23505"

type UserRepo interface {
	GetUserById(ctx context.Context, ID int) (*models.User, error)
	GetUserByName(ctx context.Context, name string) (*models.User, error)
	GetUserByEmail(ctx context.Context, mail string) (*models.User, error)
	CreateUser(ctx context.Context, user *models.User) (int, error)
}

func NewUserRepo(pool *pgxpool.Pool) UserRepo {
	return &pgxUserRepo{pool}
}

type pgxUserRepo struct {
	pool *pgxpool.Pool
}

const userColumns = `id, name, password, email`

func (r *pgxUserRepo) GetUserById(ctx context.Context, ID int) (*models.User, error) {
	return r.getUserBy(ctx, `id`, ID)
}

func (r *pgxUserRepo) GetUserByName(ctx context.Context, name string) (*models.User, error) {
	return r.getUserBy(ctx, `name`, name)
}

func (r *pgxUserRepo) GetUserByEmail(ctx context.Context, mail string) (*models.User, error) {
	return r.getUserBy(ctx, `email`, mail)
}

// getUserBy читает одного пользователя по указанной колонке. Имя колонки
// приходит только из констант этого файла, значение — всегда через плейсхолдер.
func (r *pgxUserRepo) getUserBy(ctx context.Context, column string, value any) (*models.User, error) {
	u := models.User{}

	err := r.pool.QueryRow(ctx, `
		SELECT `+userColumns+`
		FROM users
		WHERE `+column+`=$1;`,
		value,
	).Scan(&u.ID, &u.Name, &u.Password, &u.Email)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &u, nil
}

func (r *pgxUserRepo) CreateUser(ctx context.Context, user *models.User) (int, error) {
	var id int

	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (name, password, email)
		VALUES ($1, $2, $3)
		RETURNING id;`,
		user.Name, user.Password, user.Email,
	).Scan(&id)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return 0, repo.ErrAlreadyExists
		}
		return 0, err
	}

	return id, nil
}
