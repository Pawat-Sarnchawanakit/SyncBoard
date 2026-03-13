package team

import (
	"errors"
	"strings"
	"sync-board/models"
)

type App interface {
	GetDatastore() *models.DataStore
}

type TeamService struct {
	app App
}

func NewTeamService(app App) (*TeamService, error) {
	return &TeamService{app: app}, nil
}

func parseTags(tags string) []string {
	if tags == "" {
		return []string{}
	}
	var result []string
	for _, t := range strings.Split(tags, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			result = append(result, t)
		}
	}
	return result
}

func joinTags(tags []string) string {
	return strings.Join(tags, ",")
}

type UpdateTeamInput struct {
	Title       *string
	Description *string
	Tags        *string
}

func (s *TeamService) CreateTeam(title, description, tags string, ownerID uint) (*models.Team, error) {
	if title == "" {
		return nil, errors.New("title is required")
	}
	if len(title) > 128 {
		return nil, errors.New("title must be 128 characters or less")
	}
	if len(description) > 500 {
		return nil, errors.New("description must be 500 characters or less")
	}
	if len(tags) > 500 {
		return nil, errors.New("tags must be 500 characters or less")
	}

	datastore := s.app.GetDatastore()
	team := models.Team{
		Title:       title,
		Description: description,
		Tags:        tags,
		OwnerID:     ownerID,
	}
	if err := datastore.GormDB.Create(&team).Error; err != nil {
		return nil, err
	}

	member := models.TeamMember{
		TeamID: team.ID,
		UserID: ownerID,
		Role:   models.TeamRoleOwner,
	}
	if err := datastore.GormDB.Create(&member).Error; err != nil {
		return nil, err
	}

	return &team, nil
}

func (s *TeamService) UpdateTeam(id uint, ownerID uint, input UpdateTeamInput) (*models.Team, error) {
	datastore := s.app.GetDatastore()
	team := models.Team{}
	if err := datastore.GormDB.Where("id = ? AND owner_id = ?", id, ownerID).First(&team).Error; err != nil {
		return nil, err
	}

	if input.Title != nil {
		if *input.Title == "" {
			return nil, errors.New("title is required")
		}
		if len(*input.Title) > 128 {
			return nil, errors.New("title must be 128 characters or less")
		}
		team.Title = *input.Title
	}
	if input.Description != nil {
		if len(*input.Description) > 500 {
			return nil, errors.New("description must be 500 characters or less")
		}
		team.Description = *input.Description
	}
	if input.Tags != nil {
		if len(*input.Tags) > 500 {
			return nil, errors.New("tags must be 500 characters or less")
		}
		team.Tags = *input.Tags
	}

	if err := datastore.GormDB.Save(&team).Error; err != nil {
		return nil, err
	}
	return &team, nil
}

func (s *TeamService) AddTags(id uint, ownerID uint, newTags []string) (*models.Team, error) {
	if len(newTags) == 0 {
		return nil, errors.New("at least one tag is required")
	}
	for _, tag := range newTags {
		if len(tag) > 50 {
			return nil, errors.New("each tag must be 50 characters or less")
		}
	}

	datastore := s.app.GetDatastore()
	team := models.Team{}
	if err := datastore.GormDB.Where("id = ? AND owner_id = ?", id, ownerID).First(&team).Error; err != nil {
		return nil, err
	}

	currentTags := parseTags(team.Tags)
	for _, newTag := range newTags {
		newTag = strings.TrimSpace(newTag)
		if newTag == "" {
			continue
		}
		found := false
		for _, t := range currentTags {
			if t == newTag {
				found = true
				break
			}
		}
		if !found {
			currentTags = append(currentTags, newTag)
		}
	}
	team.Tags = joinTags(currentTags)

	if err := datastore.GormDB.Save(&team).Error; err != nil {
		return nil, err
	}
	return &team, nil
}

