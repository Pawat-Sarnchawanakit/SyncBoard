package services

import (
	"sync-board/models"
	"sync-board/services/auth"
	"sync-board/services/board"
	"sync-board/services/team"

	"github.com/gin-gonic/gin"
)

type Services struct {
	AuthenticationService *auth.AuthenticationService
	BoardService          *board.BoardService
	TeamService           *team.TeamService
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
	services.BoardService, err = board.NewBoardService(app)
	if err != nil {
		return nil, err
	}
	services.TeamService, err = team.NewTeamService(app)
	if err != nil {
		return nil, err
	}
	return services, nil
}
