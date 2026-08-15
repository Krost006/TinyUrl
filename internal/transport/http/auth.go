package http

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"tinyURL/internal/models"
	"tinyURL/internal/services/auth"
	"tinyURL/internal/services/registration"
	"tinyURL/internal/validate"
)

var (
	errMissingToken = errors.New("authorization token is required")
	errBadJSON      = errors.New("request body must be a valid JSON object")
)

// maxBodySize ограничивает тело запроса, чтобы клиент не мог занять память
// сервера гигабайтным JSON.
const maxBodySize = 1 << 20 // 1 MiB

// AuthHandler — HTTP-ручки сервиса пользователей.
type AuthHandler struct {
	service      *auth.Service
	registration *registration.Service
	auth         *Auth
}

func NewAuthHandler(service *auth.Service, reg *registration.Service, a *Auth) *AuthHandler {
	return &AuthHandler{service: service, registration: reg, auth: a}
}

// Routes навешивает ручки сервиса пользователей на мультиплексор.
func (h *AuthHandler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/register", h.register)
	mux.HandleFunc("POST /api/auth/login", h.login)
	mux.Handle("GET /api/auth/me", h.auth.RequireFunc(h.me))
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type userResponse struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type authResponse struct {
	User      userResponse `json:"user"`
	Token     string       `json:"token"`
	ExpiresIn int          `json:"expires_in"` // секунды
}

func (h *AuthHandler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	user, token, err := h.registration.Register(r.Context(), req.Name, req.Email, req.Password)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, h.authResponse(user, token))
}

func (h *AuthHandler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	user, token, err := h.service.Login(r.Context(), req.Login, req.Password)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, h.authResponse(user, token))
}

func (h *AuthHandler) me(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, errMissingToken)
		return
	}

	writeJSON(w, http.StatusOK, toUserResponse(user))
}

func (h *AuthHandler) authResponse(user *models.User, token string) authResponse {
	return authResponse{
		User:      toUserResponse(user),
		Token:     token,
		ExpiresIn: int(h.service.TokenTTL().Seconds()),
	}
}

func toUserResponse(user *models.User) userResponse {
	return userResponse{ID: user.ID, Name: user.Name, Email: user.Email}
}

// writeAuthError переводит ошибку сервиса в HTTP-статус. Всё, что не является
// известной доменной ошибкой, отдаём как 500 и не показываем клиенту детали.
func (h *AuthHandler) writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, err)
	case errors.Is(err, auth.ErrInvalidToken):
		writeError(w, http.StatusUnauthorized, err)
	case errors.Is(err, registration.ErrUserExists):
		writeError(w, http.StatusConflict, err)
	case errors.Is(err, validate.ErrEmptyName),
		errors.Is(err, validate.ErrShortName),
		errors.Is(err, validate.ErrLongName),
		errors.Is(err, validate.ErrInvalidEmail),
		errors.Is(err, validate.ErrShortPassword),
		errors.Is(err, validate.ErrLongPassword):
		writeError(w, http.StatusBadRequest, err)
	default:
		log.Printf("auth: internal error: %s", err)
		writeError(w, http.StatusInternalServerError, errors.New("internal server error"))
	}
}

// decodeJSON читает тело запроса. При ошибке сам отвечает клиенту и возвращает false.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, errBadJSON)
		return false
	}

	return true
}
