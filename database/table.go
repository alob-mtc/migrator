package database

import (
	"gorm.io/gorm"
	"time"
)

type Test1 struct {
	ID        string `gorm:"primaryKey;"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Name      string         `json:"name" gorm:"index; not null"`
	Active    *bool          `json:"active" gorm:"index; not null; default:false"`
}

type Test2 struct {
	ID        string `gorm:"primaryKey;"`
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Name      string         `json:"name" gorm:"index; not null"`
	Active    *bool          `json:"active" gorm:"default:false"`
	PartnerID string         `json:"-" gorm:"index; not null; unique"`
}
