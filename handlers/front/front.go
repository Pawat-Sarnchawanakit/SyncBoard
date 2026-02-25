package front

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type App interface {
	GetRouter() *gin.Engine
}

func RegisterHandlers(app App) {
	router := app.GetRouter()
	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{})
	})
	router.GET("/board", func(c *gin.Context) {
		c.HTML(http.StatusOK, "board.html", gin.H{})
	})
	router.GET("/signup", func(c *gin.Context) {
		c.HTML(http.StatusOK, "signup.html", gin.H{})
	})
	router.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", gin.H{})
	})
}