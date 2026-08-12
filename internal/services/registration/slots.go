package registration

import (
	"context"
	"errors"
	"fmt"

	"tinyURL/internal/generator"
	hrefrepo "tinyURL/internal/models/hrefRepo"
	"tinyURL/internal/models/repo"
)

// SlotsPerUser — сколько слотов получает пользователь при регистрации.
// Требование "предоставляется места только для 10 ссылок" выполняется тем,
// что слотов ровно столько и новые не создаются: лимит структурный,
// а не проверка перед вставкой.
const SlotsPerUser = 10

// maxAttempts — сколько раз пробовать другой код, если сгенерированный занят.
// При 62^8 вариантов повтор практически невозможен, но без ограничения
// исчерпание кодов дало бы бесконечный цикл.
const maxAttempts = 5

// ErrNoFreeCode означает, что maxAttempts попыток подряд дали занятые коды.
var ErrNoFreeCode = errors.New("could not generate a free short code")

// createSlots создаёт пользователю SlotsPerUser пустых слотов.
//
// Репозиторий должен быть уже привязан к транзакции: слоты и сам пользователь
// обязаны появиться атомарно, иначе пользователь останется без слотов навсегда —
// выдаются они только при регистрации.
func createSlots(ctx context.Context, hrefs hrefrepo.HrefRepo, userID int) error {
	for i := 0; i < SlotsPerUser; i++ {
		if err := createSlot(ctx, hrefs, userID); err != nil {
			return fmt.Errorf("slot %d of %d: %w", i+1, SlotsPerUser, err)
		}
	}

	return nil
}

// createSlot вставляет один слот, подбирая свободный код.
//
// Занятость кода не проверяется отдельным запросом: между проверкой и вставкой
// код мог бы занять другой запрос. Вместо этого полагаемся на UNIQUE (url) —
// база отвергает дубль атомарно, а мы пробуем следующий код.
func createSlot(ctx context.Context, hrefs hrefrepo.HrefRepo, userID int) error {
	for attempt := 0; attempt < maxAttempts; attempt++ {
		code, err := generator.GenURL()
		if err != nil {
			return err
		}

		_, err = hrefs.CreateSlot(ctx, userID, code)

		// Код занят — берём новый. Любая другая ошибка означает, что дальше
		// пробовать бессмысленно.
		if errors.Is(err, repo.ErrAlreadyExists) {
			continue
		}

		return err
	}

	return ErrNoFreeCode
}
