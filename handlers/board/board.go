package board

import (
	"github.com/gorilla/websocket"
	"github.com/gin-gonic/gin"
)

type App interface {
	GetRouter() *gin.Engine
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

func RegisterHandlers(app App) {
	router := app.GetRouter()
	router.GET("/ws", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
	})
}