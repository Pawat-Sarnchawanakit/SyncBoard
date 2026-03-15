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
	TeamID       uint  `gorm:"not null;index"`
	BoardID      uint  `gorm:"not null;index"`
	BoardOwnerID uint  `gorm:"not null"`
	Permissions  uint8 `gorm:"not null;default:0"`
}

const (
	PermCanGrant    uint8 = 1 << 0
	PermCanDelete   uint8 = 1 << 1
	PermCanEditMeta uint8 = 1 << 2
	PermCanDraw     uint8 = 1 << 3
)

func (t *TeamBoard) HasPermission(p uint8) bool {
	return t.Permissions&p != 0
}

func (t *TeamBoard) SetPermission(p uint8, val bool) {
	if val {
		t.Permissions |= p
	} else {
		t.Permissions &^= p
	}
}

func (t *TeamBoard) GetCanGrantPermission() bool {
	return t.Permissions&PermCanGrant != 0
}

func (t *TeamBoard) SetCanGrantPermission(val bool) {
	if val {
		t.Permissions |= PermCanGrant
	} else {
		t.Permissions &^= PermCanGrant
	}
}

func (t *TeamBoard) GetCanDelete() bool {
	return t.Permissions&PermCanDelete != 0
}

func (t *TeamBoard) SetCanDelete(val bool) {
	if val {
		t.Permissions |= PermCanDelete
	} else {
		t.Permissions &^= PermCanDelete
	}
}

func (t *TeamBoard) GetCanEditMetadata() bool {
	return t.Permissions&PermCanEditMeta != 0
}

func (t *TeamBoard) SetCanEditMetadata(val bool) {
	if val {
		t.Permissions |= PermCanEditMeta
	} else {
		t.Permissions &^= PermCanEditMeta
	}
}

func (t *TeamBoard) GetCanDraw() bool {
	return t.Permissions&PermCanDraw != 0
}

func (t *TeamBoard) SetCanDraw(val bool) {
	if val {
		t.Permissions |= PermCanDraw
	} else {
		t.Permissions &^= PermCanDraw
	}
}
