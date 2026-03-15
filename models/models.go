package models

import (
	"fmt"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DataStore struct {
	GormDB *gorm.DB
}

func NewDataStore() (*DataStore, error) {
	var db *gorm.DB
	var err error

	dbType := os.Getenv("DB_TYPE")
	if dbType == "sqlite" {
		db, err = gorm.Open(sqlite.Open("db.db"), &gorm.Config{})
	} else {
		host := getEnv("DB_HOST", "localhost")
		port := getEnv("DB_PORT", "5432")
		user := getEnv("DB_USER", "postgres")
		password := getEnv("DB_PASSWORD", "postgres")
		dbname := getEnv("DB_NAME", "syncboard")

		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
			host, user, password, dbname, port)

		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	}

	if err != nil {
		return nil, err
	}
	db.Logger = db.Logger.LogMode(logger.Info)
	err = db.AutoMigrate(&User{}, &Board{}, &BoardMember{}, &Team{}, &TeamBoard{}, &TeamMember{})
	if err != nil {
		return nil, err
	}
	return &DataStore{GormDB: db}, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