func (s *TeamService) DeleteTeam(id uint, ownerID uint) error {
	datastore := s.app.GetDatastore()

	team := models.Team{}
	if err := datastore.GormDB.Where("id = ? AND owner_id = ?", id, ownerID).First(&team).Error; err != nil {
		return errors.New("team not found or unauthorized")
	}

	if err := datastore.GormDB.Where("team_id = ?", id).Delete(&models.TeamBoard{}).Error; err != nil {
		return err
	}

	if err := datastore.GormDB.Where("team_id = ?", id).Delete(&models.TeamMember{}).Error; err != nil {
		return err
	}

	result := datastore.GormDB.Where("id = ? AND owner_id = ?", id, ownerID).Delete(&models.Team{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("team not found")
	}
	return nil
}

func (s *TeamService) GetTeam(id uint) (*models.Team, error) {
	datastore := s.app.GetDatastore()
	team := models.Team{}
	if err := datastore.GormDB.First(&team, id).Error; err != nil {
		return nil, errors.New("team not found")
	}
	return &team, nil
}

func (s *TeamService) GetTeamByIDAndOwner(id uint, ownerID uint) (*models.Team, error) {
	datastore := s.app.GetDatastore()
	team := models.Team{}
	if err := datastore.GormDB.Where("id = ? AND owner_id = ?", id, ownerID).First(&team).Error; err != nil {
		return nil, err
	}
	return &team, nil
}

func (s *TeamService) IsTeamMember(teamID uint, userID uint) bool {
	datastore := s.app.GetDatastore()
	var member models.TeamMember
	err := datastore.GormDB.Where("team_id = ? AND user_id = ?", teamID, userID).First(&member).Error
	return err == nil
}

func (s *TeamService) IsTeamOwner(teamID uint, userID uint) bool {
	datastore := s.app.GetDatastore()
	var member models.TeamMember
	err := datastore.GormDB.Where("team_id = ? AND user_id = ? AND role = ?", teamID, userID, models.TeamRoleOwner).First(&member).Error
	return err == nil
}

type teamMemberResponse struct {
	ID       uint
	UserID   uint
	Username string
	Role     string
}

func (s *TeamService) GetTeamMembers(teamID uint) ([]teamMemberResponse, error) {
	datastore := s.app.GetDatastore()
	team, err := s.GetTeam(teamID)
	if err != nil {
		return nil, errors.New("team not found")
	}

	var members []teamMemberResponse

	var ownerUser models.User
	_ = datastore.GormDB.First(&ownerUser, team.OwnerID)
	members = append(members, teamMemberResponse{
		ID:       team.OwnerID,
		UserID:   team.OwnerID,
		Username: ownerUser.Username,
		Role:     models.TeamRoleOwner,
	})

	var teamMembers []models.TeamMember
	if err := datastore.GormDB.Where("team_id = ?", teamID).Find(&teamMembers).Error; err != nil {
		return nil, err
	}

	for _, m := range teamMembers {
		if m.UserID == team.OwnerID {
			continue
		}
		var user models.User
		if err := datastore.GormDB.First(&user, m.UserID).Error; err != nil {
			continue
		}
		members = append(members, teamMemberResponse{
			ID:       m.ID,
			UserID:   m.UserID,
			Username: user.Username,
			Role:     m.Role,
		})
	}

	return members, nil
}

func (s *TeamService) GetTeamMembersPaginated(teamID uint, offset, limit int) ([]teamMemberResponse, int, error) {
	datastore := s.app.GetDatastore()
	team, err := s.GetTeam(teamID)
	if err != nil {
		return nil, 0, errors.New("team not found")
	}

	var total int64 = 1
	var ownerUser models.User
	_ = datastore.GormDB.First(&ownerUser, team.OwnerID)

	var teamMembers []models.TeamMember
	query := datastore.GormDB.Model(&models.TeamMember{}).Where("team_id = ?", teamID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	total++

	if err := query.Offset(offset).Limit(limit).Find(&teamMembers).Error; err != nil {
		return nil, 0, err
	}

	var members []teamMemberResponse

	if offset == 0 {
		members = append(members, teamMemberResponse{
			ID:       team.OwnerID,
			UserID:   team.OwnerID,
			Username: ownerUser.Username,
			Role:     models.TeamRoleOwner,
		})
		offset--
		limit--
	}

	for _, m := range teamMembers {
		if limit <= 0 {
			break
		}
		if m.UserID == team.OwnerID {
			continue
		}
		var user models.User
		if err := datastore.GormDB.First(&user, m.UserID).Error; err != nil {
			continue
		}
		members = append(members, teamMemberResponse{
			ID:       m.ID,
			UserID:   m.UserID,
			Username: user.Username,
			Role:     m.Role,
		})
		limit--
	}

	return members, int(total), nil
}

func (s *TeamService) AddTeamMember(teamID uint, requestUserID uint, targetUserID uint) error {
	team, err := s.GetTeam(teamID)
	if err != nil {
		return errors.New("team not found")
	}
	if team.OwnerID != requestUserID {
		return errors.New("only owner can add members")
	}
	if targetUserID == team.OwnerID {
		return errors.New("cannot add owner as member")
	}
	if s.IsTeamMember(teamID, targetUserID) {
		return errors.New("user is already a member")
	}

	datastore := s.app.GetDatastore()
	member := models.TeamMember{
		TeamID: teamID,
		UserID: targetUserID,
		Role:   models.TeamRoleMember,
	}
	return datastore.GormDB.Create(&member).Error
}

func (s *TeamService) RemoveTeamMember(teamID uint, requestUserID uint, targetUserID uint) error {
	team, err := s.GetTeam(teamID)
	if err != nil {
		return errors.New("team not found")
	}
	if team.OwnerID != requestUserID {
		return errors.New("only owner can remove members")
	}
	if targetUserID == team.OwnerID {
		return errors.New("cannot remove owner")
	}

	datastore := s.app.GetDatastore()
	result := datastore.GormDB.Where("team_id = ? AND user_id = ?", teamID, targetUserID).Delete(&models.TeamMember{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("member not found")
	}
	return nil
}

func (s *TeamService) UpdateTeamMemberRole(teamID uint, requestUserID uint, targetUserID uint, newRole string) error {
	team, err := s.GetTeam(teamID)
	if err != nil {
		return errors.New("team not found")
	}
	if team.OwnerID != requestUserID {
		return errors.New("only owner can update members")
	}
	if targetUserID == team.OwnerID {
		return errors.New("cannot update owner role")
	}
	if newRole != models.TeamRoleOwner && newRole != models.TeamRoleMember {
		newRole = models.TeamRoleMember
	}

	datastore := s.app.GetDatastore()
	var member models.TeamMember
	err = datastore.GormDB.Where("team_id = ? AND user_id = ?", teamID, targetUserID).First(&member).Error
	if err != nil {
		return errors.New("member not found")
	}

	member.Role = newRole
	return datastore.GormDB.Save(&member).Error
}

type BoardRestrictions struct {
	CanGrantPermission bool
	CanDelete          bool
	CanEditMetadata    bool
}

func (s *TeamService) CreateTeamBoard(teamID uint, targetUserID uint, title, description, tags string, requestUserID uint) (*models.Board, error) {
	team, err := s.GetTeam(teamID)
	if err != nil {
		return nil, errors.New("team not found")
	}
	if team.OwnerID != requestUserID {
		return nil, errors.New("only owner can create team boards")
	}
	if !s.IsTeamMember(teamID, targetUserID) {
		return nil, errors.New("target user must be a team member")
	}
	if targetUserID == team.OwnerID {
		return nil, errors.New("cannot create board for owner")
	}

	if title == "" {
		return nil, errors.New("title is required")
	}
	if len(title) > 128 {
		return nil, errors.New("title must be 128 characters or less")
	}
	if len(description) > 500 {
		return nil, errors.New("description must be 500 characters or less")
	}
	if len(tags) > 500 {
		return nil, errors.New("tags must be 500 characters or less")
	}

	datastore := s.app.GetDatastore()

	board := models.Board{
		Title:       title,
		Description: description,
		Tags:        tags,
		OwnerID:     requestUserID,
		TeamID:      teamID,
	}
	if err := datastore.GormDB.Create(&board).Error; err != nil {
		return nil, err
	}

	teamBoard := models.TeamBoard{
		TeamID:  teamID,
		BoardID: board.ID,
	}
	if err := datastore.GormDB.Create(&teamBoard).Error; err != nil {
		return nil, err
	}

	member := models.BoardMember{
		BoardID:            board.ID,
		UserID:             targetUserID,
		Role:               models.RoleEditor,
		CanGrantPermission: true,
		CanDelete:          false,
		CanEditMetadata:    false,
	}
	if err := datastore.GormDB.Create(&member).Error; err != nil {
		return nil, err
	}

	return &board, nil
}

type TeamBoardWithOwner struct {
	models.Board
	OwnerName string `json:"ownerName"`
}

func (s *TeamService) GetTeamBoards(teamID uint, offset, limit int) ([]TeamBoardWithOwner, int, error) {
	datastore := s.app.GetDatastore()

	_, err := s.GetTeam(teamID)
	if err != nil {
		return nil, 0, errors.New("team not found")
	}

	var total int64
	var teamBoards []models.TeamBoard
	query := datastore.GormDB.Model(&models.TeamBoard{}).Where("team_id = ?", teamID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Offset(offset).Limit(limit).Find(&teamBoards).Error; err != nil {
		return nil, 0, err
	}

	if len(teamBoards) == 0 {
		return []TeamBoardWithOwner{}, int(total), nil
	}

	boardIDs := make([]uint, len(teamBoards))
	for i, tb := range teamBoards {
		boardIDs[i] = tb.BoardID
	}

	var boards []models.Board
	if err := datastore.GormDB.Where("id IN ?", boardIDs).Find(&boards).Error; err != nil {
		return nil, 0, err
	}

	var ownerUsers = make(map[uint]string)
	for _, b := range boards {
		if _, ok := ownerUsers[b.OwnerID]; !ok {
			var user models.User
			if err := datastore.GormDB.First(&user, b.OwnerID).Error; err == nil {
				ownerUsers[b.OwnerID] = user.Username
			}
		}
	}

	var result []TeamBoardWithOwner
	for _, b := range boards {
		result = append(result, TeamBoardWithOwner{
			Board:     b,
			OwnerName: ownerUsers[b.OwnerID],
		})
	}

	return result, int(total), nil
}

func (s *TeamService) GetUserTeams(userID uint, offset, limit int) ([]models.Team, int, error) {
	datastore := s.app.GetDatastore()

	var teamMembers []models.TeamMember
	if err := datastore.GormDB.Where("user_id = ?", userID).Find(&teamMembers).Error; err != nil {
		return nil, 0, err
	}

	teamIDs := make([]uint, len(teamMembers))
	for i, m := range teamMembers {
		teamIDs[i] = m.TeamID
	}

	if len(teamIDs) == 0 {
		return []models.Team{}, 0, nil
	}

	var total int64
	if err := datastore.GormDB.Model(&models.Team{}).Where("id IN ?", teamIDs).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var teams []models.Team
	if err := datastore.GormDB.Where("id IN ?", teamIDs).Order("created_at desc").Offset(offset).Limit(limit).Find(&teams).Error; err != nil {
		return nil, 0, err
	}

	type teamWithRole struct {
		models.Team
		Role string `json:"role"`
	}

	var result []teamWithRole
	roleMap := make(map[uint]string)
	for _, m := range teamMembers {
		roleMap[m.TeamID] = m.Role
	}

	for _, t := range teams {
		result = append(result, teamWithRole{
			Team: t,
			Role: roleMap[t.ID],
		})
	}

	var teamsResult []models.Team
	for _, tr := range result {
		teamsResult = append(teamsResult, tr.Team)
	}

	return teamsResult, int(total), nil
}

func (s *TeamService) GetTeamBoardMemberRestrictions(boardID uint, userID uint) (*BoardRestrictions, error) {
	datastore := s.app.GetDatastore()
	var member models.BoardMember
	err := datastore.GormDB.Where("board_id = ? AND user_id = ?", boardID, userID).First(&member).Error
	if err != nil {
		return nil, errors.New("member not found")
	}
	return &BoardRestrictions{
		CanGrantPermission: member.CanGrantPermission,
		CanDelete:          member.CanDelete,
		CanEditMetadata:    member.CanEditMetadata,
	}, nil
}

func (s *TeamService) UpdateTeamBoardMemberRestrictions(boardID uint, userID uint, requestUserID uint, restrictions BoardRestrictions) error {
	datastore := s.app.GetDatastore()

	board := models.Board{}
	if err := datastore.GormDB.First(&board, boardID).Error; err != nil {
		return errors.New("board not found")
	}
	if board.TeamID == 0 {
		return errors.New("not a team board")
	}

	team, err := s.GetTeam(board.TeamID)
	if err != nil {
		return errors.New("team not found")
	}
	if team.OwnerID != requestUserID {
		return errors.New("only team owner can update restrictions")
	}

	var member models.BoardMember
	err = datastore.GormDB.Where("board_id = ? AND user_id = ?", boardID, userID).First(&member).Error
	if err != nil {
		return errors.New("member not found")
	}

	member.CanGrantPermission = restrictions.CanGrantPermission
	member.CanDelete = restrictions.CanDelete
	member.CanEditMetadata = restrictions.CanEditMetadata

	return datastore.GormDB.Save(&member).Error
}

func (s *TeamService) GetTeamMembersForUser(userID uint) ([]models.TeamMember, error) {
	datastore := s.app.GetDatastore()
	var members []models.TeamMember
	if err := datastore.GormDB.Where("user_id = ?", userID).Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}
