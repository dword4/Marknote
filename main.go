package main

import (
	"log"
	"os"

	"gioui.org/app"
)

func main() {
	a := newApp()
	go func() {
		if err := a.run(); err != nil {
			log.Println(err)
			os.Exit(1)
		}
		os.Exit(0)
	}()
	app.Main()
}
