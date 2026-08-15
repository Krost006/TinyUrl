package http

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"tinyURL/internal/models"
	hrefrepo "tinyURL/internal/models/hrefRepo"
	"tinyURL/internal/models/repo"
	"tinyURL/internal/services/shorting"

	"github.com/jackc/pgx/v5"
)

type slotRepo struct{ slots map[int]*models.Href }

func (s *slotRepo) ListByUser(context.Context, int) ([]models.Href, error) {
	out := []models.Href{}
	for _, h := range s.slots {
		out = append(out, *h)
	}
	return out, nil
}
func (s *slotRepo) FillSlot(_ context.Context, _, id int, u string) error {
	h, ok := s.slots[id]
	if !ok {
		return repo.ErrNotFound
	}
	h.LongURL = &u
	return nil
}
func (s *slotRepo) ClearSlot(_ context.Context, _, id int) error {
	h, ok := s.slots[id]
	if !ok {
		return repo.ErrNotFound
	}
	h.LongURL = nil
	return nil
}
func (s *slotRepo) GetHrefById(context.Context, int) (*models.Href, error)     { return nil, nil }
func (s *slotRepo) GetHrefByURL(context.Context, string) (*models.Href, error) { return nil, nil }
func (s *slotRepo) GetHrefByUserAndLongURL(context.Context, int, string) (*models.Href, error) {
	return nil, nil
}
func (s *slotRepo) CreateHref(context.Context, int, *models.Href) (int, error) { return 0, nil }
func (s *slotRepo) CreateSlot(context.Context, int, string) (int, error)       { return 0, nil }
func (s *slotRepo) CountByUser(context.Context, int) (int, error)              { return 0, nil }
func (s *slotRepo) WithTx(pgx.Tx) hrefrepo.HrefRepo                            { return s }

func TestSlotRoutes(t *testing.T) {
	users := newMemRepo()
	addUser(t, users, "krost", "k@example.com", "password123")

	authSvc, _ := newAuthService(t, users)
	token := mustToken(t, authSvc, 1, "krost")

	sr := &slotRepo{slots: map[int]*models.Href{1: {ID: 1, URL: "abc12345"}}}
	sh := shorting.New(sr, "short.ly")

	mux := http.NewServeMux()
	NewSlotHandler(sh, NewAuth(authSvc), "https://short.ly").Routes(mux)

	tests := []struct {
		name, method, path, body, tok string
		want                          int
	}{
		{"list ok", "GET", "/api/hrefs", "", token, 200},
		{"list no token", "GET", "/api/hrefs", "", "", 401},
		{"fill ok", "PUT", "/api/hrefs/1", `{"long_url":"https://example.com/x"}`, token, 204},
		{"fill bad scheme", "PUT", "/api/hrefs/1", `{"long_url":"javascript:x"}`, token, 400},
		{"fill self link", "PUT", "/api/hrefs/1", `{"long_url":"https://short.ly/a"}`, token, 400},
		{"fill missing slot", "PUT", "/api/hrefs/99", `{"long_url":"https://example.com"}`, token, 404},
		{"fill bad id", "PUT", "/api/hrefs/abc", `{"long_url":"https://example.com"}`, token, 400},
		{"clear ok", "DELETE", "/api/hrefs/1", "", token, 204},
		{"clear missing", "DELETE", "/api/hrefs/99", "", token, 404},
		{"clear no token", "DELETE", "/api/hrefs/1", "", "", 401},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, mux, tt.method, tt.path, tt.body, tt.tok)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tt.want, rec.Body)
			}
		})
	}

	// После clear слот снова пуст: long_url должен быть null, а не пустая строка.
	rec := do(t, mux, "GET", "/api/hrefs", "", token)

	var list listSlotsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}

	if len(list.Slots) != 1 {
		t.Fatalf("got %d slots, want 1", len(list.Slots))
	}

	if list.Slots[0].LongURL != nil {
		t.Errorf("long_url = %v, want null after clear", *list.Slots[0].LongURL)
	}

	if list.Slots[0].ShortURL != "https://short.ly/abc12345" {
		t.Errorf("short_url = %q", list.Slots[0].ShortURL)
	}

	if list.Used != 0 || list.Total != 1 {
		t.Errorf("used=%d total=%d, want 0 and 1", list.Used, list.Total)
	}
}
