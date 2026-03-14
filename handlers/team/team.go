package team

import (
	"net/http"
	"strconv"
	"sync-board/services"
	teamsvc "sync-board/services/team"
	"time"

	"github.com/gin-gonic/gin"
)

type App interface {
	GetRouter() *gin.Engine
	GetServices() *services.Services
	GetHost() string
}

type CreateTeamRequest struct {
	Title       string `json:"title" binding:"required,min=1,max=128"`
	Description string `json:"description"`
	Tags        string `json:"tags"`
}

type UpdateTeamRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Tags        *string `json:"tags"`
}

type AddMemberRequest struct {
	Username string `json:"username" binding:"required"`
}

type UpdateMemberRequest struct {
	Role string `json:"role" binding:"required"`
}

type CreateTeamBoardRequest struct {
	MemberUsername string `json:"memberUsername" binding:"required"`
	Title          string `json:"title" binding:"required,min=1,max=128"`
	Description    string `json:"description"`
	Tags           string `json:"tags"`
}

type UpdateRestrictionsRequest struct {
	CanGrantPermission *bool `json:"canGrantPermission"`
	CanDelete          *bool `json:"canDelete"`
	CanEditMetadata    *bool `json:"canEditMetadata"`
}

type MemberResponse struct {
	ID       uint   `json:"id"`
	UserID   uint   `json:"userId"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type TeamBoardResponse struct {
	ID             uint   `json:"id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	Tags           string `json:"tags"`
	OwnerID        uint   `json:"ownerId"`
	OwnerName      string `json:"ownerName"`
	TeamID         uint   `json:"teamId"`
	BoardOwnerID   *uint  `json:"boardOwnerId,omitempty"`
	BoardOwnerName string `json:"boardOwnerName,omitempty"`
	CreatedAt      string `json:"createdAt"`
}

