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

func NewServices(app App) (*Services, error) {
	services := &Services{}
	var err error
	services.AuthenticationService, err = auth.NewAuthenticationService(app)
	if err != nil {
		return nil, err
	}
	return services, nil
}