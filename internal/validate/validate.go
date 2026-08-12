// Package validate проверяет пользовательский ввод. Живёт отдельно от сервисов,
// потому что нужен и регистрации (полная проверка), и авторизации (разобрать,
// email перед нами или имя).
package validate

import (
	"errors"
	"net/mail"
	"strings"
)

const (
	MinNameLen     = 3
	MaxNameLen     = 32
	MinPasswordLen = 8
	// MaxPasswordLen — не настройка, а ограничение bcrypt: он молча отбрасывает
	// всё после 72 байт, поэтому длинные пароли отсекаем сами.
	MaxPasswordLen = 72
)

var (
	ErrEmptyName     = errors.New("username is required")
	ErrShortName     = errors.New("username must be at least 3 characters")
	ErrLongName      = errors.New("username must be at most 32 characters")
	ErrInvalidEmail  = errors.New("email is invalid")
	ErrShortPassword = errors.New("password must be at least 8 characters")
	ErrLongPassword  = errors.New("password must be at most 72 bytes")
)

// Name проверяет имя пользователя и возвращает его в нормализованном виде.
func Name(name string) (string, error) {
	name = strings.TrimSpace(name)

	switch {
	case name == "":
		return "", ErrEmptyName
	case len([]rune(name)) < MinNameLen:
		return "", ErrShortName
	case len([]rune(name)) > MaxNameLen:
		return "", ErrLongName
	}

	return name, nil
}

// Email проверяет адрес и возвращает его в нижнем регистре.
//
// Регистр локальной части по RFC 5321 значим, но на практике ни один почтовый
// провайдер этим не пользуется. Нормализация избавляет от двух аккаунтов на
// один ящик и от входа, который не работает из-за заглавной буквы.
func Email(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	// ParseAddress принимает и "Имя <a@b.com>", поэтому сверяем результат
	// с исходной строкой: нам нужен чистый адрес.
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", ErrInvalidEmail
	}

	return email, nil
}

// Password проверяет пароль. Пробелы не срезаем — они значимая часть пароля.
func Password(password string) error {
	switch {
	case len(password) < MinPasswordLen:
		return ErrShortPassword
	case len(password) > MaxPasswordLen:
		return ErrLongPassword
	}

	return nil
}

// LooksLikeEmail отвечает на вопрос "по email искать пользователя или по имени".
// Именно проверка, а не Email: логин из формы входа не нужно нормализовать,
// его всё равно ждёт запрос в базу.
func LooksLikeEmail(login string) bool {
	_, err := Email(login)
	return err == nil
}
