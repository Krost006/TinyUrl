package clickrepo

import (
	"context"

	"tinyURL/internal/models"
	"tinyURL/internal/models/repo"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ClickRepo interface {
	// CreateClick фиксирует переход по ссылке.
	//
	// Время берётся из click.Time, а не из now() в базе: между переходом и
	// записью проходит время, и оно будет только расти, когда запись станет
	// асинхронной. При батчинге now() дал бы всей пачке один момент вместо
	// реальных времён переходов.
	CreateClick(ctx context.Context, click *models.Click) error

	// ListByHref возвращает переходы по ссылке, новые первыми.
	// Проверка владельца — не его дело: репозиторий не знает про пользователей,
	// права проверяет сервис перед вызовом.
	ListByHref(ctx context.Context, hrefID int) ([]models.Click, error)

	// CountByHref возвращает число переходов — для списка ссылок, где вся
	// статистика не нужна, а счётчик показать надо.
	CountByHref(ctx context.Context, hrefID int) (int, error)

	// WithTx возвращает тот же репозиторий, но выполняющий запросы внутри
	// переданной транзакции.
	WithTx(tx pgx.Tx) ClickRepo
}

func NewClickRepo(pool *pgxpool.Pool) ClickRepo {
	return &pgxClickRepo{ex: pool}
}

type pgxClickRepo struct {
	ex repo.Executor
}

func (r *pgxClickRepo) WithTx(tx pgx.Tx) ClickRepo {
	return &pgxClickRepo{ex: tx}
}

const clickColumns = `id, href_id, ip, country, time`

func (r *pgxClickRepo) CreateClick(ctx context.Context, click *models.Click) error {
	_, err := r.ex.Exec(ctx, `
		INSERT INTO click (href_id, ip, country, time)
		VALUES ($1, $2, $3, $4);`,
		click.HrefID, click.IP, click.Country, click.Time)

	return err
}

func (r *pgxClickRepo) ListByHref(ctx context.Context, hrefID int) ([]models.Click, error) {
	rows, err := r.ex.Query(ctx, `
		SELECT `+clickColumns+`
		FROM click
		WHERE href_id=$1
		ORDER BY time DESC;`,
		hrefID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	// Пустой срез, а не nil: в JSON он даст [], а nil — null.
	clicks := []models.Click{}

	for rows.Next() {
		click := models.Click{}
		err = rows.Scan(&click.ID, &click.HrefID, &click.IP, &click.Country, &click.Time)
		if err != nil {
			return nil, err
		}

		clicks = append(clicks, click)
	}

	// rows.Err() обязателен: обрыв соединения посреди выборки не виден по Next().
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return clicks, nil
}

func (r *pgxClickRepo) CountByHref(ctx context.Context, hrefID int) (int, error) {
	var count int

	err := r.ex.QueryRow(ctx, `
		SELECT count(*)
		FROM click
		WHERE href_id=$1;`,
		hrefID,
	).Scan(&count)

	return count, err
}
