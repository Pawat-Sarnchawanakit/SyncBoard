package board

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync-board/models"

	"github.com/gorilla/websocket"
)

type App interface {
	GetDatastore() *models.DataStore
}

type BoardService struct {
	app App
	hub *Hub
}

func NewBoardService(app App) (*BoardService, error) {
	s := &BoardService{
		app: app,
		hub: NewHub(),
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

type Hub struct {
	boards map[uint]map[*websocket.Conn]*Client
	mutex  sync.RWMutex
}

type Client struct {
	Conn     *websocket.Conn
	BoardID  uint
	Username string
	Send     chan []byte
}

type Message struct {
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
