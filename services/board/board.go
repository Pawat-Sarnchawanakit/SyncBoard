package board

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"os"
	"strings"
	"sync"
	"sync-board/models"
	"time"

	"github.com/chai2010/webp"
	"github.com/gorilla/websocket"
	"github.com/tfriedel6/canvas"
	"github.com/tfriedel6/canvas/backend/softwarebackend"
)

const (
	CanvasWidth  = 1920
	CanvasHeight = 1080
)

type App interface {
	GetDatastore() *models.DataStore
}

type BoardService struct {
	app           App
	hub           *Hub
	canvasManager *CanvasManager
}

type canvasData struct {
	Backend *softwarebackend.SoftwareBackend
	Canvas  *canvas.Canvas
	Font    *canvas.Font
}

type CanvasManager struct {
	canvases  map[uint]*canvasData
	clients   map[uint]int
	font      *canvas.Font
	mutex     sync.RWMutex
	datastore *models.DataStore
}

func NewCanvasManager(datastore *models.DataStore) *CanvasManager {
	cm := &CanvasManager{
		canvases:  make(map[uint]*canvasData),
		clients:   make(map[uint]int),
		font:      nil,
		datastore: datastore,
	}
	go cm.periodicSave()
	return cm
}

func (cm *CanvasManager) createCanvasData(boardID uint) *canvasData {
	backend := softwarebackend.New(CanvasWidth, CanvasHeight)
	cv := canvas.New(backend)

	cv.SetFillStyle("#FFFFFF")
	cv.FillRect(0, 0, float64(CanvasWidth), float64(CanvasHeight))

	if cm.font == nil {
		fontData, err := os.ReadFile("assets/open-sans.ttf")
		if err != nil {
			fmt.Printf("Failed to load font: %v\n", fontData)
		} else {
			cm.font, err = cv.LoadFont(fontData)
			if err != nil {
				fmt.Printf("Failed to load font into canvas: %v\n", err)
			}
		}
	}

	return &canvasData{
		Backend: backend,
		Canvas:  cv,
		Font:    cm.font,
	}
}

func (cm *CanvasManager) GetOrCreateCanvas(boardID uint) *canvasData {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if c, exists := cm.canvases[boardID]; exists {
		return c
	}

	board := models.Board{}
	if err := cm.datastore.GormDB.First(&board, boardID).Error; err == nil && len(board.Content) > 0 {
		img, err := webp.Decode(bytes.NewReader(board.Content))
		if err == nil {
			data := cm.createCanvasData(boardID)
			draw.Draw(data.Backend.Image, data.Backend.Image.Bounds(), img, image.Point{}, draw.Src)
			cm.canvases[boardID] = data
			return data
		}
	}

	data := cm.createCanvasData(boardID)
	cm.canvases[boardID] = data
	return data
}

func (cm *CanvasManager) ApplyDraw(boardID uint, x1, y1, x2, y2 float64, col string, size float64, tool string) {
	cm.mutex.RLock()
	data, exists := cm.canvases[boardID]
	cm.mutex.RUnlock()

	if !exists {
		data = cm.GetOrCreateCanvas(boardID)
	}

	cv := data.Canvas

	if tool == "eraser" {
		cv.SetStrokeStyle("#FFFFFF")
		cv.SetLineWidth(size * 4)
	} else {
		cv.SetStrokeStyle(col)
		cv.SetLineWidth(size)
	}
	cv.SetLineCap(canvas.Round)
	cv.SetLineJoin(canvas.Round)

	cv.BeginPath()
	cv.MoveTo(x1, y1)
	cv.LineTo(x2, y2)
	cv.Stroke()
}

func (cm *CanvasManager) ApplyText(boardID uint, x, y float64, textStr, col string, size float64) {
	cm.mutex.RLock()
	data, exists := cm.canvases[boardID]
	cm.mutex.RUnlock()

	if !exists {
		data = cm.GetOrCreateCanvas(boardID)
	}

	cv := data.Canvas
	fontSize := size * 2

	if data.Font != nil {
		cv.SetFont(data.Font, fontSize*1.3)
	} else {
		return
	}

	cv.SetFillStyle(col)

	lines := strings.Split(textStr, "\n")
	lineHeight := fontSize * 1.2
	for i, line := range lines {
		cv.FillText(line, x, y+float64(i)*lineHeight)
	}
}

