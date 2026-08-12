package auth

import "errors"

var (
	// ErrInvalidCredentials — неверная пара логин/пароль. Намеренно одна ошибка
	// и на "нет такого пользователя", и на "пароль не совпал", чтобы нельзя было
	// перебором узнать, какие логины зарегистрированы.
	ErrInvalidCredentials = errors.New("invalid username or password")

	// ErrInvalidToken — токен просрочен, повреждён или подписан чужим ключом.
	ErrInvalidToken = errors.New("invalid or expired token")

	// ErrEmptySecretKey — сервис создан без ключа подписи токенов.
	ErrEmptySecretKey = errors.New("jwt secret key is not configured")
)