type CreateTeamResponse struct {
	ID          uint      `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Tags        string    `json:"tags"`
	CreatedAt   time.Time `json:"createdAt"`
}

type TeamResponse struct {
	ID          uint      `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Tags        string    `json:"tags"`
	OwnerID     uint      `json:"ownerId"`
	CreatedAt   time.Time `json:"createdAt"`
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

func createTeamHandler(app App, c *gin.Context) {
	userID, ok := getUserID(app, c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req CreateTeamRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	team, err := app.GetServices().TeamService.CreateTeam(req.Title, req.Description, req.Tags, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, CreateTeamResponse{
		ID:          team.ID,
		Title:       team.Title,
		Description: team.Description,
		Tags:        team.Tags,
		CreatedAt:   team.CreatedAt,
	})
}

func getTeamsHandler(app App, c *gin.Context) {
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

	teams, total, err := app.GetServices().TeamService.GetUserTeams(userID, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type TeamDTO struct {
		ID          uint      `json:"id"`
		Title       string    `json:"title"`
		Description string    `json:"description"`
		Tags        string    `json:"tags"`
		OwnerID     uint      `json:"ownerId"`
		CreatedAt   time.Time `json:"createdAt"`
		Role        string    `json:"role"`
	}

	var teamsWithRole []TeamDTO
	for _, t := range teams {
		teamsWithRole = append(teamsWithRole, TeamDTO{
			ID:          t.ID,
			Title:       t.Title,
			Description: t.Description,
			Tags:        t.Tags,
			OwnerID:     t.OwnerID,
			CreatedAt:   t.CreatedAt,
			Role:        t.Role,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"teams":  teamsWithRole,
		"total":  total,
		"offset": offset,
		"limit":  limit,
	})
}

func getTeamHandler(app App, c *gin.Context) {
	userID, ok := getUserID(app, c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	teamID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
		return
	}

	team, err := app.GetServices().TeamService.UserRequestGetTeam(uint(teamID), userID)
	if err != nil {
		if err.Error() == "not a member of this team" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "team not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":          team.ID,
		"title":       team.Title,
		"description": team.Description,
		"tags":        team.Tags,
		"ownerId":     team.OwnerID,
		"createdAt":   team.CreatedAt,
		"role":        team.Role,
	})
}

func updateTeamHandler(app App, c *gin.Context) {
	userID, ok := getUserID(app, c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	teamID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
		return
	}

	var req UpdateTeamRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var title, description, tags string
	if req.Title != nil {
		title = *req.Title
	}
	if req.Description != nil {
		description = *req.Description
	}
	if req.Tags != nil {
		tags = *req.Tags
	}

	team, err := app.GetServices().TeamService.UserRequestUpdateTeam(uint(teamID), userID, title, description, tags)
	if err != nil {
		if err.Error() == "unauthorized to update team" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, TeamResponse{
		ID:          team.ID,
		Title:       team.Title,
		Description: team.Description,
		Tags:        team.Tags,
		CreatedAt:   team.CreatedAt,
	})
}

func deleteTeamHandler(app App, c *gin.Context) {
	userID, ok := getUserID(app, c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	teamID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
		return
	}

	err = app.GetServices().TeamService.UserRequestDeleteTeam(uint(teamID), userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

type AddTagsRequest struct {
	Tags []string `json:"tags" binding:"required,min=1"`
}

func addTeamTagsHandler(app App, c *gin.Context) {
	userID, ok := getUserID(app, c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	teamID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
		return
	}

	var req AddTagsRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	team, err := app.GetServices().TeamService.UserRequestAddTeamTags(uint(teamID), userID, req.Tags)
	if err != nil {
		if err.Error() == "unauthorized to add tags" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, TeamResponse{
		ID:          team.ID,
		Title:       team.Title,
		Description: team.Description,
		Tags:        team.Tags,
		CreatedAt:   team.CreatedAt,
	})
}

func getTeamMembersHandler(app App, c *gin.Context) {
	userID, ok := getUserID(app, c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	teamID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
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

	members, total, err := app.GetServices().TeamService.UserRequestGetTeamMembers(uint(teamID), userID, offset, limit)
	if err != nil {
		if err.Error() == "not a member of this team" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
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

func addTeamMemberHandler(app App, c *gin.Context) {
	userID, ok := getUserID(app, c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	teamID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
		return
	}

	var req AddMemberRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	targetUser, err := app.GetServices().AuthenticationService.GetUserByUsername(req.Username)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	err = app.GetServices().TeamService.AddTeamMember(uint(teamID), userID, targetUser.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "member added"})
}

func removeTeamMemberHandler(app App, c *gin.Context) {
	userID, ok := getUserID(app, c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	teamID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
		return
	}

	targetUserID, err := strconv.ParseUint(c.Param("userId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	err = app.GetServices().TeamService.RemoveTeamMember(uint(teamID), userID, uint(targetUserID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "member removed"})
}

func getTeamBoardsHandler(app App, c *gin.Context) {
	userID, ok := getUserID(app, c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	teamID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
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

	boards, err := app.GetServices().TeamService.UserRequestGetTeamBoards(uint(teamID), userID, offset, limit)
	if err != nil {
		if err.Error() == "not a member of this team" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var response []TeamBoardResponse
	for _, b := range boards {
		boardResp := TeamBoardResponse{
			ID:          b.BoardID,
			Title:       b.Title,
			Description: b.Description,
			Tags:        b.Tags,
			OwnerID:     b.OwnerID,
			OwnerName:   b.OwnerName,
			TeamID:      uint(teamID),
			CreatedAt:   b.CreatedAt.Format(time.RFC3339),
		}
		response = append(response, boardResp)
	}

	c.JSON(http.StatusOK, gin.H{
		"boards": response,
		"limit":  limit,
	})
}

func createTeamBoardHandler(app App, c *gin.Context) {
	userID, ok := getUserID(app, c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	teamID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
		return
	}

	var req CreateTeamBoardRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	targetUser, err := app.GetServices().AuthenticationService.GetUserByUsername(req.MemberUsername)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "team member not found"})
		return
	}

	board, err := app.GetServices().TeamService.CreateTeamBoard(uint(teamID), targetUser.ID, req.Title, req.Description, req.Tags, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":          board.ID,
		"title":       board.Title,
		"description": board.Description,
		"tags":        board.Tags,
		"ownerId":     board.OwnerID,
		"teamId":      board.TeamID,
		"createdAt":   board.CreatedAt,
	})
}

func getTeamBoardMemberRestrictionsHandler(app App, c *gin.Context) {
	userID, ok := getUserID(app, c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	boardID, err := strconv.ParseUint(c.Param("boardId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid board id"})
		return
	}

	memberUserID, err := strconv.ParseUint(c.Param("userId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	if err := app.GetServices().TeamService.UserCanViewTeamBoardRestrictions(uint(boardID), userID); err != nil {
		if err.Error() == "board not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "board not found"})
			return
		}
		if err.Error() == "not a team board" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "not a team board"})
			return
		}
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	restrictions, err := app.GetServices().TeamService.GetTeamBoardMemberRestrictions(uint(boardID), uint(memberUserID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"canGrantPermission": restrictions.CanGrantPermission,
		"canDelete":          restrictions.CanDelete,
		"canEditMetadata":    restrictions.CanEditMetadata,
	})
}

func updateTeamBoardMemberRestrictionsHandler(app App, c *gin.Context) {
	userID, ok := getUserID(app, c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	boardID, err := strconv.ParseUint(c.Param("boardId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid board id"})
		return
	}

	memberUserID, err := strconv.ParseUint(c.Param("userId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var req UpdateRestrictionsRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	restrictions := teamsvc.BoardRestrictions{}
	if req.CanGrantPermission != nil {
		restrictions.CanGrantPermission = *req.CanGrantPermission
	}
	if req.CanDelete != nil {
		restrictions.CanDelete = *req.CanDelete
	}
	if req.CanEditMetadata != nil {
		restrictions.CanEditMetadata = *req.CanEditMetadata
	}

	err = app.GetServices().TeamService.UpdateTeamBoardMemberRestrictions(uint(boardID), uint(memberUserID), userID, restrictions)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "restrictions updated"})
}

func RegisterHandlers(app App) {
	router := app.GetRouter()
	router.POST("/api/teams", func(c *gin.Context) { createTeamHandler(app, c) })
	router.GET("/api/teams", func(c *gin.Context) { getTeamsHandler(app, c) })
	router.GET("/api/teams/:id", func(c *gin.Context) { getTeamHandler(app, c) })
	router.PATCH("/api/teams/:id", func(c *gin.Context) { updateTeamHandler(app, c) })
	router.DELETE("/api/teams/:id", func(c *gin.Context) { deleteTeamHandler(app, c) })
	router.POST("/api/teams/:id/tags", func(c *gin.Context) { addTeamTagsHandler(app, c) })
	router.GET("/api/teams/:id/members", func(c *gin.Context) { getTeamMembersHandler(app, c) })
	router.POST("/api/teams/:id/members", func(c *gin.Context) { addTeamMemberHandler(app, c) })
	router.DELETE("/api/teams/:id/members/:userId", func(c *gin.Context) { removeTeamMemberHandler(app, c) })
	router.GET("/api/teams/:id/boards", func(c *gin.Context) { getTeamBoardsHandler(app, c) })
	router.POST("/api/teams/:id/boards", func(c *gin.Context) { createTeamBoardHandler(app, c) })
	router.GET("/api/teams/:id/boards/:boardId/members/:userId/restrictions", func(c *gin.Context) { getTeamBoardMemberRestrictionsHandler(app, c) })
	router.PATCH("/api/teams/:id/boards/:boardId/members/:userId/restrictions", func(c *gin.Context) { updateTeamBoardMemberRestrictionsHandler(app, c) })
}
