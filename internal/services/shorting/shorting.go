package shorting

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"tinyURL/internal/models"
	hrefrepo "tinyURL/internal/models/hrefRepo"
	"tinyURL/internal/models/repo"
)

type Service struct {
	repo    hrefrepo.HrefRepo
	ownHost string
}

// maxURLLen — потолок для длинного адреса. Браузеры и так режут примерно на
// 2000 символах; ограничение нужно, чтобы гигантский URL не поехал в каждый
// ответ ListSlots.
const maxURLLen = 8000

var (
	ErrBadScheme  = errors.New("not http(s) scheme")
	ErrInvalidURL = errors.New("invalid URL")
	ErrSelfLink   = errors.New("cycle href")
	ErrTooLongURL = errors.New("URL is too long")

	// ErrSlotNotFound — слота нет либо он принадлежит другому пользователю.
	// Намеренно одна ошибка на оба случая: иначе перебором ID можно было бы
	// узнать, какие слоты существуют у других.
	ErrSlotNotFound = errors.New("slot not found")
)

// New создаёт сервис. ownHost нужен, чтобы не дать сократить ссылку на сам
// сервис: иначе короткая ссылка вела бы на короткую, вплоть до петли.
func New(repo hrefrepo.HrefRepo, ownHost string) *Service {
	return &Service{
		repo:    repo,
		ownHost: ownHost,
	}
}

func (s *Service) FillSlot(ctx context.Context, userID, hrefID int, longURL string) error {
	longURL = strings.TrimSpace(longURL)

	if len(longURL) > maxURLLen {
		return ErrTooLongURL
	}

	// url.Parse на удивление терпим: строку без схемы и хоста он разберёт без
	// ошибки, просто вернёт пустые поля. Поэтому проверяем Host отдельно.
	u, err := url.Parse(longURL)
	if err != nil || u.Host == "" {
		return ErrInvalidURL
	}

	u.Scheme = strings.ToLower(u.Scheme)
	if u.Scheme != "http" && u.Scheme != "https" {
		return ErrBadScheme
	}

	if strings.EqualFold(u.Host, s.ownHost) {
		return ErrSelfLink
	}

	// Регистр значим только в хосте: DNS регистронезависим. Путь и query
	// трогать нельзя — там регистр несёт смысл (ID видео, имена файлов).
	u.Host = strings.ToLower(u.Host)

	return slotError(s.repo.FillSlot(ctx, userID, hrefID, u.String()))
}

func (s *Service) ClearSlot(ctx context.Context, userID, hrefID int) error {
	return slotError(s.repo.ClearSlot(ctx, userID, hrefID))
}

func (s *Service) ListSlots(ctx context.Context, userID int) ([]models.Href, error) {
	return s.repo.ListByUser(ctx, userID)
}

// slotError переводит ошибку хранилища в доменную, чтобы HTTP-слою не
// приходилось знать про пакет repo.
func slotError(err error) error {
	if errors.Is(err, repo.ErrNotFound) {
		return ErrSlotNotFound
	}

	return err
}
