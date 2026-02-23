package app

import "sync-board/handlers/front"
import "github.com/gin-gonic/gin"

type App struct {
  router *gin.Engine
}

func NewApp() *App {
  app := &App {}
  app.router = gin.Default()
  app.router.LoadHTMLGlob("templates/*")
  app.RegisterHandlers()
  return app
}

func (app *App) RegisterHandlers() {
  front.RegisterFrontHandlers(app)
}

func (app *App) GetRouter() *gin.Engine {
  return app.router
}

func (app *App) Run() {
  app.router.Run("127.0.0.1:8000")
}