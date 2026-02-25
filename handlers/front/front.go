package front

import (
	"net/http"
	"sync-board/handlers/auth"
	"sync-board/services"

	"github.com/gin-gonic/gin"
)

type App interface {
	GetRouter() *gin.Engine
	GetServices() *services.Services
	GetHost() string
}

func RegisterHandlers(app App) {
	router := app.GetRouter()
	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"Authenticated": auth.IsAuthenticated(app, c),
		})
	})
	router.GET("/board", func(c *gin.Context) {
		c.HTML(http.StatusOK, "board.html", gin.H{})
	})
	router.GET("/signup", func(c *gin.Context) {
		if auth.IsAuthenticated(app, c) {
			c.Redirect(http.StatusTemporaryRedirect, "/")
			return
		}
		c.HTML(http.StatusOK, "signup.html", gin.H{})
	})
	router.GET("/login", func(c *gin.Context) {
		if auth.IsAuthenticated(app, c) {
			c.Redirect(http.StatusTemporaryRedirect, "/")
			return
		}
		c.HTML(http.StatusOK, "login.html", gin.H{})
	})
}