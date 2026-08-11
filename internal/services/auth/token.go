package auth

import (
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims — полезная нагрузка токена. Имя кладём для удобства UI,
// авторитетным идентификатором пользователя остаётся Subject.
type Claims struct {
	Name string `json:"name"`
	jwt.RegisteredClaims
}

// UserID возвращает ID пользователя, зашитый в Subject.
func (c *Claims) UserID() (int, error) {
	return strconv.Atoi(c.Subject)
}

// GenerateToken подписывает JWT для пользователя.
func (s *Service) GenerateToken(userID int, name string) (string, error) {
	now := s.now()

	claims := Claims{
		Name: name,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.Itoa(userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.tokenTTL)),
		},
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secretKey)
}

// ParseToken проверяет подпись и срок жизни токена и возвращает его claims.
func (s *Service) ParseToken(token string) (*Claims, error) {
	claims := &Claims{}

	_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		return s.secretKey, nil
	},
		// Алгоритм фиксируем явно: иначе токен с alg=none или подменённым
		// алгоритмом может пройти проверку.
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(s.now),
	)

	if err != nil {
		return nil, ErrInvalidToken
	}

	if _, err := claims.UserID(); err != nil {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// TokenTTL возвращает срок жизни выдаваемых токенов.
func (s *Service) TokenTTL() time.Duration {
	return s.tokenTTL
}
