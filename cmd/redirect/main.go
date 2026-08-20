package main

import (
	"log"

	redirect "tinyURL/internal/app/redirect"
)

func main() {
	if err := redirect.Start(); err != nil {
		log.Fatalf("tinyURL redirect: %s", err)
	}
}
