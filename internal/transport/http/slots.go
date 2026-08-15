package http

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"tinyURL/internal/models"
	"tinyURL/internal/services/shorting"
)

var errBadSlotID = errors.New("slot id must be a number")

// SlotHandler — ручки управления слотами. Все требуют авторизации: слоты
// принадлежат конкретному пользователю.
type SlotHandler struct {
	service *shorting.Service
	auth    *Auth

	// baseURL — адрес сервиса, чтобы отдавать готовую короткую ссылку,
	// а не голый код.
	baseURL string
}

func NewSlotHandler(service *shorting.Service, a *Auth, baseURL string) *SlotHandler {
	return &SlotHandler{service: service, auth: a, baseURL: baseURL}
}

func (h *SlotHandler) Routes(mux *http.ServeMux) {
	mux.Handle("GET /api/hrefs", h.auth.RequireFunc(h.list))
	mux.Handle("PUT /api/hrefs/{id}", h.auth.RequireFunc(h.fill))
	mux.Handle("DELETE /api/hrefs/{id}", h.auth.RequireFunc(h.clear))
}

type slotResponse struct {
	ID       int     `json:"id"`
	Code     string  `json:"code"`
	ShortURL string  `json:"short_url"`
	LongURL  *string `json:"long_url"` // null — слот свободен
}

type listSlotsResponse struct {
	Slots []slotResponse `json:"slots"`
	Used  int            `json:"used"`
	Total int            `json:"total"`
}

type fillSlotRequest struct {
	LongURL string `json:"long_url"`
}

func (h *SlotHandler) list(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, errMissingToken)
		return
	}

	hrefs, err := h.service.ListSlots(r.Context(), user.ID)
	if err != nil {
		h.writeSlotError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, h.toListResponse(hrefs))
}

func (h *SlotHandler) fill(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, errMissingToken)
		return
	}

	hrefID, ok := slotID(w, r)
	if !ok {
		return
	}

	var req fillSlotRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if err := h.service.FillSlot(r.Context(), user.ID, hrefID, req.LongURL); err != nil {
		h.writeSlotError(w, err)
		return
	}

	writeJSON(w, http.StatusNoContent, nil)
}

func (h *SlotHandler) clear(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, errMissingToken)
		return
	}

	hrefID, ok := slotID(w, r)
	if !ok {
		return
	}

	if err := h.service.ClearSlot(r.Context(), user.ID, hrefID); err != nil {
		h.writeSlotError(w, err)
		return
	}

	writeJSON(w, http.StatusNoContent, nil)
}

// slotID достаёт {id} из пути. При ошибке сам отвечает клиенту.
func slotID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, errBadSlotID)
		return 0, false
	}

	return id, true
}

func (h *SlotHandler) toListResponse(hrefs []models.Href) listSlotsResponse {
	slots := make([]slotResponse, 0, len(hrefs))
	used := 0

	for _, href := range hrefs {
		if href.LongURL != nil {
			used++
		}

		slots = append(slots, slotResponse{
			ID:       href.ID,
			Code:     href.URL,
			ShortURL: h.baseURL + "/" + href.URL,
			LongURL:  href.LongURL,
		})
	}

	return listSlotsResponse{Slots: slots, Used: used, Total: len(slots)}
}

// writeSlotError переводит ошибку сервиса в HTTP-статус. Всё неизвестное —
// это 500, и клиенту деталей не показываем.
func (h *SlotHandler) writeSlotError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, shorting.ErrSlotNotFound):
		// Чужой слот и несуществующий слот дают одинаковый ответ: иначе
		// перебором ID можно узнать, какие слоты есть у других.
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, shorting.ErrInvalidURL),
		errors.Is(err, shorting.ErrBadScheme),
		errors.Is(err, shorting.ErrSelfLink),
		errors.Is(err, shorting.ErrTooLongURL):
		writeError(w, http.StatusBadRequest, err)
	default:
		log.Printf("slots: internal error: %s", err)
		writeError(w, http.StatusInternalServerError, errors.New("internal server error"))
	}
}
