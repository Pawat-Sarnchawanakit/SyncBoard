package auth

import (
	"net/http"
	"sync-board/services"
	"github.com/gin-gonic/gin"
)

type App interface {
	GetRouter() *gin.Engine
	GetServices() *services.Services
}

func signUpHandler(app App, c *gin.Context) {
	type SignUpRequest struct {
		Username string `json:"username" binding:"required,min=3,max=128"`
		Password string `json:"password" binding:"required,min=8,max=128"`
	};
	var signup_request SignUpRequest
	if err := c.BindJSON(&signup_request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	if err := app.GetServices().AuthenticationService.SignUp(signup_request.Username, signup_request.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	});
}

func RegisterHandlers(app App) {
	router := app.GetRouter()
	router.POST("/api/signup", func(c *gin.Context) { signUpHandler(app, c) })
}