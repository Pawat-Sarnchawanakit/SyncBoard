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

type TeamInfo struct {
	ID          uint
	Title       string
	Description string
	Tags        string
	OwnerID     uint
	CreatedAt   time.Time
}

type TeamWithRole struct {
	TeamInfo
	Role string
}

type BoardInfo struct {
	ID          uint
	Title       string
	Description string
	Tags        string
	OwnerID     uint
	TeamID      uint
	CreatedAt   time.Time
}

func (s *TeamService) CreateTeam(title, description, tags string, ownerID uint) (*TeamInfo, error) {
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

	return &TeamInfo{
		ID:          team.ID,
		Title:       team.Title,
		Description: team.Description,
		Tags:        team.Tags,
		OwnerID:     team.OwnerID,
		CreatedAt:   team.CreatedAt,
	}, nil
}

func (s *TeamService) UpdateTeam(id uint, ownerID uint, title, description, tags string) (*TeamInfo, error) {
	datastore := s.app.GetDatastore()
	team := models.Team{}
	if err := datastore.GormDB.Where("id = ? AND owner_id = ?", id, ownerID).First(&team).Error; err != nil {
		return nil, err
	}

	if title != "" {
		if len(title) > 128 {
			return nil, errors.New("title must be 128 characters or less")
		}
		team.Title = title
	}
	if description != "" {
		if len(description) > 500 {
			return nil, errors.New("description must be 500 characters or less")
		}
		team.Description = description
	}
	if tags != "" {
		if len(tags) > 500 {
			return nil, errors.New("tags must be 500 characters or less")
		}
		team.Tags = tags
	}

	if err := datastore.GormDB.Save(&team).Error; err != nil {
		return nil, err
	}
	return &TeamInfo{
		ID:          team.ID,
		Title:       team.Title,
		Description: team.Description,
		Tags:        team.Tags,
		OwnerID:     team.OwnerID,
		CreatedAt:   team.CreatedAt,
	}, nil
}

func (s *TeamService) UserRequestUpdateTeam(id uint, userID uint, title, description, tags string) (*TeamInfo, error) {
	team, err := s.GetTeam(id)
	if err != nil {
		return nil, err
	}

	if team.OwnerID != userID {
		return nil, errors.New("unauthorized to update team")
	}

	return s.UpdateTeam(id, userID, title, description, tags)
}

func (s *TeamService) AddTags(id uint, ownerID uint, newTags []string) (*TeamInfo, error) {
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
	return &TeamInfo{
		ID:          team.ID,
		Title:       team.Title,
		Description: team.Description,
		Tags:        team.Tags,
		OwnerID:     team.OwnerID,
		CreatedAt:   team.CreatedAt,
	}, nil
}

