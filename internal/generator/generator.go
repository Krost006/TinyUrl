package generator

import (
	"crypto/rand"
	"math/big"
)

const (
	alphabet   = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	CodeLength = 8
)

// Gen возвращает n различных кодов.
func Gen(n int) ([]string, error) {
	seen := make(map[string]struct{}, n)
	codes := make([]string, 0, n)

	for len(codes) < n {
		code, err := GenURL()
		if err != nil {
			return nil, err
		}

		if _, dup := seen[code]; dup {
			continue
		}

		seen[code] = struct{}{}
		codes = append(codes, code)
	}

	return codes, nil
}

// GenURL возвращает случайный код из CodeLength символов алфавита.
func GenURL() (string, error) {
	code := make([]byte, CodeLength)

	// Границу считаем один раз: внутри цикла это была бы лишняя аллокация
	// на каждый символ.
	max := big.NewInt(int64(len(alphabet)))

	for i := range code {
		// rand.Int даёт равномерное значение в [0, max) без смещения,
		// которое появилось бы при взятии остатка от случайного числа.
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}

		code[i] = alphabet[n.Int64()]
	}

	return string(code), nil
}
