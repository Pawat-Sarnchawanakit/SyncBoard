package board

import (
	"net/http"
	"strconv"
	"sync-board/services"
	"sync-board/services/board"

	"github.com/gin-gonic/gin"
)

type App interface {
	GetRouter() *gin.Engine
	GetServices() *services.Services
	GetHost() string
}

type CreateBoardRequest struct {
	Title       string `json:"title" binding:"required,min=1,max=128"`
	Description string `json:"description"`
	Tags        string `json:"tags"`
}

type UpdateBoardRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Tags        *string `json:"tags"`
}

type AddTagsRequest struct {
	Tags []string `json:"tags" binding:"required,min=1"`
}

func getUserID(app App, c *gin.Context) (uint, bool) {
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

func createBoardHandler(app App, c *gin.Context) {
	userID, ok := getUserID(app, c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req CreateBoardRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	board, err := app.GetServices().BoardService.CreateBoard(req.Title, req.Description, req.Tags, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":          board.ID,
		"title":       board.Title,
		"description": board.Description,
		"tags":        board.Tags,
		"createdAt":   board.CreatedAt,
	})
}

func getBoardsHandler(app App, c *gin.Context) {
	userID, ok := getUserID(app, c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	boards, err := app.GetServices().BoardService.GetUserBoards(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"boards": boards})
}

func getBoardHandler(app App, c *gin.Context) {
	userID, ok := getUserID(app, c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	boardID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid board id"})
		return
	}

	board, err := app.GetServices().BoardService.GetBoardByIDAndOwner(uint(boardID), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "board not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":          board.ID,
		"title":       board.Title,
		"description": board.Description,
		"tags":        board.Tags,
		"createdAt":   board.CreatedAt,
	})
}

func updateBoardHandler(app App, c *gin.Context) {
	userID, ok := getUserID(app, c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	boardID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid board id"})
		return
	}

	var req UpdateBoardRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input := board.UpdateBoardInput{}
	if req.Title != nil {
		input.Title = req.Title
	}
	if req.Description != nil {
		input.Description = req.Description
	}
	if req.Tags != nil {
		input.Tags = req.Tags
	}

	board, err := app.GetServices().BoardService.UpdateBoard(uint(boardID), userID, input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":          board.ID,
		"title":       board.Title,
		"description": board.Description,
		"tags":        board.Tags,
		"createdAt":   board.CreatedAt,
	})
}

func addTagsHandler(app App, c *gin.Context) {
	userID, ok := getUserID(app, c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	boardID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid board id"})
		return
	}

	var req AddTagsRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	board, err := app.GetServices().BoardService.AddTags(uint(boardID), userID, req.Tags)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":          board.ID,
		"title":       board.Title,
		"description": board.Description,
		"tags":        board.Tags,
		"createdAt":   board.CreatedAt,
	})
}

func deleteBoardHandler(app App, c *gin.Context) {
	userID, ok := getUserID(app, c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	boardID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid board id"})
		return
	}

	err = app.GetServices().BoardService.DeleteBoard(uint(boardID), userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func RegisterHandlers(app App) {
	router := app.GetRouter()
	router.GET("/ws", func(c *gin.Context) {
	})
	router.POST("/api/boards", func(c *gin.Context) { createBoardHandler(app, c) })
	router.GET("/api/boards", func(c *gin.Context) { getBoardsHandler(app, c) })
	router.GET("/api/boards/:id", func(c *gin.Context) { getBoardHandler(app, c) })
	router.PATCH("/api/boards/:id", func(c *gin.Context) { updateBoardHandler(app, c) })
	router.POST("/api/boards/:id/tags", func(c *gin.Context) { addTagsHandler(app, c) })
	router.DELETE("/api/boards/:id", func(c *gin.Context) { deleteBoardHandler(app, c) })
}
