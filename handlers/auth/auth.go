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
	return err == nil
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

func searchUsersHandler(app App, c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query required"})
		return
	}

	users, err := app.GetServices().AuthenticationService.SearchUsers(query, 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type userResult struct {
		ID       uint   `json:"id"`
		Username string `json:"username"`
	}
	var results []userResult
	for _, u := range users {
		results = append(results, userResult{ID: u.ID, Username: u.Username})
	}

	c.JSON(http.StatusOK, gin.H{"users": results})
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
	router.GET("/api/users/search", func(c *gin.Context) { searchUsersHandler(app, c) })

	router.POST("/api/settings/password", func(c *gin.Context) { changePasswordHandler(app, c) })
	router.DELETE("/api/settings/account", func(c *gin.Context) { deleteAccountHandler(app, c) })
}

func getUserIDFromContext(app App, c *gin.Context) (uint, bool) {
	token, err := c.Cookie("tk")
	if err != nil {
		return 0, false
	}
	userID, err := app.GetServices().AuthenticationService.VerifyToken(token)
	if err != nil {
		return 0, false
	}
	return userID, true
}

func changePasswordHandler(app App, c *gin.Context) {
	userID, ok := getUserIDFromContext(app, c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	type ChangePasswordRequest struct {
		CurrentPassword string `json:"currentPassword" binding:"required,min=8,max=128"`
		NewPassword     string `json:"newPassword" binding:"required,min=8,max=128"`
	}

	var req ChangePasswordRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := app.GetServices().AuthenticationService.ChangePassword(userID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password changed successfully"})
}

func deleteAccountHandler(app App, c *gin.Context) {
	userID, ok := getUserIDFromContext(app, c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	type DeleteAccountRequest struct {
		Password string `json:"password" binding:"required,min=8,max=128"`
	}

	var req DeleteAccountRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := app.GetServices().AuthenticationService.DeleteUser(userID, req.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.SetCookie("tk", "", -1, "/", "", false, false)
	c.JSON(http.StatusOK, gin.H{"message": "account deleted successfully"})
}
