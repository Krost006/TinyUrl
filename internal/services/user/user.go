package user

import (
	"context"
	"time"
	"tinyURL/internal/models"
	hrefrepo "tinyURL/internal/models/hrefRepo"
	userrepo "tinyURL/internal/models/userRepo"
)

type Service struct {
	userrepo userrepo.UserRepo
	hrefrepo hrefrepo.HrefRepo
	now      func() time.Time // подменяется в тестах
}

func New() {}

func (s *Service) Accaunt(ctx context.Context, Id int) (*models.User, error) {
	user, err := s.userrepo.GetUserById(ctx, Id)

	return user, err
}

func (s *Service) HrefList(ctx context.Context, Id int) (*[]models.Href, error) {
	hrefs, err := s.hrefrepo.ListByUser(ctx, Id)

	return &hrefs, err
}
