package main

import (
	"log"

	"github.com/pixll/atlas/api"
)

func main() {
	if err := api.New(":8080").Run(); err != nil {
		log.Fatal(err)
	}
}
