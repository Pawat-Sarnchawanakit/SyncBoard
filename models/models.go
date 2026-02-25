package models

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type DataStore struct {
	GormDB *gorm.DB
}

func NewDataStore() (*DataStore, error) {
	db, err := gorm.Open(sqlite.Open("db.db"), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	db.AutoMigrate(&User{})
	return &DataStore{ GormDB: db }, nil
}