package models

import (
	"time"

	"gorm.io/gorm"
)

type Board struct {
	gorm.Model
	Title       string `gorm:"size:128;not null"`
	Description string `gorm:"size:500"`
	Tags        string `gorm:"size:500"`
	OwnerID     uint   `gorm:"not null"`
	TeamID      uint   `gorm:"index"`
	Content     []byte
}

const (
	RoleOwner  = "owner"
	RoleEditor = "editor"
	RoleViewer = "viewer"
)

type BoardMember struct {
	BoardID uint   `gorm:"primaryKey"`
	UserID  uint   `gorm:"primaryKey"`
	Role    string `gorm:"size:20;not null;default:'viewer'"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
