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
		cv.SetFont(data.Font, fontSize * 1.3)
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

func (s *BoardService) CreateBoard(title, description, tags string, ownerID uint) (*models.Board, error) {
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
	return &board, nil
}

type UpdateBoardInput struct {
	Title       *string
	Description *string
	Tags        *string
}

func (s *BoardService) UpdateBoard(id uint, ownerID uint, input UpdateBoardInput) (*models.Board, error) {
	datastore := s.app.GetDatastore()
	board := models.Board{}
	if err := datastore.GormDB.Where("id = ? AND owner_id = ?", id, ownerID).First(&board).Error; err != nil {
		return nil, err
	}

	if input.Title != nil {
		if *input.Title == "" {
			return nil, errors.New("title is required")
		}
		if len(*input.Title) > 128 {
			return nil, errors.New("title must be 128 characters or less")
		}
		board.Title = *input.Title
	}
	if input.Description != nil {
		if len(*input.Description) > 500 {
			return nil, errors.New("description must be 500 characters or less")
		}
		board.Description = *input.Description
	}
	if input.Tags != nil {
		if len(*input.Tags) > 500 {
			return nil, errors.New("tags must be 500 characters or less")
		}
		board.Tags = *input.Tags
	}

	if err := datastore.GormDB.Save(&board).Error; err != nil {
		return nil, err
	}
	return &board, nil
}

