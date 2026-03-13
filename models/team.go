package models

import "gorm.io/gorm"

type Team struct {
	gorm.Model
	Title       string `gorm:"size:128;not null"`
	Description string `gorm:"size:500"`
	Tags        string `gorm:"size:500"`
	OwnerID     uint   `gorm:"not null"`
}

const (
	TeamRoleOwner  = "owner"
	TeamRoleMember = "member"
)

type TeamMember struct {
	gorm.Model
	TeamID uint   `gorm:"not null;index"`
	UserID uint   `gorm:"not null;index"`
	Role   string `gorm:"size:20;not null;default:'member'"`
}

type TeamBoard struct {
	gorm.Model
	TeamID  uint `gorm:"not null;index"`
	BoardID uint `gorm:"not null;index"`
}
