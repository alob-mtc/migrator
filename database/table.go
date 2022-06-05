package database

import (
	"gorm.io/gorm"
	"time"
)

type Test1 struct {
	ID        string `gorm:"primaryKey;"`
	CreatedAt time.Time
	UpdatedAt time.Time      `gorm:"index"`
	After     time.Time      `gorm:"index"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Name      string         `gorm:"index; not null"`
	//Active    *bool          `gorm:"index; not null; default:false"`
}

type Test2 struct {
	ID        string `gorm:"primaryKey;"`
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Name      string         `gorm:"index; not null"`
	Active    *bool          `gorm:"default:false"`
	PartnerID string         `gorm:"index; not null; unique"`
}

type User struct {
	ID        string `gorm:"primaryKey;"`
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Name      string         `gorm:"index; not null"`
	Active    *bool          `gorm:"default:false"`
	PartnerID string         `gorm:"index; not null; unique"`
}
