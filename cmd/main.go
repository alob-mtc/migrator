package main

import (
	"fmt"
	"github.com/alobmtc/migrator/database"
)

func main() {
	fmt.Println("Connecting to postgres")
	dbConnection := database.ConnectDB("postgres://test:test@localhost:5431/test_db")
	if err := database.MigrateAll(dbConnection); err != nil {
		fmt.Println(err)
	}

}
