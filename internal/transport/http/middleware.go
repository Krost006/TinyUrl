package http

import (
	"context"
	"net/http"
	"strings"

	"tinyURL/internal/models"
)

type contextKey struct{}

var userContextKey = contextKey{}

// UserFromContext достаёт пользователя, положенного в контекст middleware.
func UserFromContext(ctx context.Context) (*models.User, bool) {
	user, ok := ctx.Value(userContextKey).(*models.User)
	return user, ok
}

// Authenticator — то, что умеет проверить токен и вернуть его владельца.
// Middleware зависит только от этого, а не от сервиса целиком: логин и выдача
// токенов ему не нужны.
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (*models.User, error)
}

// Auth — проверка авторизации для защищённых ручек. Отдельный тип, а не метод
// AuthHandler: middleware нужен всем хендлерам, а не только тому, что отвечает
// за вход.
type Auth struct {
	service Authenticator
}

func NewAuth(service Authenticator) *Auth {
	return &Auth{service: service}
}

// Require пропускает дальше только запросы с валидным Bearer-токеном.
// Требование "сервис доступен только авторизированным пользователям"
// реализуется навешиванием этого middleware на защищённые ручки.
func (a *Auth) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, errMissingToken)
			return
		}

		user, err := a.service.Authenticate(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireFunc — то же самое для http.HandlerFunc, чтобы не оборачивать вручную.
func (a *Auth) RequireFunc(next http.HandlerFunc) http.Handler {
	return a.Require(next)
}

// bearerToken вытаскивает токен из заголовка Authorization.
func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")

	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return "", false
	}

	return token, true
}