func (s *BoardService) AddTags(id uint, ownerID uint, newTags []string) (*models.Board, error) {
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
	if err := datastore.GormDB.Where("id = ? AND owner_id = ?", id, ownerID).First(&board).Error; err != nil {
		return nil, err
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
	return &board, nil
}

func (s *BoardService) DeleteBoard(id uint, ownerID uint) error {
	datastore := s.app.GetDatastore()
	result := datastore.GormDB.Where("id = ? AND owner_id = ?", id, ownerID).Delete(&models.Board{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("board not found")
	}
	return nil
}

func (s *BoardService) GetUserBoards(userID uint) ([]models.Board, error) {
	datastore := s.app.GetDatastore()
	var boards []models.Board
	if err := datastore.GormDB.Where("owner_id = ?", userID).Order("created_at desc").Find(&boards).Error; err != nil {
		return nil, err
	}
	return boards, nil
}

func (s *BoardService) GetBoard(id uint) (*models.Board, error) {
	datastore := s.app.GetDatastore()
	board := models.Board{}
	if err := datastore.GormDB.First(&board, id).Error; err != nil {
		return nil, err
	}
	return &board, nil
}

func (s *BoardService) GetBoardByIDAndOwner(id uint, ownerID uint) (*models.Board, error) {
	datastore := s.app.GetDatastore()
	board := models.Board{}
	if err := datastore.GormDB.Where("id = ? AND owner_id = ?", id, ownerID).First(&board).Error; err != nil {
		return nil, err
	}
	return &board, nil
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
	if board.OwnerID != requestUserID {
		return errors.New("only owner can add members")
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
	if board.OwnerID != requestUserID {
		return errors.New("only owner can remove members")
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
	if board.OwnerID != requestUserID {
		return errors.New("only owner can update members")
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
	board, err := s.GetBoard(boardID)
	if err != nil {
		return nil, errors.New("board not found")
	}

	var members []memberResponse
	var ownerUser models.User
	_ = datastore.GormDB.First(&ownerUser, board.OwnerID)
	members = append(members, memberResponse{
		ID:       board.OwnerID,
		UserID:   board.OwnerID,
		Username: ownerUser.Username,
		Role:     models.RoleOwner,
	})

	var boardMembers []models.BoardMember
	if err := datastore.GormDB.Where("board_id = ?", boardID).Find(&boardMembers).Error; err != nil {
		return nil, err
	}

	for _, m := range boardMembers {
		var user models.User
		if err := datastore.GormDB.First(&user, m.UserID).Error; err != nil {
			continue
		}
		members = append(members, memberResponse{
			ID:       m.ID,
			UserID:   m.UserID,
			Username: user.Username,
			Role:     m.Role,
		})
	}

	return members, nil
}

func (s *BoardService) GetBoardMembersPaginated(boardID uint, offset, limit int) ([]memberResponse, int, error) {
	datastore := s.app.GetDatastore()
	board, err := s.GetBoard(boardID)
	if err != nil {
		return nil, 0, errors.New("board not found")
	}

	var total int64 = 1
	var ownerUser models.User
	_ = datastore.GormDB.First(&ownerUser, board.OwnerID)

	var boardMembers []models.BoardMember
	query := datastore.GormDB.Model(&models.BoardMember{}).Where("board_id = ?", boardID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	total++

	if err := query.Offset(offset).Limit(limit).Find(&boardMembers).Error; err != nil {
		return nil, 0, err
	}

	var members []memberResponse

	if offset == 0 {
		members = append(members, memberResponse{
			ID:       board.OwnerID,
			UserID:   board.OwnerID,
			Username: ownerUser.Username,
			Role:     models.RoleOwner,
		})
		offset--
		limit--
	}

	for _, m := range boardMembers {
		if limit <= 0 {
			break
		}
		var user models.User
		if err := datastore.GormDB.First(&user, m.UserID).Error; err != nil {
			continue
		}
		members = append(members, memberResponse{
			ID:       m.ID,
			UserID:   m.UserID,
			Username: user.Username,
			Role:     m.Role,
		})
		limit--
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

	datastore := s.app.GetDatastore()
	var member models.BoardMember
	err = datastore.GormDB.Where("board_id = ? AND user_id = ?", boardID, userID).First(&member).Error
	if err != nil {
		return "", errors.New("no access to this board")
	}

	return member.Role, nil
}

func (s *BoardService) HasViewAccess(boardID uint, userID uint) bool {
	board, err := s.GetBoard(boardID)
	if err != nil {
		return false
	}

	if board.OwnerID == userID {
		return true
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

func (s *BoardService) GetUserBoardsWithAccess(userID uint, offset, limit int) ([]models.Board, int, error) {
	datastore := s.app.GetDatastore()

	var ownedCount int64
	if err := datastore.GormDB.Model(&models.Board{}).Where("owner_id = ?", userID).Count(&ownedCount).Error; err != nil {
		return nil, 0, err
	}

	var sharedBoardIDs []uint
	var members []models.BoardMember
	if err := datastore.GormDB.Where("user_id = ?", userID).Find(&members).Error; err != nil {
		return nil, 0, err
	}
	for _, m := range members {
		sharedBoardIDs = append(sharedBoardIDs, m.BoardID)
	}

	var sharedCount int64
	if len(sharedBoardIDs) > 0 {
		datastore.GormDB.Model(&models.Board{}).Where("id IN ?", sharedBoardIDs).Count(&sharedCount)
	}

	total := ownedCount + sharedCount

	var boards []models.Board

	ownedQuery := datastore.GormDB.Where("owner_id = ?", userID).Order("created_at desc").Offset(offset).Limit(limit)
	if err := ownedQuery.Find(&boards).Error; err != nil {
		return nil, 0, err
	}

	remaining := limit - len(boards)
	if remaining > 0 && len(sharedBoardIDs) > 0 {
		sharedOffset := offset - int(ownedCount)
		if sharedOffset < 0 {
			sharedOffset = 0
		}
		var sharedBoards []models.Board
		if err := datastore.GormDB.Where("id IN ?", sharedBoardIDs).Order("created_at desc").Offset(sharedOffset).Limit(remaining).Find(&sharedBoards).Error; err != nil {
			return nil, 0, err
		}
		boards = append(boards, sharedBoards...)
	} else if offset < int(ownedCount) && len(sharedBoardIDs) > 0 && offset+limit > int(ownedCount) {
		overlap := offset + limit - int(ownedCount)
		if overlap > 0 {
			var sharedBoards []models.Board
			if err := datastore.GormDB.Where("id IN ?", sharedBoardIDs).Order("created_at desc").Limit(overlap).Find(&sharedBoards).Error; err != nil {
				return nil, 0, err
			}
			boards = append(boards, sharedBoards...)
		}
	}

	return boards, int(total), nil
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
