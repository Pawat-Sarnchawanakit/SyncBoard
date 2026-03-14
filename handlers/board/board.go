package board

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync-board/services"
	boardsvc "sync-board/services/board"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

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

type AddMemberRequest struct {
	Username string `json:"username" binding:"required"`
	Role     string `json:"role"`
}

type UpdateMemberRequest struct {
	Role string `json:"role" binding:"required"`
}

type MemberResponse struct {
	ID       uint   `json:"id"`
	UserID   uint   `json:"userId"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type WsMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type DrawPayload struct {
	X1    float64 `json:"x1"`
	Y1    float64 `json:"y1"`
	X2    float64 `json:"x2"`
	Y2    float64 `json:"y2"`
	Color string  `json:"color"`
	Size  float64 `json:"size"`
	Tool  string  `json:"tool"`
}

type TextPayload struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Text  string  `json:"text"`
	Color string  `json:"color"`
	Size  float64 `json:"size"`
}

type CursorPayload struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Color string  `json:"color"`
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

	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	boards, err := app.GetServices().BoardService.GetUserBoardsWithAccess(userID, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"boards": boards,
		"limit":  limit,
	})
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

	if !app.GetServices().BoardService.HasViewAccess(uint(boardID), userID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "no access to this board"})
		return
	}

	board, err := app.GetServices().BoardService.GetBoard(uint(boardID))
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
		"ownerId":     board.OwnerID,
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

	input := boardsvc.UpdateBoardInput{}
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

type WSApp interface {
	GetServices() *services.Services
}

func wsHandler(app WSApp, c *gin.Context) {
	boardIDStr := c.Query("board_id")
	if boardIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "board_id required"})
		return
	}

	boardID, err := strconv.ParseUint(boardIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid board_id"})
		return
	}

	token, err := c.Cookie("tk")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	userID, err := app.GetServices().AuthenticationService.VerifyToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if !app.GetServices().BoardService.HasViewAccess(uint(boardID), userID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "no access to this board"})
		return
	}

	permission, err := app.GetServices().BoardService.GetUserPermission(uint(boardID), userID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "no access to this board"})
		return
	}

	board, err := app.GetServices().BoardService.GetBoard(uint(boardID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "board not found"})
		return
	}

	user, err := app.GetServices().AuthenticationService.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	client := &boardsvc.Client{
		Conn:       conn,
		BoardID:    board.ID,
		Username:   user.Username,
		Send:       make(chan []byte, 256),
		Permission: permission,
	}

	hub := app.GetServices().BoardService.GetHub()
	hub.Register(client)

	app.GetServices().BoardService.GetCanvasManager().RegisterClient(board.ID)

	content := app.GetServices().BoardService.GetCanvasManager().GetContent(board.ID)
	if content != nil {
		historyMsg, _ := json.Marshal(map[string]interface{}{
			"type":    "history",
			"content": content,
		})
		client.Send <- historyMsg
	}

	go writePump(client)
	go readPump(app, hub, client)
}

func readPump(app WSApp, hub *boardsvc.Hub, c *boardsvc.Client) {
	defer func() {
		hub.Unregister(c)
		app.GetServices().BoardService.GetCanvasManager().UnregisterClient(c.BoardID)
		c.Conn.Close()
	}()

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		var msg WsMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "draw":
			if c.Permission == "owner" || c.Permission == "editor" {
				var payload DrawPayload
				if err := json.Unmarshal(message, &payload); err == nil {
					app.GetServices().BoardService.GetCanvasManager().ApplyDraw(c.BoardID, payload.X1, payload.Y1, payload.X2, payload.Y2, payload.Color, payload.Size, payload.Tool)
				}
				hub.Broadcast(c.BoardID, message, c.Conn)
			}
		case "text":
			if c.Permission == "owner" || c.Permission == "editor" {
				var payload TextPayload
				if err := json.Unmarshal(message, &payload); err == nil {
					app.GetServices().BoardService.GetCanvasManager().ApplyText(c.BoardID, payload.X, payload.Y, payload.Text, payload.Color, payload.Size)
				}
				hub.Broadcast(c.BoardID, message, c.Conn)
			}
		case "cursor":
			var payload CursorPayload
			if err := json.Unmarshal(message, &payload); err != nil {
				log.Println(err)
			}
			payloadWithUsername, _ := json.Marshal(map[string]interface{}{
				"type":     "cursor",
				"x":        payload.X,
				"y":        payload.Y,
				"color":    payload.Color,
				"username": c.Username,
			})
			hub.Broadcast(c.BoardID, payloadWithUsername, c.Conn)
		case "clear":
			if c.Permission == "owner" || c.Permission == "editor" {
				app.GetServices().BoardService.GetCanvasManager().ClearCanvas(c.BoardID)
				hub.Broadcast(c.BoardID, message, c.Conn)
			}
		}
	}
}

func writePump(c *boardsvc.Client) {
	defer c.Conn.Close()
	for {
		message, ok := <-c.Send
		if !ok {
			c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}
		if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
			return
		}
	}
}

func getUserIDWithUsername(app App, c *gin.Context) (uint, string, bool) {
	token, err := c.Cookie("tk")
	if err != nil {
		return 0, "", false
	}
	userID, err := app.GetServices().AuthenticationService.VerifyToken(token)
	if err != nil {
		return 0, "", false
	}
	user, err := app.GetServices().AuthenticationService.GetUserByID(userID)
	if err != nil {
		return 0, "", false
	}
	return userID, user.Username, true
}

func getBoardAccessHandler(app App, c *gin.Context) {
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

	permission, err := app.GetServices().BoardService.GetUserPermission(uint(boardID), userID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "no access to this board"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"permission": permission})
}

func getBoardMembersHandler(app App, c *gin.Context) {
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

	if !app.GetServices().BoardService.HasViewAccess(uint(boardID), userID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "no access to this board"})
		return
	}

	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	members, total, err := app.GetServices().BoardService.GetBoardMembersPaginated(uint(boardID), offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var response []MemberResponse
	for _, m := range members {
		response = append(response, MemberResponse{
			ID:       m.ID,
			UserID:   m.UserID,
			Username: m.Username,
			Role:     m.Role,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"members": response,
		"total":   total,
		"offset":  offset,
		"limit":   limit,
	})
}

func addBoardMemberHandler(app App, c *gin.Context) {
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

	var req AddMemberRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	users, err := app.GetServices().AuthenticationService.SearchUsers(req.Username, 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var targetUserID uint
	found := false
	for _, u := range users {
		if u.Username == req.Username {
			targetUserID = u.ID
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	role := req.Role
	if role == "" {
		role = "viewer"
	}

	err = app.GetServices().BoardService.UserRequestAddMember(uint(boardID), userID, targetUserID, role)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "member added"})
}

func removeBoardMemberHandler(app App, c *gin.Context) {
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

	targetUserID, err := strconv.ParseUint(c.Param("userId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	err = app.GetServices().BoardService.UserRequestRemoveMember(uint(boardID), userID, uint(targetUserID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "member removed"})
}

func updateBoardMemberHandler(app App, c *gin.Context) {
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

	targetUserID, err := strconv.ParseUint(c.Param("userId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var req UpdateMemberRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = app.GetServices().BoardService.UserRequestUpdateMemberRole(uint(boardID), userID, uint(targetUserID), req.Role)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "member updated"})
}

func RegisterHandlers(app App) {
	router := app.GetRouter()
	router.GET("/api/sync-board", func(c *gin.Context) { wsHandler(app, c) })
	router.POST("/api/boards", func(c *gin.Context) { createBoardHandler(app, c) })
	router.GET("/api/boards", func(c *gin.Context) { getBoardsHandler(app, c) })
	router.GET("/api/boards/:id", func(c *gin.Context) { getBoardHandler(app, c) })
	router.GET("/api/boards/:id/access", func(c *gin.Context) { getBoardAccessHandler(app, c) })
	router.GET("/api/boards/:id/members", func(c *gin.Context) { getBoardMembersHandler(app, c) })
	router.POST("/api/boards/:id/members", func(c *gin.Context) { addBoardMemberHandler(app, c) })
	router.PATCH("/api/boards/:id/members/:userId", func(c *gin.Context) { updateBoardMemberHandler(app, c) })
	router.DELETE("/api/boards/:id/members/:userId", func(c *gin.Context) { removeBoardMemberHandler(app, c) })
	router.PATCH("/api/boards/:id", func(c *gin.Context) { updateBoardHandler(app, c) })
	router.POST("/api/boards/:id/tags", func(c *gin.Context) { addTagsHandler(app, c) })
	router.DELETE("/api/boards/:id", func(c *gin.Context) { deleteBoardHandler(app, c) })
}
