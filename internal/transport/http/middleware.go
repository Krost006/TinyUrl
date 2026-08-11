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

// RequireAuth пропускает дальше только запросы с валидным Bearer-токеном.
// Требование "сервис доступен только авторизированным пользователям"
// реализуется навешиванием этого middleware на защищённые ручки.
func (h *AuthHandler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, errMissingToken)
			return
		}

		user, err := h.service.Authenticate(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
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
