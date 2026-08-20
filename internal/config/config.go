package config

import (
	"errors"
	"net/url"
	"os"
	"strings"
	"time"
)

type Config struct {
	ServerPort string
	DBURL      string
	JWTSecret  string
	TokenTTL   time.Duration

	// BaseURL — адрес, по которому сервис доступен снаружи, например
	// "https://short.ly". Нужен, чтобы отдавать клиенту готовую короткую
	// ссылку, а не голый код.
	BaseURL string

	// WebDir — каталог с файлами фронтенда.
	WebDir string
}

// ErrNoJWTSecret — сервис не должен подниматься с пустым или дефолтным ключом
// подписи: иначе токены сможет выпустить кто угодно.
var ErrNoJWTSecret = errors.New("JWT_SECRET is required")

var ErrNoDBURL = errors.New("DBURL is required")

// ErrBadBaseURL — BASE_URL задан, но это не абсолютный http(s)-адрес.
var ErrBadBaseURL = errors.New("BASE_URL must be an absolute http(s) URL")

func LoadConfig() (Config, error) {
	port := getEnv("PORT", "8080")

	c := Config{
		ServerPort: port,
		DBURL:      getEnv("DBURL", ""),
		JWTSecret:  getEnv("JWT_SECRET", ""),
		TokenTTL:   getDuration("TOKEN_TTL", 24*time.Hour),
		// Для разработки хватает локального адреса, в проде BASE_URL задают явно.
		BaseURL: strings.TrimRight(getEnv("BASE_URL", "http://localhost:"+port), "/"),
		WebDir:  getEnv("WEB_DIR", "web"),
	}

	if c.DBURL == "" {
		return Config{}, ErrNoDBURL
	}

	if c.JWTSecret == "" {
		return Config{}, ErrNoJWTSecret
	}

	if _, err := c.Host(); err != nil {
		return Config{}, err
	}

	return c, nil
}

// Host возвращает хост сервиса без схемы — в таком виде его сравнивают с хостом
// сокращаемой ссылки, чтобы не дать зациклить редирект.
//
// Заодно проверяет, что BaseURL вообще пригоден: поэтому LoadConfig зовёт Host
// на старте, а не ждёт первого запроса.
func (c Config) Host() (string, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", ErrBadBaseURL
	}

	return u.Host, nil
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
