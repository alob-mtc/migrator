package migrator

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"gorm.io/gorm"
)

func (mg *Migrator) Run(db *gorm.DB, command string, migrationname ...string) error {
	var err error
	var driver database.Driver

	if command == "" {
		log.Fatal("Specify a Command to run")
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("error getting sql.DB representation:", err)
	}

	dbName := db.Config.Dialector.Name()
	switch dbName {
	case "postgres":
		driver, err = postgres.WithInstance(sqlDB, &postgres.Config{})
		if err != nil {
			log.Fatal("error instantiating postgres instance:", err)
		}
	case "mysql":
		driver, err = mysql.WithInstance(sqlDB, &mysql.Config{})
		if err != nil {
			log.Fatal("error instantiating postgres instance:", err)
		}
	case "sqlite":
		driver, err = sqlite.WithInstance(sqlDB, &sqlite.Config{})
		if err != nil {
			log.Fatal("error instantiating postgres instance:", err)
		}
	default:
		log.Fatal("Datebase not supported")
	}

	m, err := migrate.NewWithDatabaseInstance("file://"+mg.migrationPath, dbName, driver)
	if err != nil {
		log.Fatal("error instantiating migration instance:", err)
	}

	startTime := time.Now()

	switch command {
	case "up":
		err = m.Up()
	case "down":
		err = m.Steps(-1)
	case "clear":
		err = m.Down()
	case "create":
		if len(migrationname) == 0 || migrationname[0] == "" {
			log.Fatal("Specify a name for the migration")
		}

		//generate migration
		sqlUp, sqlDown, err := mg.AutoMigrate()
		if err != nil {
			log.Fatal(err)
		}
		if sqlUp == "" && sqlDown == "" {
			return nil
		}

		mg.createCmd(startTime, migrationname[0], sqlUp, sqlDown)
	}

	if err != nil {
		if err != migrate.ErrNoChange {
			log.Fatal(err)
		} else {
			fmt.Println(err)
		}
	}

	fmt.Println("Finished after: ", time.Since(startTime).String())
	return nil
}

func (mg *Migrator) createCmd(timestamp time.Time, name string, sqlUp string, sqlDown string) {
	_ = os.MkdirAll(mg.migrationPath, os.ModePerm)
	upName, downName := mg.NamingStrategy(mg.migrationPath, name, timestamp)
	createFile(upName, sqlUp)
	createFile(downName, sqlDown)
}

func createFile(fname string, content string) {
	f, err := os.Create(fname)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if _, err = f.WriteString(content); err != nil {
		log.Fatal(err)
	}

}
