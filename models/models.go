package models

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DataStore struct {
	GormDB *gorm.DB
}

func NewDataStore() (*DataStore, error) {
	db, err := gorm.Open(sqlite.Open("db.db"), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	db.Logger = db.Logger.LogMode(logger.Info)
	err = db.AutoMigrate(&User{}, &Board{}, &BoardMember{}, &Team{}, &TeamMember{})
	if err != nil {
		return nil, err
	}
	return &DataStore{GormDB: db}, nil
}
