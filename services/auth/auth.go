package auth

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync-board/models"
	"time"

	"github.com/alexedwards/argon2id"
	"golang.org/x/crypto/blake2b"
)

type App interface {
	GetDatastore() *models.DataStore
}

type AuthenticationService struct {
	app          App
	token_secret []byte
}

func NewAuthenticationService(app App) (*AuthenticationService, error) {
	service := &AuthenticationService{
		app: app,
	}
	token_secret, err := hex.DecodeString(os.Getenv("AUTH_TOKEN_SECRET"))
	if err != nil {
		return nil, err
	}
	if len(token_secret) != 64 {
		return nil, errors.New("AUTH_TOKEN_SECRET must be 64 bytes long")
	}
	service.token_secret = token_secret
	return service, nil
}

var argon2id_params = &argon2id.Params{
	Memory:      131072,
	Iterations:  6,
	Parallelism: 1,
	SaltLength:  16,
	KeyLength:   32,
}

func (cur *AuthenticationService) SignUp(username string, password string) (string, error) {
	datastore := cur.app.GetDatastore()
	hash, err := argon2id.CreateHash(password, argon2id_params)
	if err != nil {
		return "", err
	}
	user := models.User{
		Username: username,
		Password: hash,
	}
	if err := datastore.GormDB.Create(&user).Error; err != nil {
		return "", err
	}
	token, err := cur.GenerateToken(user.ID)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (cur *AuthenticationService) Login(username string, password string) (string, error) {
	datastore := cur.app.GetDatastore()
	user := models.User{}
	// Prevents user enumeration, use generic errors
	if err := datastore.GormDB.First(&user, &models.User{Username: username}).Error; err != nil {
		return "", errors.New("Invalid Credentials")
	}
	match, err := argon2id.ComparePasswordAndHash(password, user.Password)
	if err != nil {
		return "", errors.New("Invalid Credentials")
	}
	if !match {
		return "", errors.New("Invalid Credentials")
	}
	token, err := cur.GenerateToken(user.ID)
	if err != nil {
		return "", errors.New("Invalid Credentials")
	}
	return token, nil
}

func (cur *AuthenticationService) GenerateToken(user_id uint) (string, error) {
	signed_part := fmt.Sprintf("%d_%d", user_id, time.Now().Unix())
	hasher, err := blake2b.New256(cur.token_secret)
	if err != nil {
		return "", err
	}
	if _, err := hasher.Write([]byte(signed_part)); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s_%s", signed_part, hex.EncodeToString(hasher.Sum([]byte("")))), nil
}

func (cur *AuthenticationService) VerifyToken(token string) (uint, error) {
	token_parts := strings.Split(token, "_")
	if len(token_parts) != 3 {
		return 0, errors.New("Bad token")
	}
	provided_signature, err := hex.DecodeString(token_parts[2])
	if err != nil {
		return 0, err
	}
	signed_part := fmt.Sprintf("%s_%s", token_parts[0], token_parts[1])
	hasher, err := blake2b.New256(cur.token_secret)
	if err != nil {
		return 0, err
	}
	if _, err := hasher.Write([]byte(signed_part)); err != nil {
		return 0, err
	}
	calculated_signature := hasher.Sum([]byte(""))
	if !bytes.Equal(calculated_signature, provided_signature) {
		return 0, errors.New("Token integrity check failed")
	}
	user_id, err := strconv.ParseUint(token_parts[0], 10, 32)
	if err != nil {
		return 0, err
	}
	ts, err := strconv.ParseInt(token_parts[1], 10, 64)
	if err != nil {
		return 0, err
	}
	if time.Now().Add(-24 * 30 * time.Hour).After(time.Unix(ts, 0)) {
		return 0, errors.New("Token expired")
	}
	return uint(user_id), nil
}

func (cur *AuthenticationService) GetUserByID(id uint) (*models.User, error) {
	datastore := cur.app.GetDatastore()
	user := models.User{}
	if err := datastore.GormDB.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (cur *AuthenticationService) GetUserByUsername(username string) (*models.User, error) {
	datastore := cur.app.GetDatastore()
	user := models.User{}
	if err := datastore.GormDB.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (cur *AuthenticationService) SearchUsers(query string, limit int) ([]models.User, error) {
	datastore := cur.app.GetDatastore()
	var users []models.User
	if limit <= 0 {
		limit = 10
	}
	if err := datastore.GormDB.Where("username LIKE ?", "%"+query+"%").Limit(limit).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}