func (cm *CanvasManager) ClearCanvas(boardID uint) {
	cm.mutex.RLock()
	data, exists := cm.canvases[boardID]
	cm.mutex.RUnlock()

	if !exists {
		return
	}

	data.Canvas.SetFillStyle("#FFFFFF")
	data.Canvas.FillRect(0, 0, float64(CanvasWidth), float64(CanvasHeight))
}

func (cm *CanvasManager) GetContent(boardID uint) []byte {
	cm.mutex.RLock()
	data, exists := cm.canvases[boardID]
	cm.mutex.RUnlock()

	if !exists || data == nil {
		return nil
	}

	var buf bytes.Buffer
	err := webp.Encode(&buf, data.Backend.Image, &webp.Options{Quality: 80})
	if err != nil {
		return nil
	}
	return buf.Bytes()
}

func (cm *CanvasManager) SaveToDB(boardID uint) {
	content := cm.GetContent(boardID)
	if content == nil {
		return
	}

	cm.datastore.GormDB.Model(&models.Board{}).Where("id = ?", boardID).Update("content", content)
}

func (cm *CanvasManager) RegisterClient(boardID uint) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.clients[boardID]++
	if _, exists := cm.canvases[boardID]; !exists {
		cm.canvases[boardID] = cm.loadFromDB(boardID)
	}
}

func (cm *CanvasManager) UnregisterClient(boardID uint) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.clients[boardID]--
	if cm.clients[boardID] <= 0 {
		delete(cm.clients, boardID)
		if data, exists := cm.canvases[boardID]; exists {
			var buf bytes.Buffer
			if err := webp.Encode(&buf, data.Backend.Image, &webp.Options{Quality: 80}); err == nil {
				cm.datastore.GormDB.Model(&models.Board{}).Where("id = ?", boardID).Update("content", buf.Bytes())
			}
			delete(cm.canvases, boardID)
		}
	}
}

func (cm *CanvasManager) loadFromDB(boardID uint) *canvasData {
	board := models.Board{}
	if err := cm.datastore.GormDB.First(&board, boardID).Error; err != nil {
		return cm.createCanvasData(boardID)
	}

	if len(board.Content) == 0 {
		return cm.createCanvasData(boardID)
	}

	img, err := webp.Decode(bytes.NewReader(board.Content))
	if err != nil {
		return cm.createCanvasData(boardID)
	}

	data := cm.createCanvasData(boardID)
	draw.Draw(data.Backend.Image, data.Backend.Image.Bounds(), img, image.Point{}, draw.Src)
	return data
}

func (cm *CanvasManager) periodicSave() {
	ticker := time.NewTicker(60 * time.Second)
	for range ticker.C {
		cm.mutex.RLock()
		boardIDs := make([]uint, 0, len(cm.clients))
		for boardID := range cm.clients {
			boardIDs = append(boardIDs, boardID)
		}
		cm.mutex.RUnlock()

		for _, boardID := range boardIDs {
			cm.SaveToDB(boardID)
		}
	}
}

func NewBoardService(app App) (*BoardService, error) {
	s := &BoardService{
		app:           app,
		hub:           NewHub(),
		canvasManager: NewCanvasManager(app.GetDatastore()),
	}
	return s, nil
}

func (s *BoardService) getTeamBoard(boardID uint) (*models.TeamBoard, error) {
	datastore := s.app.GetDatastore()
	var teamBoard models.TeamBoard
	err := datastore.GormDB.Where("board_id = ?", boardID).First(&teamBoard).Error
	if err != nil {
		return nil, err
	}
	return &teamBoard, nil
}

func (s *BoardService) IsTeamBoard(boardID uint) bool {
	_, err := s.getTeamBoard(boardID)
	return err == nil
}

func (s *BoardService) GetTeamBoardOwnerID(boardID uint) (uint, error) {
	teamBoard, err := s.getTeamBoard(boardID)
	if err != nil {
		return 0, err
	}
	return teamBoard.BoardOwnerID, nil
}

func (s *BoardService) CanGrantPermission(boardID uint, userID uint) (bool, error) {
	board, err := s.GetBoard(boardID)
	if err != nil {
		return false, err
	}

	if board.TeamID == 0 {
		return false, errors.New("not a team board")
	}

	teamBoard, err := s.getTeamBoard(boardID)
	if err != nil {
		return false, err
	}

	if teamBoard.BoardOwnerID == userID {
		return teamBoard.CanGrantPermission, nil
	}

	return false, errors.New("not the board owner")
}

