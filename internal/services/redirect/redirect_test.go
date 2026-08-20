package redirect

import (
	"context"
	"errors"
	"testing"
	"time"

	"tinyURL/internal/models"
	"tinyURL/internal/models/repo"
)

// Узкие интерфейсы окупаются здесь: мок — одна функция вместо десяти.
type stubHrefs struct {
	href *models.Href
	err  error
}

func (s stubHrefs) GetHrefByURL(context.Context, string) (*models.Href, error) {
	return s.href, s.err
}

type stubClicks struct {
	got *models.Click
	err error
}

func (s *stubClicks) CreateClick(_ context.Context, c *models.Click) error {
	s.got = c
	return s.err
}

func ptr(s string) *string { return &s }

func TestResolve(t *testing.T) {
	long := "https://example.com/page"

	tests := []struct {
		name string
		code string
		repo stubHrefs
		want error
	}{
		{"filled slot", "abc12345", stubHrefs{href: &models.Href{ID: 1, URL: "abc12345", LongURL: ptr(long)}}, nil},
		{"empty code", "", stubHrefs{}, ErrNotFound},
		{"unknown code", "nope", stubHrefs{err: repo.ErrNotFound}, ErrNotFound},
		{"empty slot", "abc12345", stubHrefs{href: &models.Href{ID: 1, URL: "abc12345"}}, ErrNotFound},
		{"db failure", "abc12345", stubHrefs{err: errors.New("connection refused")}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(tt.repo, &stubClicks{})

			href, err := s.Resolve(context.Background(), tt.code)

			if tt.name == "db failure" {
				if err == nil || errors.Is(err, ErrNotFound) {
					t.Fatalf("got %v, want the underlying error", err)
				}
				return
			}

			if !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}

			if tt.want == nil && *href.LongURL != long {
				t.Errorf("LongURL = %q", *href.LongURL)
			}
		})
	}
}

func TestRecordClick(t *testing.T) {
	moment := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	clicks := &stubClicks{}

	s := New(stubHrefs{}, clicks, WithClock(func() time.Time { return moment }))
	s.RecordClick(context.Background(), 7, "10.0.0.1")

	if clicks.got == nil {
		t.Fatal("click was not written")
	}
	if clicks.got.HrefID != 7 || clicks.got.IP != "10.0.0.1" {
		t.Errorf("click = %+v", clicks.got)
	}
	// Время берётся из сервиса, а не из базы.
	if !clicks.got.Time.Equal(moment) {
		t.Errorf("Time = %v, want %v", clicks.got.Time, moment)
	}
}

func TestRecordClickSurvivesFailure(t *testing.T) {
	clicks := &stubClicks{err: errors.New("db is down")}
	s := New(stubHrefs{}, clicks)

	// Не должно паниковать и не должно ничего возвращать: редирект уже ушёл.
	s.RecordClick(context.Background(), 7, "10.0.0.1")
}

func TestRecordClickIgnoresCanceledContext(t *testing.T) {
	clicks := &stubClicks{err: context.Canceled}
	s := New(stubHrefs{}, clicks)

	s.RecordClick(context.Background(), 7, "10.0.0.1")
}
