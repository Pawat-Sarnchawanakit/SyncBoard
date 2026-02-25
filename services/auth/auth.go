package auth

import (
	"sync-board/models"

	"github.com/alexedwards/argon2id"
)

type App interface {
	GetDatastore() *models.DataStore
}

type AuthenticationService struct{
	app App
}

func NewAuthenticationService(app App) *AuthenticationService {
	return &AuthenticationService{
		app: app,
	}
}

var argon2id_params = &argon2id.Params{
	Memory: 131072,
	Iterations: 6,
	Parallelism: 1,
	SaltLength: 24,
	KeyLength: 48,
}

func (cur *AuthenticationService) SignUp(username string, password string) error {
	datastore := cur.app.GetDatastore()
	hash, err := argon2id.CreateHash(password, argon2id_params)
	if err != nil {
		return err
	}
	if err := datastore.GormDB.Create(&models.User{
		Username: username,
		Password: hash,
	}).Error; err != nil {
		return err
	}
	return nil
}