func (s *BoardService) CanDelete(boardID uint, userID uint) (bool, error) {
	board, err := s.GetBoard(boardID)
	if err != nil {
		return false, err
	}

	if board.TeamID == 0 {
		return false, errors.New("not a team board")
	}

	teamBoard, err := s.getTeamBoard(boardID)
	if err != nil {
		return false, err
	}

	if teamBoard.BoardOwnerID == userID {
		return teamBoard.CanDelete, nil
	}

	return false, errors.New("not the board owner")
}

func (s *BoardService) CanEditMetadata(boardID uint, userID uint) (bool, error) {
	board, err := s.GetBoard(boardID)
	if err != nil {
		return false, err
	}

	if board.TeamID == 0 {
		return false, errors.New("not a team board")
	}

	teamBoard, err := s.getTeamBoard(boardID)
	if err != nil {
		return false, err
	}

	if teamBoard.BoardOwnerID == userID {
		return teamBoard.CanEditMetadata, nil
	}

	return false, errors.New("not the board owner")
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

func (s *BoardService) CreateBoard(title, description, tags string, ownerID uint) (*BoardInfo, error) {
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
		OwnerID:     ownerID,
	}
	if err := datastore.GormDB.Create(&board).Error; err != nil {
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

type BoardInfo struct {
	ID          uint
	Title       string
	Description string
	Tags        string
	OwnerID     uint
	TeamID      uint
	CreatedAt   time.Time
}

type BoardResponse struct {
	ID          uint
	Title       string
	Description string
	Tags        string
	OwnerID     uint
	TeamID      uint
	CreatedAt   time.Time
}

func (s *BoardService) UpdateBoard(id uint, userID uint, title, description, tags string) (*BoardInfo, error) {
	datastore := s.app.GetDatastore()
	board := models.Board{}
	if err := datastore.GormDB.First(&board, id).Error; err != nil {
		return nil, err
	}

	canEdit := false
	if board.TeamID == 0 {
		if board.OwnerID == userID {
			canEdit = true
		}
	} else {
		if board.OwnerID == userID {
			canEdit = true
		} else {
			canEditMeta, err := s.CanEditMetadata(id, userID)
			if err == nil && canEditMeta {
				canEdit = true
			}
		}
	}

	if !canEdit {
		return nil, errors.New("unauthorized to edit board")
	}

	if title != "" {
		if len(title) > 128 {
			return nil, errors.New("title must be 128 characters or less")
		}
		board.Title = title
	}
	if description != "" {
		if len(description) > 500 {
			return nil, errors.New("description must be 500 characters or less")
		}
		board.Description = description
	}
	if tags != "" {
		if len(tags) > 500 {
			return nil, errors.New("tags must be 500 characters or less")
		}
		board.Tags = tags
	}

	if err := datastore.GormDB.Save(&board).Error; err != nil {
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

func (s *BoardService) AddTags(id uint, userID uint, newTags []string) (*BoardInfo, error) {
	if len(newTags) == 0 {
		return nil, errors.New("at least one tag is required")
	}
	for _, tag := range newTags {
		if len(tag) > 50 {
			return nil, errors.New("each tag must be 50 characters or less")
		}
	}

	datastore := s.app.GetDatastore()
	board := models.Board{}
	if err := datastore.GormDB.First(&board, id).Error; err != nil {
		return nil, err
	}

	canEdit := false
	if board.TeamID == 0 {
		if board.OwnerID == userID {
			canEdit = true
		}
	} else {
		if board.OwnerID == userID {
			canEdit = true
		} else {
			canEditMeta, err := s.CanEditMetadata(id, userID)
			if err == nil && canEditMeta {
				canEdit = true
			}
		}
	}

	if !canEdit {
		return nil, errors.New("unauthorized to edit board tags")
	}

	currentTags := parseTags(board.Tags)
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
	board.Tags = joinTags(currentTags)

	if err := datastore.GormDB.Save(&board).Error; err != nil {
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

func (s *BoardService) DeleteBoard(id uint, userID uint) error {
	datastore := s.app.GetDatastore()
	board := models.Board{}
	if err := datastore.GormDB.First(&board, id).Error; err != nil {
		return errors.New("board not found")
	}

	canDelete := false
	if board.TeamID == 0 {
		if board.OwnerID == userID {
			canDelete = true
		}
	} else {
		if board.OwnerID == userID {
			canDelete = true
		} else {
			canDel, err := s.CanDelete(id, userID)
			if err == nil && canDel {
				canDelete = true
			}
		}
	}

	if !canDelete {
		return errors.New("unauthorized to delete board")
	}

	result := datastore.GormDB.Delete(&board)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("board not found")
	}
	return nil
}

func (s *BoardService) UserRequestDeleteBoard(id uint, userID uint) error {
	return s.DeleteBoard(id, userID)
}

func (s *BoardService) GetUserBoards(userID uint) ([]models.Board, error) {
	datastore := s.app.GetDatastore()
	var boards []models.Board
	if err := datastore.GormDB.Where("owner_id = ?", userID).Order("created_at desc").Find(&boards).Error; err != nil {
		return nil, err
	}
	return boards, nil
}

func (s *BoardService) GetBoard(id uint) (*BoardInfo, error) {
	datastore := s.app.GetDatastore()
	board := models.Board{}
	if err := datastore.GormDB.First(&board, id).Error; err != nil {
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

func (s *BoardService) GetBoardByIDAndOwner(id uint, ownerID uint) (*BoardInfo, error) {
	datastore := s.app.GetDatastore()
	board := models.Board{}
	if err := datastore.GormDB.Where("id = ? AND owner_id = ?", id, ownerID).First(&board).Error; err != nil {
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

func (s *BoardService) GetHub() *Hub {
	return s.hub
}

func (s *BoardService) GetCanvasManager() *CanvasManager {
	return s.canvasManager
}

type memberResponse struct {
	ID       uint
	UserID   uint
	Username string
	Role     string
}

func (s *BoardService) UserRequestAddMember(boardID uint, requestUserID uint, targetUserID uint, role string) error {
	board, err := s.GetBoard(boardID)
	if err != nil {
		return errors.New("board not found")
	}

	canGrant := false
	if board.TeamID == 0 {
		if board.OwnerID == requestUserID {
			canGrant = true
		}
	} else {
		if board.OwnerID == requestUserID {
			canGrant = true
		} else {
			canGrantPerm, err := s.CanGrantPermission(boardID, requestUserID)
			if err == nil && canGrantPerm {
				canGrant = true
			}
		}
	}

	if !canGrant {
		return errors.New("unauthorized to grant permissions")
	}

	if targetUserID == board.OwnerID {
		return errors.New("cannot add owner as member")
	}
	if role != models.RoleViewer && role != models.RoleEditor {
		role = models.RoleViewer
	}

	datastore := s.app.GetDatastore()
	var existing models.BoardMember
	err = datastore.GormDB.Where("board_id = ? AND user_id = ?", boardID, targetUserID).First(&existing).Error
	if err == nil {
		existing.Role = role
		return datastore.GormDB.Save(&existing).Error
	}

	member := models.BoardMember{
		BoardID: boardID,
		UserID:  targetUserID,
		Role:    role,
	}
	return datastore.GormDB.Create(&member).Error
}

func (s *BoardService) UserRequestRemoveMember(boardID uint, requestUserID uint, targetUserID uint) error {
	board, err := s.GetBoard(boardID)
	if err != nil {
		return errors.New("board not found")
	}

	canGrant := false
	if board.TeamID == 0 {
		if board.OwnerID == requestUserID {
			canGrant = true
		}
	} else {
		if board.OwnerID == requestUserID {
			canGrant = true
		} else {
			canGrantPerm, err := s.CanGrantPermission(boardID, requestUserID)
			if err == nil && canGrantPerm {
				canGrant = true
			}
		}
	}

	if !canGrant {
		return errors.New("unauthorized to remove members")
	}

	if targetUserID == board.OwnerID {
		return errors.New("cannot remove owner")
	}

	datastore := s.app.GetDatastore()
	result := datastore.GormDB.Where("board_id = ? AND user_id = ?", boardID, targetUserID).Delete(&models.BoardMember{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("member not found")
	}
	return nil
}

func (s *BoardService) UserRequestUpdateMemberRole(boardID uint, requestUserID uint, targetUserID uint, newRole string) error {
	board, err := s.GetBoard(boardID)
	if err != nil {
		return errors.New("board not found")
	}

	canGrant := false
	if board.TeamID == 0 {
		if board.OwnerID == requestUserID {
			canGrant = true
		}
	} else {
		if board.OwnerID == requestUserID {
			canGrant = true
		} else {
			canGrantPerm, err := s.CanGrantPermission(boardID, requestUserID)
			if err == nil && canGrantPerm {
				canGrant = true
			}
		}
	}

	if !canGrant {
		return errors.New("unauthorized to update member role")
	}

	if targetUserID == board.OwnerID {
		return errors.New("cannot update owner role")
	}
	if newRole != models.RoleViewer && newRole != models.RoleEditor {
		newRole = models.RoleViewer
	}

	datastore := s.app.GetDatastore()
	var member models.BoardMember
	err = datastore.GormDB.Where("board_id = ? AND user_id = ?", boardID, targetUserID).First(&member).Error
	if err != nil {
		return errors.New("member not found")
	}

	member.Role = newRole
	return datastore.GormDB.Save(&member).Error
}

func (s *BoardService) GetBoardMembers(boardID uint) ([]memberResponse, error) {
	datastore := s.app.GetDatastore()

	type memberWithUser struct {
		models.BoardMember
		Username string
	}

	type ownerWithUser struct {
		OwnerID  uint
		Username string
	}

	var ownerUser ownerWithUser
	if err := datastore.GormDB.
		Table("users").
		Select("id as owner_id, username").
		Where("id = (SELECT owner_id FROM boards WHERE id = ?)", boardID).
		Scan(&ownerUser).Error; err != nil {
		return nil, errors.New("board not found")
	}

	var boardMembersWithUsers []memberWithUser
	if err := datastore.GormDB.
		Joins("JOIN users ON users.id = board_members.user_id").
		Where("board_members.board_id = ?", boardID).
		Find(&boardMembersWithUsers).Error; err != nil {
		return nil, err
	}

	var members []memberResponse
	members = append(members, memberResponse{
		ID:       ownerUser.OwnerID,
		UserID:   ownerUser.OwnerID,
		Username: ownerUser.Username,
		Role:     models.RoleOwner,
	})

	for _, m := range boardMembersWithUsers {
		members = append(members, memberResponse{
			ID:       m.ID,
			UserID:   m.UserID,
			Username: m.Username,
			Role:     m.Role,
		})
	}

	return members, nil
}

func (s *BoardService) GetBoardMembersPaginated(boardID uint, offset, limit int) ([]memberResponse, int, error) {
	datastore := s.app.GetDatastore()

	type ownerWithUser struct {
		OwnerID  uint
		Username string
	}

	var ownerUser ownerWithUser
	if err := datastore.GormDB.
		Table("users").
		Select("id as owner_id, username").
		Where("id = (SELECT owner_id FROM boards WHERE id = ?)", boardID).
		Scan(&ownerUser).Error; err != nil {
		return nil, 0, errors.New("board not found")
	}

	var total int64
	if err := datastore.GormDB.Model(&models.BoardMember{}).Where("board_id = ?", boardID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	total++

	type memberWithUser struct {
		models.BoardMember
		Username string
	}

	var boardMembersWithUsers []memberWithUser
	if err := datastore.GormDB.
		Joins("JOIN users ON users.id = board_members.user_id").
		Where("board_members.board_id = ?", boardID).
		Order("board_members.created_at asc").
		Offset(offset).Limit(limit).
		Find(&boardMembersWithUsers).Error; err != nil {
		return nil, 0, err
	}

	var members []memberResponse

	if offset == 0 {
		members = append(members, memberResponse{
			ID:       ownerUser.OwnerID,
			UserID:   ownerUser.OwnerID,
			Username: ownerUser.Username,
			Role:     models.RoleOwner,
		})
		limit--
	}

	for _, m := range boardMembersWithUsers {
		if limit <= 0 {
			break
		}
		members = append(members, memberResponse{
			ID:       m.ID,
			UserID:   m.UserID,
			Username: m.Username,
			Role:     m.Role,
		})
	}

	return members, int(total), nil
}

func (s *BoardService) GetUserPermission(boardID uint, userID uint) (string, error) {
	board, err := s.GetBoard(boardID)
	if err != nil {
		return "", errors.New("board not found")
	}

	if board.OwnerID == userID {
		return models.RoleOwner, nil
	}

	if board.TeamID != 0 {
		teamBoard, err := s.getTeamBoard(boardID)
		if err == nil && teamBoard.BoardOwnerID == userID {
			return models.RoleOwner, nil
		}
	}

	datastore := s.app.GetDatastore()
	var member models.BoardMember
	err = datastore.GormDB.Where("board_id = ? AND user_id = ?", boardID, userID).First(&member).Error
	if err != nil {
		return "", errors.New("no access to this board")
	}

	return member.Role, nil
}

func (s *BoardService) GetBoardTitleAndPermission(boardID uint, userID uint) (string, string, error) {
	datastore := s.app.GetDatastore()

	type result struct {
		BoardID       uint
		Title         string
		Description   string
		OwnerID       uint
		TeamID        uint
		BoardOwnerID  uint
		MemberRole    string
		TeamBoardRole string
	}

	var results []result
	err := datastore.GormDB.
		Table("boards").
		Select("boards.id as board_id, boards.title, boards.description, boards.owner_id, boards.team_id, board_members.role as member_role, team_boards.board_owner_id").
		Joins("LEFT JOIN board_members ON boards.id = board_members.board_id AND board_members.user_id = ?", userID).
		Joins("LEFT JOIN team_boards ON boards.id = team_boards.board_id").
		Where("boards.id = ?", boardID).
		Scan(&results).Error

	if err != nil || len(results) == 0 {
		return "", "", errors.New("board not found")
	}

	r := results[0]

	role := ""
	if r.OwnerID == userID {
		role = models.RoleOwner
	} else if r.TeamID != 0 && r.BoardOwnerID == userID {
		role = models.RoleOwner
	} else if r.MemberRole != "" {
		role = r.MemberRole
	}

	if role == "" {
		return "", "", errors.New("no access to this board")
	}

	return r.Title, role, nil
}

func (s *BoardService) HasViewAccess(boardID uint, userID uint) bool {
	board, err := s.GetBoard(boardID)
	if err != nil {
		return false
	}

	if board.OwnerID == userID {
		return true
	}

	if board.TeamID != 0 {
		teamBoard, err := s.getTeamBoard(boardID)
		if err == nil && teamBoard.BoardOwnerID == userID {
			return true
		}
	}

	datastore := s.app.GetDatastore()
	var member models.BoardMember
	err = datastore.GormDB.Where("board_id = ? AND user_id = ?", boardID, userID).First(&member).Error
	return err == nil
}

func (s *BoardService) CanEdit(boardID uint, userID uint) bool {
	permission, err := s.GetUserPermission(boardID, userID)
	if err != nil {
		return false
	}
	return permission == models.RoleOwner || permission == models.RoleEditor
}

type userBoardInfo struct {
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	Tags             string    `json:"tags"`
	ID               uint      `json:"id"`
	OwnerID          uint      `json:"ownerId"`
	TeamBoardOwnerID uint      `json:"teamBoardOwnerId"`
	Role             string    `json:"role"`
	TeamID           uint      `json:"teamId"`
	CreatedAt        time.Time `json:"createdAt"`
}

func (s *BoardService) GetUserBoardsWithAccess(userID uint, offset, limit int) ([]userBoardInfo, error) {
	datastore := s.app.GetDatastore()

	var boardInfos []userBoardInfo
	if err := datastore.GormDB.
		Model(&models.Board{}).
		Select("boards.title as title, boards.description as description, boards.tags as tags, boards.id as id, boards.owner_id as owner_id, team_boards.board_owner_id as team_board_owner_id, board_members.role as role, team_boards.team_id as team_id, boards.created_at as created_at").
		Joins("LEFT JOIN board_members ON boards.id = board_members.id").
		Joins("LEFT JOIN team_boards on boards.id = team_boards.board_id").
		Where("boards.owner_id = ?", userID).
		Or("board_members.user_id = ?", userID).
		Or("team_boards.board_owner_id = ?", userID).
		Find(&boardInfos).Error; err != nil {
		return nil, err
	}
	return boardInfos, nil
}

type Hub struct {
	boards map[uint]map[*websocket.Conn]*Client
	mutex  sync.RWMutex
}

type Client struct {
	Conn       *websocket.Conn
	BoardID    uint
	Username   string
	Permission string
	Send       chan []byte
}

func NewHub() *Hub {
	return &Hub{
		boards: make(map[uint]map[*websocket.Conn]*Client),
	}
}

func (h *Hub) Register(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if h.boards[client.BoardID] == nil {
		h.boards[client.BoardID] = make(map[*websocket.Conn]*Client)
	}
	h.boards[client.BoardID][client.Conn] = client
}

func (h *Hub) Unregister(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if clients, ok := h.boards[client.BoardID]; ok {
		if _, ok := clients[client.Conn]; ok {
			delete(clients, client.Conn)
			close(client.Send)
			if len(clients) == 0 {
				delete(h.boards, client.BoardID)
			}
		}
	}
}

func (h *Hub) Broadcast(boardID uint, message []byte, exclude *websocket.Conn) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	if clients, ok := h.boards[boardID]; ok {
		for conn, client := range clients {
			if conn != exclude {
				select {
				case client.Send <- message:
				default:
				}
			}
		}
	}
}
