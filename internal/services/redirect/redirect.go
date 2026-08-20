// Package redirect обслуживает переходы по коротким ссылкам: отдаёт длинный
// адрес по коду и фиксирует переход.
//
// Пакет намеренно не знает ни про пользователей, ни про авторизацию: редирект
// публичен, а сам сервис рассчитан на вынос в отдельный контейнер. Импорты
// auth, registration, userrepo и shorting сюда добавлять нельзя — иначе
// разделение перестанет быть настоящим.
package redirect

import (
	"context"
	"errors"
	"log"
	"time"

	"tinyURL/internal/models"
	"tinyURL/internal/models/repo"
)

// ErrNotFound — кода нет либо слот с таким кодом ещё не заполнен.
// Одна ошибка на оба случая: пользователю всё равно 404, а разные ответы
// позволили бы перебором выяснить, какие коды выданы.
var ErrNotFound = errors.New("short link not found")

// HrefReader — то, что умеет найти ссылку по коду. Интерфейс объявлен здесь,
// а не в репозитории: потребитель описывает ровно то, что ему нужно, и не
// тащит остальные девять методов HrefRepo.
type HrefReader interface {
	GetHrefByURL(ctx context.Context, url string) (*models.Href, error)
}

// ClickWriter — запись перехода. Чтение статистики живёт в другом сервисе,
// поэтому здесь только вставка.
type ClickWriter interface {
	CreateClick(ctx context.Context, click *models.Click) error
}

type Service struct {
	hrefs  HrefReader
	clicks ClickWriter
	now    func() time.Time // подменяется в тестах
}

// Option настраивает Service при создании.
type Option func(*Service)

// WithClock подменяет источник времени.
func WithClock(now func() time.Time) Option {
	return func(s *Service) {
		if now != nil {
			s.now = now
		}
	}
}

func New(hrefs HrefReader, clicks ClickWriter, opts ...Option) *Service {
	s := &Service{
		hrefs:  hrefs,
		clicks: clicks,
		now:    time.Now,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Resolve возвращает ссылку по короткому коду.
//
// Владелец не проверяется: по короткой ссылке должен переходить кто угодно,
// иначе делиться ей бессмысленно.
func (s *Service) Resolve(ctx context.Context, code string) (*models.Href, error) {
	if code == "" {
		return nil, ErrNotFound
	}

	href, err := s.hrefs.GetHrefByURL(ctx, code)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if href.LongURL == nil {
		return nil, ErrNotFound
	}

	return href, nil
}

// RecordClick фиксирует переход.
//
// Ошибку не возвращает намеренно: к моменту вызова редирект уже отправлен
// пользователю, и повлиять на ответ она не может. Потерянный клик — приемлемая
// потеря, статистика посещений не отчётность.
func (s *Service) RecordClick(ctx context.Context, hrefID int, ip string) {
	// Время фиксируем здесь, а не через now() в базе: между переходом и
	// записью проходит время, и оно вырастет, когда запись станет асинхронной.
	//
	// Country пока пустая: определять страну по IP нечем, нужна GeoIP-база.
	err := s.clicks.CreateClick(ctx, &models.Click{
		HrefID: hrefID,
		IP:     ip,
		Time:   s.now(),
	})

	// context.Canceled — это закрывшееся после редиректа соединение, а не сбой.
	// Логировать его значит забить лог сообщениями на каждый обычный переход.
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("redirect: click for href %d not recorded: %s", hrefID, err)
	}
}
