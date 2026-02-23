package main

import "sync-board/app"

// Views
// Sign Up
// Login
// Home page (create board, logim, join team, create time)
// Team page (view team boards, view members, edit permission)
// Board page (view/edit boards)

func main() {
  app := app.NewApp()
  app.Run()  
}
