package config

import (
	"errors"
	"os"
	"time"
)

type Config struct {
	ServerPort string
	DBURL      string
	JWTSecret  string
	TokenTTL   time.Duration
}

// ErrNoJWTSecret — сервис не должен подниматься с пустым или дефолтным ключом
// подписи: иначе токены сможет выпустить кто угодно.
var ErrNoJWTSecret = errors.New("JWT_SECRET is required")

var ErrNoDBURL = errors.New("DBURL is required")

func LoadConfig() (Config, error) {
	c := Config{
		ServerPort: getEnv("PORT", "8080"),
		DBURL:      getEnv("DBURL", ""),
		JWTSecret:  getEnv("JWT_SECRET", ""),
		TokenTTL:   getDuration("TOKEN_TTL", 24*time.Hour),
	}

	if c.DBURL == "" {
		return Config{}, ErrNoDBURL
	}

	if c.JWTSecret == "" {
		return Config{}, ErrNoJWTSecret
	}

	return c, nil
}

func getEnv(vari, def string) string {
	v := os.Getenv(vari)
	if v == "" {
		return def
	}

	return v
}

func getDuration(vari string, def time.Duration) time.Duration {
	d, err := time.ParseDuration(os.Getenv(vari))
	if err != nil || d <= 0 {
		return def
	}

	return d
}
