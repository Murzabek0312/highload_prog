package main

import (
	"fmt"
	"highload_prog/internal/app"
	"highload_prog/internal/config"
	"log"
)

func main() {
	c, err := config.New()
	if err != nil {
		log.Fatal(err)
	}

	err = app.Run(c)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Hello World")
}
