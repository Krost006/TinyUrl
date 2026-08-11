package hrefrepo

import (
	"context"
	"errors"

	"tinyURL/internal/models"
	"tinyURL/internal/models/repo"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type HrefRepo interface {
	// GetHrefById возвращает ссылку по её идентификатору.
	GetHrefById(ctx context.Context, ID int) (*models.Href, error)

	// GetHrefByURL возвращает ссылку по короткому коду. Владелец не проверяется:
	// редирект обязан работать для всех, а не только для автора ссылки.
	GetHrefByURL(ctx context.Context, url string) (*models.Href, error)

	// GetHrefByUserAndLongURL ищет ссылку этого пользователя на указанный
	// длинный адрес. Нужен, чтобы повторное сокращение того же URL не тратило
	// место из лимита. Ищем в паре с userID: короткие коды у пользователей
	// свои, поэтому один long_url встречается в таблице много раз.
	GetHrefByUserAndLongURL(ctx context.Context, userID int, longURL string) (*models.Href, error)

	// CreateHref создаёт ссылку и привязывает её к пользователю. Обе вставки
	// идут одной транзакцией. Возвращает ID созданной ссылки.
	CreateHref(ctx context.Context, userID int, href *models.Href) (int, error)

	// ListByUser возвращает ссылки пользователя, новые первыми.
	ListByUser(ctx context.Context, userID int) ([]models.Href, error)

	// CountByUser возвращает число ссылок пользователя — для проверки лимита.
	CountByUser(ctx context.Context, userID int) (int, error)

	// DeleteByUser удаляет ссылку, если она принадлежит этому пользователю.
	// Чужую ссылку удалить нельзя: вернётся repo.ErrNotFound.
	DeleteByUser(ctx context.Context, userID, hrefID int) error
}

func NewHrefRepo(pool *pgxpool.Pool) HrefRepo {
	return &pgxHrefRepo{pool}
}

type pgxHrefRepo struct {
	pool *pgxpool.Pool
}

// hrefColumns перечисляем явно: SELECT * сломается при добавлении колонки.
const hrefColumns = `id, url, long_url`

// uniqueViolation — код ошибки PostgreSQL при нарушении UNIQUE.
const uniqueViolation = "23505"

func (r *pgxHrefRepo) GetHrefById(ctx context.Context, ID int) (*models.Href, error) {
	return r.getHref(ctx, `
		SELECT `+hrefColumns+`
		FROM href
		WHERE id=$1;`,
		ID)
}

func (r *pgxHrefRepo) GetHrefByURL(ctx context.Context, url string) (*models.Href, error) {
	return r.getHref(ctx, `
		SELECT `+hrefColumns+`
		FROM href
		WHERE url=$1;`,
		url)
}

func (r *pgxHrefRepo) GetHrefByUserAndLongURL(ctx context.Context, userID int, longURL string) (*models.Href, error) {
	return r.getHref(ctx, `
		SELECT h.id, h.url, h.long_url
		FROM href h
		JOIN userhref uh ON uh.href_id = h.id
		WHERE uh.user_id=$1 AND h.long_url=$2;`,
		userID, longURL)
}

// getHref читает ровно одну ссылку. Отсутствие строки — это repo.ErrNotFound,
// а не пустая структура: иначе вызывающий код не отличит "нет ссылки" от "нашли".
func (r *pgxHrefRepo) getHref(ctx context.Context, query string, args ...any) (*models.Href, error) {
	h := models.Href{}

	err := r.pool.QueryRow(ctx, query, args...).Scan(&h.ID, &h.URL, &h.LongURL)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &h, nil
}

func (r *pgxHrefRepo) CreateHref(ctx context.Context, userID int, href *models.Href) (int, error) {
	// Транзакция: без неё упавшая вставка в userhref оставила бы ссылку без
	// владельца — она заняла бы короткий код, но не попала бы ни в чей список.
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}

	// Rollback после успешного Commit возвращает ErrTxClosed и ничего не делает,
	// поэтому defer безопасен и в удачном сценарии.
	defer tx.Rollback(ctx)

	var id int

	err = tx.QueryRow(ctx, `
		INSERT INTO href (url, long_url)
		VALUES ($1, $2)
		RETURNING id;`,
		href.URL, href.LongURL,
	).Scan(&id)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			// Короткий код занят — сервис должен сгенерировать другой.
			return 0, repo.ErrAlreadyExists
		}
		return 0, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO userhref (user_id, href_id)
		VALUES ($1, $2);`,
		userID, id)

	if err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	return id, nil
}

func (r *pgxHrefRepo) ListByUser(ctx context.Context, userID int) ([]models.Href, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT h.id, h.url, h.long_url
		FROM href h
		JOIN userhref uh ON uh.href_id = h.id
		WHERE uh.user_id=$1
		ORDER BY h.id DESC;`,
		userID)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	hrefs := []models.Href{}

	for rows.Next() {
		h := models.Href{}
		if err := rows.Scan(&h.ID, &h.URL, &h.LongURL); err != nil {
			return nil, err
		}

		hrefs = append(hrefs, h)
	}

	// rows.Err() обязателен: обрыв соединения посреди выборки не виден по Next().
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return hrefs, nil
}

func (r *pgxHrefRepo) CountByUser(ctx context.Context, userID int) (int, error) {
	var count int

	err := r.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM userhref
		WHERE user_id=$1;`,
		userID,
	).Scan(&count)

	return count, err
}

func (r *pgxHrefRepo) DeleteByUser(ctx context.Context, userID, hrefID int) error {
	// Удаляем саму href, а не только связь: короткий код должен освободиться.
	// Строки в userhref и click уходят каскадом (ON DELETE CASCADE).
	// Условие EXISTS не даёт удалить чужую ссылку.
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM href
		WHERE id=$1
		  AND EXISTS (
			SELECT 1 FROM userhref
			WHERE href_id=$1 AND user_id=$2
		  );`,
		hrefID, userID)

	if err != nil {
		return err
	}

	// Ноль удалённых строк — ссылки нет или она чужая. Наружу это одно и то же:
	// иначе перебором ID можно узнать, какие ссылки существуют у других.
	if tag.RowsAffected() == 0 {
		return repo.ErrNotFound
	}

	return nil
}
