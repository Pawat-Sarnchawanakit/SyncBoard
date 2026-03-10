package models

import "gorm.io/gorm"

type Board struct {
	gorm.Model
	Title       string `gorm:"size:128;not null"`
	Description string `gorm:"size:500"`
	Tags        string `gorm:"size:500"`
	OwnerID     uint   `gorm:"not null"`
}
