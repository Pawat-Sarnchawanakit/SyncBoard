package app

import (
	"sync-board/handlers/auth"
	"sync-board/handlers/front"
	"sync-board/models"
	"sync-board/services"

	"github.com/gin-gonic/gin"
)

type App struct {
	router    *gin.Engine
	services  *services.Services
	datastore *models.DataStore
}

func NewApp() (*App, error) {
	app := &App{}
	app.router = gin.Default()
	app.router.LoadHTMLGlob("templates/*")
	var err error
	if app.datastore, err = models.NewDataStore(); err != nil {
		return nil, err
	}
	app.services = services.NewServices(app)
	app.RegisterHandlers()
	return app, nil
}

func (app *App) RegisterHandlers() {
	front.RegisterHandlers(app)
	auth.RegisterHandlers(app)
}

func (app *App) GetRouter() *gin.Engine {
	return app.router
}

func (app *App) GetServices() *services.Services {
	return app.services
}

func (app *App) GetDatastore() *models.DataStore {
	return app.datastore
}

func (app *App) Run() {
	app.router.Run("127.0.0.1:8000")
}
