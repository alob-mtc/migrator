package cmd

import (
	"fmt"
	"github.com/alobmtc/migrator/example/table"
	migrator "github.com/alobmtc/migrator/lib"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Migrations() *cobra.Command {
	return &cobra.Command{
		Use:   "migrations",
		Short: "Run test-app database migrations",
		Run: func(cmd *cobra.Command, args []string) {
			log.Info("Starting test-app migrations")
			runMigrations(args)
		},
	}
}

func runMigrations(args []string) {
	fmt.Println("Connecting to postgres")
	dns := "postgres://test:test@localhost:5431/test_db"
	db, err := gorm.Open(postgres.Open(dns), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Established database connection")

	newMigrator := migrator.New(db)
	newMigrator.RegisterModel(&table.Test1{}, &table.Test2{})

	err = newMigrator.Run(db, args, "example/migrations/sql/")
	if err != nil {
		fmt.Println("err:", err)
		return
	}
}
