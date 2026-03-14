package team

import (
	"errors"
	"strings"
	"sync-board/models"
	"time"
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

func (s *TeamService) GetTeamByIDAndMember(id uint, memberID uint) (*models.Team, error) {
	datastore := s.app.GetDatastore()
	team := models.Team{}
	if err := datastore.GormDB.
		Model(&models.Team{}).
		Joins("JOIN team_members ON team_members.team_id = teams.id").
		Where("teams.id = ? AND team_members.user_id = ?", id, memberID).
		Take(&team).Error; err != nil {
		return nil, err
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

	type memberWithUser struct {
		models.TeamMember
		Username string
	}

	var teamMembersWithUsers []memberWithUser
	if err := datastore.GormDB.
		Model(models.TeamMember{}).
		Joins("JOIN users ON users.id = team_members.user_id").
		Where("team_members.team_id = ?", teamID).
		Find(&teamMembersWithUsers).Error; err != nil {
		return nil, err
	}

	var ownerUser models.User
	_ = datastore.GormDB.First(&ownerUser, team.OwnerID)

	var members []teamMemberResponse
	members = append(members, teamMemberResponse{
		ID:       team.OwnerID,
		UserID:   team.OwnerID,
		Username: ownerUser.Username,
		Role:     models.TeamRoleOwner,
	})

	for _, m := range teamMembersWithUsers {
		if m.UserID == team.OwnerID {
			continue
		}
		members = append(members, teamMemberResponse{
			ID:       m.ID,
			UserID:   m.UserID,
			Username: m.Username,
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

	var total int64
	if err := datastore.GormDB.Model(&models.TeamMember{}).Where("team_id = ?", teamID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	total++

	type memberWithUser struct {
		models.TeamMember
		Username string
	}

	var teamMembersWithUsers []memberWithUser
	if err := datastore.GormDB.
		Model(&models.TeamMember{}).
		Joins("JOIN users ON users.id = team_members.user_id").
		Where("team_members.team_id = ?", teamID).
		Order("team_members.created_at asc").
		Offset(offset).Limit(limit).
		Find(&teamMembersWithUsers).Error; err != nil {
		return nil, 0, err
	}

	var ownerUser models.User
	_ = datastore.GormDB.First(&ownerUser, team.OwnerID)

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

	for _, m := range teamMembersWithUsers {
		if limit <= 0 {
			break
		}
		if m.UserID == team.OwnerID {
			continue
		}
		members = append(members, teamMemberResponse{
			ID:       m.ID,
			UserID:   m.UserID,
			Username: m.Username,
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
		TeamID:             teamID,
		BoardID:            board.ID,
		BoardOwnerID:       targetUserID,
		CanGrantPermission: true,
		CanDelete:          false,
		CanEditMetadata:    false,
	}
	if err := datastore.GormDB.Create(&teamBoard).Error; err != nil {
		return nil, err
	}

	member := models.BoardMember{
		BoardID: board.ID,
		UserID:  targetUserID,
		Role:    models.RoleEditor,
	}
	if err := datastore.GormDB.Create(&member).Error; err != nil {
		return nil, err
	}

	return &board, nil
}

type TeamBoardWithOwner struct {
	BoardID      uint
	Title        string
	Description  string
	Tags         string
	OwnerID      uint
	OwnerName    string
	BoardOwnerID uint
	CreatedAt    time.Time
}

func (s *TeamService) GetTeamBoards(teamID uint, offset, limit int) ([]TeamBoardWithOwner, error) {
	datastore := s.app.GetDatastore()

	_, err := s.GetTeam(teamID)
	if err != nil {
		return nil, errors.New("team not found")
	}

	var boardResults []TeamBoardWithOwner
	if err := datastore.GormDB.
		Model(&models.TeamBoard{}).
		Select("boards.id as board_id, boards.created_at as created_at, users.username as owner_name, *").
		Joins("JOIN boards ON boards.id = team_boards.board_id").
		Joins("JOIN users ON team_boards.board_owner_id = users.id").
		Where("team_boards.team_id = ?", teamID).
		Order("team_boards.created_at desc").
		Offset(offset).Limit(limit).
		Scan(&boardResults).Error; err != nil {
		return nil, err
	}

	return boardResults, nil
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
	var teamBoard models.TeamBoard
	err := datastore.GormDB.Where("board_id = ?", boardID).First(&teamBoard).Error
	if err != nil {
		return nil, errors.New("team board not found")
	}
	return &BoardRestrictions{
		CanGrantPermission: teamBoard.CanGrantPermission,
		CanDelete:          teamBoard.CanDelete,
		CanEditMetadata:    teamBoard.CanEditMetadata,
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

	var teamBoard models.TeamBoard
	err = datastore.GormDB.Where("board_id = ?", boardID).First(&teamBoard).Error
	if err != nil {
		return errors.New("team board not found")
	}

	teamBoard.CanGrantPermission = restrictions.CanGrantPermission
	teamBoard.CanDelete = restrictions.CanDelete
	teamBoard.CanEditMetadata = restrictions.CanEditMetadata

	return datastore.GormDB.Save(&teamBoard).Error
}

func (s *TeamService) GetTeamMembersForUser(userID uint) ([]models.TeamMember, error) {
	datastore := s.app.GetDatastore()
	var members []models.TeamMember
	if err := datastore.GormDB.Where("user_id = ?", userID).Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}
