// Package redirect — HTTP-слой сервиса редиректов. Одна публичная ручка,
// без авторизации и без JSON.
package redirect

import (
	"context"
	"net"
	"net/http"
	"strings"

	"tinyURL/internal/services/redirect"
)

type Handler struct {
	service *redirect.Service
}

func NewHandler(service *redirect.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	// Здесь нет RequireAuth: короткая ссылка публична, иначе делиться ей
	// не имеет смысла. Требование "сервис доступен только авторизированным"
	// относится к управлению ссылками, а не к переходу по ним.
	mux.HandleFunc("GET /{code}", h.redirect)
}

func (h *Handler) redirect(w http.ResponseWriter, r *http.Request) {
	href, err := h.service.Resolve(r.Context(), r.PathValue("code"))
	if err != nil {
		// Виды ошибок наружу не различаем: и несуществующий код, и пустой
		// слот, и сбой базы дают 404. Разные ответы позволили бы перебором
		// выяснить, какие коды выданы.
		http.NotFound(w, r)
		return
	}

	// 302, а не 301: постоянный редирект браузер кэширует навсегда, и после
	// смены адреса в слоте пользователь продолжит уходить на старый — клика
	// мы при этом тоже не увидим.
	http.Redirect(w, r, *href.LongURL, http.StatusFound)

	// Клик пишем после ответа: редирект — самая горячая ручка сервиса,
	// и заставлять пользователя ждать вставку в базу незачем.
	//
	// WithoutCancel обязателен: r.Context() отменяется, как только клиент
	// закрыл соединение, а он закрывает его сразу после редиректа. Без этого
	// каждая запись падала бы с context.Canceled.
	ctx := context.WithoutCancel(r.Context())
	h.service.RecordClick(ctx, href.ID, clientIP(r))
}

// clientIP возвращает адрес клиента.
//
// clientIP возвращает адрес клиента.
//
// Сервис работает за nginx, поэтому в RemoteAddr лежит адрес прокси, а
// настоящий клиент приходит в X-Forwarded-For.
//
// Заголовку здесь доверяют безусловно, и это допустимо ровно до тех пор, пока
// сервис недоступен снаружи напрямую: клиент подделывает X-Forwarded-For одной
// строкой, и при прямом доступе в статистику можно записать любой адрес.
// Если сервис когда-нибудь начнёт принимать запросы без прокси, доверие нужно
// будет ограничить проверкой RemoteAddr.
func clientIP(r *http.Request) string {
	// В цепочке "203.0.113.5, 10.0.0.1" первый элемент — исходный клиент,
	// остальные дописаны прокси по пути.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first, _, _ := strings.Cut(xff, ",")

		// SplitHostPort здесь не годится: в X-Forwarded-For порта нет,
		// и разбор вернул бы ошибку на каждом запросе.
		if ip := strings.TrimSpace(first); ip != "" {
			return ip
		}
	}

	// RemoteAddr всегда имеет вид "host:port", порт нам не нужен —
	// он случайный и меняется с каждым соединением.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// Не разобралось — лучше записать адрес как есть, чем пустую строку.
		return r.RemoteAddr
	}

	return host
}
