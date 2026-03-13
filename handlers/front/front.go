package front

import (
	"net/http"
	"strconv"
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
	router.GET("/myboards", func(c *gin.Context) {
		if !auth.IsAuthenticated(app, c) {
			c.Redirect(http.StatusTemporaryRedirect, "/login")
			return
		}
		c.HTML(http.StatusOK, "myboards.html", gin.H{})
	})
	router.GET("/board/:id", func(c *gin.Context) {
		authenticated := auth.IsAuthenticated(app, c)
		if !authenticated {
			c.Redirect(http.StatusTemporaryRedirect, "/login")
			return
		}

		boardID, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.HTML(http.StatusBadRequest, "board.html", gin.H{
				"Authenticated": false,
				"BoardId":       "",
				"Permission":    "",
			})
			return
		}

		token, _ := c.Cookie("tk")
		userID, err := app.GetServices().AuthenticationService.VerifyToken(token)
		if err != nil {
			c.Redirect(http.StatusTemporaryRedirect, "/login")
			return
		}

		board, err := app.GetServices().BoardService.GetBoard(uint(boardID))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}

		permission, err := app.GetServices().BoardService.GetUserPermission(uint(boardID), userID)
		if err != nil {
			permission = ""
		}

		c.HTML(http.StatusOK, "board.html", gin.H{
			"Authenticated": true,
			"BoardId":       c.Param("id"),
			"BoardTitle":    board.Title,
			"Permission":    permission,
		})
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
	router.GET("/teams", func(c *gin.Context) {
		if !auth.IsAuthenticated(app, c) {
			c.Redirect(http.StatusTemporaryRedirect, "/login")
			return
		}
		c.HTML(http.StatusOK, "teams.html", gin.H{})
	})
	router.GET("/team/:id", func(c *gin.Context) {
		if !auth.IsAuthenticated(app, c) {
			c.Redirect(http.StatusTemporaryRedirect, "/login")
			return
		}

		teamID, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.Redirect(http.StatusTemporaryRedirect, "/teams")
			return
		}

		token, _ := c.Cookie("tk")
		userID, err := app.GetServices().AuthenticationService.VerifyToken(token)
		if err != nil {
			c.Redirect(http.StatusTemporaryRedirect, "/login")
			return
		}

		if !app.GetServices().TeamService.IsTeamMember(uint(teamID), userID) {
			c.Redirect(http.StatusTemporaryRedirect, "/teams")
			return
		}

		team, err := app.GetServices().TeamService.GetTeam(uint(teamID))
		if err != nil {
			c.Redirect(http.StatusTemporaryRedirect, "/teams")
			return
		}

		role := "member"
		if team.OwnerID == userID {
			role = "owner"
		}

		c.HTML(http.StatusOK, "team.html", gin.H{
			"TeamId":    c.Param("id"),
			"TeamTitle": team.Title,
			"Role":      role,
		})
	})
}
