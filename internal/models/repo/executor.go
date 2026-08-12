package repo

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Executor — то, что умеет выполнять запросы. Под него подходят и *pgxpool.Pool,
// и pgx.Tx, поэтому репозиторий с полем этого типа работает одинаково внутри
// транзакции и вне её.
//
// Begin здесь намеренно нет: транзакцию открывает тот, кто владеет пулом,
// а репозиторий только выполняет запросы в уже открытой.
type Executor interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}
