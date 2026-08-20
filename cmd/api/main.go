package main

import (
	"log"

	"tinyURL/internal/app"
)

func main() {
	if err := app.Start(); err != nil {
		log.Fatalf("tinyURL: %s", err)
	}
}
