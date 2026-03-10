package auth

import (
	"net/http"
	"sync-board/services"

	"github.com/gin-gonic/gin"
)

type App interface {
	GetRouter() *gin.Engine
	GetServices() *services.Services
	GetHost() string
}

func IsAuthenticated(app App, c *gin.Context) bool {
	token, err := c.Cookie("tk")
	if err != nil {
		return false
	}
	_, err = app.GetServices().AuthenticationService.VerifyToken(token)
	if err != nil {
		return false
	}
	return true
}

func signUpHandler(app App, c *gin.Context) {
	type SignUpRequest struct {
		Username string `json:"username" binding:"required,min=3,max=128"`
		Password string `json:"password" binding:"required,min=8,max=128"`
	}
	var signup_request SignUpRequest
	if err := c.BindJSON(&signup_request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	token, err := app.GetServices().AuthenticationService.SignUp(signup_request.Username, signup_request.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.SetCookie("tk", token, 3600*30, "/", app.GetHost(), true, true)
	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func loginHandler(app App, c *gin.Context) {
	type LoginRequest struct {
		Username string `json:"username" binding:"required,min=3,max=128"`
		Password string `json:"password" binding:"required,min=8,max=128"`
	}
	var login_request LoginRequest
	if err := c.BindJSON(&login_request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	token, err := app.GetServices().AuthenticationService.Login(login_request.Username, login_request.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.SetCookie("tk", token, 3600*30, "/", app.GetHost(), true, true)
	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func logoutHandler(app App, c *gin.Context) {
	c.SetCookie("tk", "", -1, "/", app.GetHost(), true, true)
}

type HandlerFunc func(app App, c *gin.Context)

func RequireAuth(app App, handler HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !IsAuthenticated(app, c) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		handler(app, c)
	}
}

func RegisterHandlers(app App) {
	router := app.GetRouter()
	router.POST("/api/signup", func(c *gin.Context) { signUpHandler(app, c) })
	router.POST("/api/login", func(c *gin.Context) { loginHandler(app, c) })
	router.POST("/api/logout", func(c *gin.Context) { logoutHandler(app, c) })
}
