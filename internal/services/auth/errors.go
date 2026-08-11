package auth

import "errors"

var (
	// ErrInvalidCredentials — неверная пара логин/пароль. Намеренно одна ошибка
	// и на "нет такого пользователя", и на "пароль не совпал", чтобы нельзя было
	// перебором узнать, какие логины зарегистрированы.
	ErrInvalidCredentials = errors.New("invalid username or password")

	// ErrUserExists — имя или email уже заняты.
	ErrUserExists = errors.New("user with this name or email already exists")

	// ErrInvalidToken — токен просрочен, повреждён или подписан чужим ключом.
	ErrInvalidToken = errors.New("invalid or expired token")

	ErrEmptyName      = errors.New("username is required")
	ErrShortName      = errors.New("username must be at least 3 characters")
	ErrLongName       = errors.New("username must be at most 32 characters")
	ErrInvalidEmail   = errors.New("email is invalid")
	ErrShortPassword  = errors.New("password must be at least 8 characters")
	ErrLongPassword   = errors.New("password must be at most 72 bytes")
	ErrEmptySecretKey = errors.New("jwt secret key is not configured")
)
