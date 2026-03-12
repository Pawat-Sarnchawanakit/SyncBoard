package models

import "gorm.io/gorm"

type Board struct {
	gorm.Model
	Title       string `gorm:"size:128;not null"`
	Description string `gorm:"size:500"`
	Tags        string `gorm:"size:500"`
	OwnerID     uint   `gorm:"not null"`
	Content     []byte `gorm:"type:blob"`
}

const (
	RoleOwner  = "owner"
	RoleEditor = "editor"
	RoleViewer = "viewer"
)

type BoardMember struct {
	gorm.Model
	BoardID uint   `gorm:"not null;index"`
	UserID  uint   `gorm:"not null;index"`
	Role    string `gorm:"size:20;not null;default:'viewer'"`
}
