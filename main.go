package main

import (
	"log"
	"sync-board/app"

	"github.com/joho/godotenv"
)

// Views
// Sign Up
// Login
// Home page (create board, logim, join team, create time)
// Team page (view team boards, view members, edit permission)
// Board page (view/edit boards)

func main() {
	_ = godotenv.Load()
	app, err := app.NewApp()
	if err != nil {
		log.Fatal(err)
	}
	err = app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
