package auth

import (
	"net/mail"
	"strings"
)

const (
	minNameLen     = 3
	maxNameLen     = 32
	minPasswordLen = 8
	// bcrypt молча обрезает всё после 72 байт, поэтому длинные пароли отсекаем сами.
	maxPasswordLen = 72
)

// validateName проверяет имя пользователя и возвращает его в нормализованном виде.
func validateName(name string) (string, error) {
	name = strings.TrimSpace(name)

	switch {
	case name == "":
		return "", ErrEmptyName
	case len([]rune(name)) < minNameLen:
		return "", ErrShortName
	case len([]rune(name)) > maxNameLen:
		return "", ErrLongName
	}

	return name, nil
}

// validateEmail проверяет адрес и возвращает его в нижнем регистре.
func validateEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", ErrInvalidEmail
	}

	return email, nil
}

// validatePassword проверяет пароль. Пробелы не срезаем — они значимая часть пароля.
func validatePassword(password string) error {
	switch {
	case len(password) < minPasswordLen:
		return ErrShortPassword
	case len(password) > maxPasswordLen:
		return ErrLongPassword
	}

	return nil
}
