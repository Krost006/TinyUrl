// Package repo содержит ошибки, общие для всех реализаций репозиториев.
// Сервисы разбирают их через errors.Is и не должны знать ни про pgx,
// ни про коды ошибок конкретной СУБД.
package repo

import "errors"

// ErrNotFound — запрошенной строки нет в базе.
var ErrNotFound = errors.New("not found")

// ErrAlreadyExists — нарушено ограничение уникальности.
var ErrAlreadyExists = errors.New("already exists")