func (s *TeamService) UserRequestAddTeamTags(id uint, userID uint, newTags []string) (*TeamInfo, error) {
	team, err := s.GetTeam(id)
	if err != nil {
		return nil, err
	}

	if team.OwnerID != userID {
		return nil, errors.New("unauthorized to add tags")
	}

	return s.AddTags(id, userID, newTags)
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

func (s *TeamService) UserRequestDeleteTeam(id uint, userID uint) error {
	return s.DeleteTeam(id, userID)
}

func (s *TeamService) GetTeam(id uint) (*TeamInfo, error) {
	datastore := s.app.GetDatastore()
	team := models.Team{}
	if err := datastore.GormDB.First(&team, id).Error; err != nil {
		return nil, errors.New("team not found")
	}
	return &TeamInfo{
		ID:          team.ID,
		Title:       team.Title,
		Description: team.Description,
		Tags:        team.Tags,
		OwnerID:     team.OwnerID,
		CreatedAt:   team.CreatedAt,
	}, nil
}

func (s *TeamService) GetTeamByIDAndMember(id uint, memberID uint) (*TeamInfo, error) {
	datastore := s.app.GetDatastore()
	team := models.Team{}
	if err := datastore.GormDB.
		Model(&models.Team{}).
		Joins("JOIN team_members ON team_members.team_id = teams.id").
		Where("teams.id = ? AND team_members.user_id = ?", id, memberID).
		Take(&team).Error; err != nil {
		return nil, err
	}
	return &TeamInfo{
		ID:          team.ID,
		Title:       team.Title,
		Description: team.Description,
		Tags:        team.Tags,
		OwnerID:     team.OwnerID,
		CreatedAt:   team.CreatedAt,
	}, nil
}

func (s *TeamService) GetTeamByIDAndOwner(id uint, ownerID uint) (*TeamInfo, error) {
	datastore := s.app.GetDatastore()
	team := models.Team{}
	if err := datastore.GormDB.Where("id = ? AND owner_id = ?", id, ownerID).First(&team).Error; err != nil {
		return nil, err
	}
	return &TeamInfo{
		ID:          team.ID,
		Title:       team.Title,
		Description: team.Description,
		Tags:        team.Tags,
		OwnerID:     team.OwnerID,
		CreatedAt:   team.CreatedAt,
	}, nil
}

type TeamWithMemberRole struct {
	TeamInfo
	Role string
}

func (s *TeamService) UserRequestGetTeam(id uint, userID uint) (*TeamWithMemberRole, error) {
	team, err := s.GetTeam(id)
	if err != nil {
		return nil, err
	}

	role := "member"
	if team.OwnerID == userID {
		role = "owner"
	} else {
		if !s.IsTeamMember(id, userID) {
			return nil, errors.New("not a member of this team")
		}
	}

	return &TeamWithMemberRole{
		TeamInfo: *team,
		Role:     role,
	}, nil
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

	type result struct {
		UserID   uint
		Username string
		Role     string
	}

	var results []result
	if err := datastore.GormDB.
		Model(&models.User{}).
		Select("COALESCE(team_members.user_id, teams.owner_id) as user_id, users.username, COALESCE(team_members.role, 'owner') as role").
		Joins("LEFT JOIN team_members ON users.id = team_members.user_id AND team_members.team_id = ?", teamID).
		Joins("JOIN teams ON teams.id = ?", teamID).
		Where("users.id = teams.owner_id OR team_members.team_id = ?", teamID).
		Scan(&results).Error; err != nil {
		return nil, err
	}

	var members []teamMemberResponse
	for _, r := range results {
		members = append(members, teamMemberResponse{
			ID:       r.UserID,
			UserID:   r.UserID,
			Username: r.Username,
			Role:     r.Role,
		})
	}

	return members, nil
}

func (s *TeamService) GetTeamMembersPaginated(teamID uint, searchQuery string, offset, limit int) ([]teamMemberResponse, int, error) {
	datastore := s.app.GetDatastore()

	type result struct {
		UserID   uint
		Username string
		Role     string
	}

	var results []result
	query := datastore.GormDB.
		Model(&models.TeamMember{}).
		Select("COALESCE(team_members.user_id, teams.owner_id) as user_id, users.username, COALESCE(team_members.role, 'owner') as role").
		Joins("JOIN users ON users.id = team_members.user_id AND team_members.team_id = ?", teamID).
		Joins("JOIN teams ON teams.id = ?", teamID).
		Where("users.id = teams.owner_id OR team_members.team_id = ?", teamID)

	if searchQuery != "" {
		searchPattern := "%" + strings.ToUpper(searchQuery) + "%"
		query = query.Where("UPPER(users.username) LIKE ?", searchPattern)
	}

	if err := query.Find(&results).Error; err != nil {
		return nil, 0, err
	}

	total := len(results)

	if offset >= total {
		return []teamMemberResponse{}, 0, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	var members []teamMemberResponse
	for i := offset; i < end; i++ {
		members = append(members, teamMemberResponse{
			ID:       results[i].UserID,
			UserID:   results[i].UserID,
			Username: results[i].Username,
			Role:     results[i].Role,
		})
	}

	return members, total, nil
}

func (s *TeamService) UserRequestGetTeamMembers(teamID uint, userID uint, searchQuery string, offset, limit int) ([]teamMemberResponse, int, error) {
	team, err := s.GetTeam(teamID)
	if err != nil {
		return nil, 0, errors.New("team not found")
	}

	if team.OwnerID != userID && !s.IsTeamMember(teamID, userID) {
		return nil, 0, errors.New("not a member of this team")
	}

	return s.GetTeamMembersPaginated(teamID, searchQuery, offset, limit)
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
	result := datastore.GormDB.Model(&models.TeamMember{}).Where("team_id = ? AND user_id = ?", teamID, targetUserID).Delete(&models.TeamMember{})
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

func (s *TeamService) SearchTeamMembers(teamID uint, query string) ([]models.User, error) {
	datastore := s.app.GetDatastore()

	var users []models.User
	if err := datastore.GormDB.
		Model(&models.TeamMember{}).
		Select("users.id, users.username").
		Joins("JOIN users ON users.id = team_members.user_id AND team_members.team_id = ?", teamID).
		Where("users.username LIKE ?", "%"+query+"%").
		Limit(10).
		Find(&users).Error; err != nil {
		return nil, err
	}

	return users, nil
}

type BoardRestrictions struct {
	CanGrantPermission bool
	CanDelete          bool
	CanEditMetadata    bool
	CanDraw            bool
}

func (s *TeamService) CreateTeamBoard(teamID uint, targetUserID uint, title, description, tags string, requestUserID uint) (*BoardInfo, error) {
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
		TeamID:       teamID,
		BoardID:      board.ID,
		BoardOwnerID: targetUserID,
	}
	teamBoard.SetCanGrantPermission(true)
	teamBoard.SetCanDelete(false)
	teamBoard.SetCanEditMetadata(false)
	teamBoard.SetCanDraw(true)
	if err := datastore.GormDB.Create(&teamBoard).Error; err != nil {
		return nil, err
	}

	return &BoardInfo{
		ID:          board.ID,
		Title:       board.Title,
		Description: board.Description,
		Tags:        board.Tags,
		OwnerID:     board.OwnerID,
		TeamID:      board.TeamID,
		CreatedAt:   board.CreatedAt,
	}, nil
}

type TeamBoardWithOwner struct {
	BoardID      uint
	Title        string
	Description  string
	Tags         string
	OwnerID      uint
	OwnerName    string
	BoardOwnerID uint
	Permissions  uint8
	CreatedAt    time.Time
}

func (s *TeamService) GetTeamBoards(teamID uint, searchQuery string, offset, limit int) ([]TeamBoardWithOwner, error) {
	datastore := s.app.GetDatastore()

	type result struct {
		models.TeamBoard
		BoardID     uint
		Title       string
		Description string
		Tags        string
		OwnerID     uint
		OwnerName   string
		CreatedAt   time.Time
	}

	var boardResults []result
	query := datastore.GormDB.
		Model(&models.TeamBoard{}).
		Select("team_boards.*, boards.id as board_id, boards.title, boards.description, boards.tags, boards.owner_id, users.username as owner_name, boards.created_at").
		Joins("JOIN boards ON boards.id = team_boards.board_id").
		Joins("JOIN users ON team_boards.board_owner_id = users.id").
		Where("team_boards.team_id = ?", teamID).
		Where("boards.deleted_at IS NULL").
		Where("users.deleted_at IS NULL")

	if searchQuery != "" {
		searchPattern := "%" + strings.ToUpper(searchQuery) + "%"
		query = query.Where("(UPPER(boards.title) LIKE ? OR UPPER(boards.description) LIKE ?)", searchPattern, searchPattern)
	}

	if err := query.Order("team_boards.created_at desc").Offset(offset).Limit(limit).Find(&boardResults).Error; err != nil {
		return nil, err
	}

	var results []TeamBoardWithOwner
	for _, r := range boardResults {
		results = append(results, TeamBoardWithOwner{
			BoardID:      r.BoardID,
			Title:        r.Title,
			Description:  r.Description,
			Tags:         r.Tags,
			OwnerID:      r.OwnerID,
			OwnerName:    r.OwnerName,
			Permissions:  r.Permissions,
			BoardOwnerID: r.BoardOwnerID,
			CreatedAt:    r.CreatedAt,
		})
	}

	return results, nil
}

func (s *TeamService) UserRequestGetTeamBoards(teamID uint, userID uint, searchQuery string, offset, limit int) ([]TeamBoardWithOwner, error) {
	datastore := s.app.GetDatastore()

	var exists bool
	if err := datastore.GormDB.Model(&models.TeamMember{}).
		Select("count(*) > 0").
		Where("team_id = ?", teamID).
		Where("user_id = ?", userID).
		Find(&exists).Error; err != nil || !exists {
		return nil, errors.New("not a member of this team or team not found")
	}

	return s.GetTeamBoards(teamID, searchQuery, offset, limit)
}

func (s *TeamService) GetUserTeams(userID uint, searchQuery string, offset, limit int) ([]TeamWithRole, int, error) {
	datastore := s.app.GetDatastore()

	type teamWithMember struct {
		models.Team
		Role string
	}

	var teamsWithRole []teamWithMember
	query := datastore.GormDB.
		Model(&models.Team{}).
		Select("teams.*, team_members.role").
		Joins("JOIN team_members ON teams.id = team_members.team_id").
		Where("team_members.user_id = ?", userID).
		Where("team_members.deleted_at IS NULL")

	if searchQuery != "" {
		searchPattern := "%" + strings.ToUpper(searchQuery) + "%"
		query = query.Where("(UPPER(teams.title) LIKE ? OR UPPER(teams.description) LIKE ? OR UPPER(teams.tags) LIKE ?)", searchPattern, searchPattern, searchPattern)
	}

	if err := query.Order("teams.created_at desc").Find(&teamsWithRole).Error; err != nil {
		return nil, 0, err
	}

	total := len(teamsWithRole)

	var result []TeamWithRole
	for _, t := range teamsWithRole {
		role := t.Role
		if role == "" {
			role = "owner"
		}
		result = append(result, TeamWithRole{
			TeamInfo: TeamInfo{
				ID:          t.ID,
				Title:       t.Title,
				Description: t.Description,
				Tags:        t.Tags,
				OwnerID:     t.OwnerID,
				CreatedAt:   t.CreatedAt,
			},
			Role: role,
		})
	}

	if offset >= len(result) {
		return []TeamWithRole{}, 0, nil
	}

	end := offset + limit
	if end > len(result) {
		end = len(result)
	}

	return result[offset:end], total, nil
}

func (s *TeamService) GetTeamBoardOwnerRestrictions(boardID uint) (*BoardRestrictions, error) {
	datastore := s.app.GetDatastore()
	var teamBoard models.TeamBoard
	err := datastore.GormDB.Where("board_id = ?", boardID).First(&teamBoard).Error
	if err != nil {
		return nil, errors.New("team board not found")
	}
	return &BoardRestrictions{
		CanGrantPermission: teamBoard.GetCanGrantPermission(),
		CanDelete:          teamBoard.GetCanDelete(),
		CanEditMetadata:    teamBoard.GetCanEditMetadata(),
		CanDraw:            teamBoard.GetCanDraw(),
	}, nil
}

func (s *TeamService) UserCanViewTeamBoardRestrictions(boardID uint, userID uint) error {
	datastore := s.app.GetDatastore()

	type boardWithTeam struct {
		TeamID  uint
		OwnerID uint
	}

	var board boardWithTeam
	if err := datastore.GormDB.
		Model(&models.Board{}).
		Select("team_id, owner_id").
		Where("id = ?", boardID).
		Scan(&board).Error; err != nil {
		return errors.New("board not found")
	}

	if board.TeamID == 0 {
		return errors.New("not a team board")
	}

	var team models.Team
	if err := datastore.GormDB.First(&team, board.TeamID).Error; err != nil {
		return errors.New("team not found")
	}

	if team.OwnerID != userID {
		return errors.New("only team owner can view restrictions")
	}

	return nil
}

func (s *TeamService) UserCanUpdateTeamBoardRestrictions(boardID uint, requestUserID uint) error {
	datastore := s.app.GetDatastore()

	type boardWithTeam struct {
		TeamID  uint
		OwnerID uint
	}

	var board boardWithTeam
	if err := datastore.GormDB.
		Model(&models.Board{}).
		Select("team_id, owner_id").
		Where("id = ?", boardID).
		Scan(&board).Error; err != nil {
		return errors.New("board not found")
	}

	if board.TeamID == 0 {
		return errors.New("not a team board")
	}

	var team models.Team
	if err := datastore.GormDB.First(&team, board.TeamID).Error; err != nil {
		return errors.New("team not found")
	}

	if team.OwnerID != requestUserID {
		return errors.New("only team owner can update restrictions")
	}

	return nil
}

func (s *TeamService) UpdateTeamBoardOwnerRestrictions(boardID uint, requestUserID uint, restrictions BoardRestrictions) error {
	if err := s.UserCanUpdateTeamBoardRestrictions(boardID, requestUserID); err != nil {
		return err
	}

	datastore := s.app.GetDatastore()

	var teamBoard models.TeamBoard
	err := datastore.GormDB.Where("board_id = ?", boardID).First(&teamBoard).Error
	if err != nil {
		return errors.New("team board not found")
	}

	teamBoard.SetCanGrantPermission(restrictions.CanGrantPermission)
	teamBoard.SetCanDelete(restrictions.CanDelete)
	teamBoard.SetCanEditMetadata(restrictions.CanEditMetadata)
	teamBoard.SetCanDraw(restrictions.CanDraw)

	return datastore.GormDB.Save(&teamBoard).Error
}

func (s *TeamService) ChangeTeamBoardOwner(teamID uint, boardID uint, newOwnerID uint, requestUserID uint) error {
	team, err := s.GetTeam(teamID)
	if err != nil {
		return errors.New("team not found")
	}

	if team.OwnerID != requestUserID {
		return errors.New("only team owner can change board owner")
	}

	if !s.IsTeamMember(teamID, newOwnerID) {
		return errors.New("new owner must be a team member")
	}

	datastore := s.app.GetDatastore()

	var board models.Board
	if err := datastore.GormDB.Where("id = ? AND team_id = ?", boardID, teamID).First(&board).Error; err != nil {
		return errors.New("board not found")
	}

	var teamBoard models.TeamBoard
	if err := datastore.GormDB.Where("board_id = ?", boardID).First(&teamBoard).Error; err != nil {
		return errors.New("team board not found")
	}

	teamBoard.BoardOwnerID = newOwnerID
	if err := datastore.GormDB.Save(&teamBoard).Error; err != nil {
		return err
	}

	return nil
}

func (s *TeamService) GetTeamMembersForUser(userID uint) ([]models.TeamMember, error) {
	datastore := s.app.GetDatastore()
	var members []models.TeamMember
	if err := datastore.GormDB.Where("user_id = ?", userID).Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}
