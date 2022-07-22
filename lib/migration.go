package migrator

import (
	"flag"
	"fmt"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"gorm.io/gorm"
	"log"
	"os"
	"strconv"
	"time"
)

const (
	DefaultMigrationsFolder = "migrations/sql/"
)

func (mig *Migrator) Run(db *gorm.DB, args []string, migrationFolder ...string) error {
	var err error

	migrationPath := DefaultMigrationsFolder
	if len(migrationFolder) > 0 {
		migrationPath = migrationFolder[0]
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("error getting sql.DB representation:", err)
	}

	driver, err := postgres.WithInstance(sqlDB, &postgres.Config{})
	if err != nil {
		log.Fatal("error instantiating postgres instance:", err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://"+migrationPath, "postgres", driver)
	if err != nil {
		log.Fatal("error instantiating migration instance:", err)
	}

	startTime := time.Now()

	if len(args) == 0 {
		log.Fatal("You must pass a least one args")
	}

	switch args[0] {
	case "up":
		err = m.Up()
	case "down":
		err = m.Steps(-1)
	case "clear":
		err = m.Down()
	case "force":
		version, _ := strconv.Atoi(args[1])
		err = m.Force(version)
	case "create":
		args := args[1:]
		createFlagSet := flag.NewFlagSet("create", flag.ExitOnError)
		_ = createFlagSet.Parse(args)

		if createFlagSet.NArg() == 0 {
			log.Fatal("Specify a name for the migration")
		}

		//generate migration
		sqlUp, sqlDown, err := mig.AutoMigrate()
		if err != nil {
			log.Fatal(err)
		}
		if sqlUp == "" && sqlDown == "" {
			return nil
		}
		createCmd(migrationPath, startTime.Unix(), createFlagSet.Arg(0), sqlUp, sqlDown)
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

func createCmd(migrationPath string, timestamp int64, name string, sqlUp string, sqlDown string) {
	base := fmt.Sprintf("%v%v_%v.", migrationPath, timestamp, name)
	_ = os.MkdirAll(migrationPath, os.ModePerm)
	createFile(base+"up.sql", sqlUp)
	createFile(base+"down.sql", sqlDown)
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
