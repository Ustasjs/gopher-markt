package main

import (
	"log"

	"github.com/ustasjs/gopher-markt/internal/router"
)

func main() {
	if err := router.StartServer(); err != nil {
		log.Fatal(err)
	}
}
