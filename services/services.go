package services

import (
	"sync-board/models"
	"sync-board/services/auth"

	"github.com/gin-gonic/gin"
)

type Services struct {
	AuthenticationService *auth.AuthenticationService
}

type App interface {
	GetRouter() *gin.Engine
	GetDatastore() *models.DataStore
}

func NewServices(app App) *Services {
	services := &Services{}
	services.AuthenticationService = auth.NewAuthenticationService(app)
	return services
}