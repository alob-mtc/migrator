package database

import (
	"fmt"
	"github.com/alobmtc/migrator/lib"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func ConnectDB(dsn string) *gorm.DB {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})

	if err != nil {
		fmt.Println(
			err.Error(),
		)
		return nil
	}

	fmt.Println("Established database connection")

	return db
}

func MigrateAll(db *gorm.DB) error {

	fmt.Println("Migration started")

	Migrator := lib.New(db)
	a, b, err := Migrator.AutoMigrate(
		&Test1{},
		&Test2{},
		//&User{},
	)
	if err != nil {
		return err
	}

	fmt.Println("Migration ended:\n", a, b)
	return nil
}
