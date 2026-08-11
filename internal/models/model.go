package models

import "time"

type User struct {
	ID       int
	Name     string
	Password string // bcrypt-хеш, наружу не отдаётся
	Email    string
}

type Href struct {
	ID      int
	URL     string
	LongURL string
}

type UserHref struct {
	ID     int
	UserID int
	HrefID int
}

type Click struct {
	ID      int
	HrefID  int
	IP      string
	Country string
	Time    time.Time
}
