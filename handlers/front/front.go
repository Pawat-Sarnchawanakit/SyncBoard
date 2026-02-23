package front

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type App interface {
	GetRouter() *gin.Engine
}

func RegisterFrontHandlers(app App) {
	router := app.GetRouter()
	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{})
	})
	router.GET("/board", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{})
	})
}