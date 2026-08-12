package shorting

import (
	"context"
	"time"
	"tinyURL/internal/models"
	hrefrepo "tinyURL/internal/models/hrefRepo"
)

type Service struct {
	repo hrefrepo.HrefRepo
	now  func() time.Time // подменяется в тестах
}

func (s *Service) CreateHref(ctx context.Context, models.Href)
