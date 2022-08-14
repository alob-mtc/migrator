# Migrator

Migrator is a database migration tool, that automates schema migration.

**It scans Schema/Model changes and generate SQL migration scripts automatically to reconcile those changes with the Database.**

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Usage](#usage)
  - [Register Model](#register-model)
  - [Creating Migrations](#creating-migrations)
  - [Running Migrations](#running-migrations)
  - [Rolling Back Migrations](#rolling-back-migrations)
- [Internals](#internals)
  - [schema_migrations table](#schema_migrations-table)
- [Alternatives](#alternatives)
- [Contributing](#contributing)

## Features

- Supports MySQL, PostgreSQL, SQLite, SQL Server.
- Uses plain SQL for writing schema migrations.
- Migrations are timestamp-versioned, to avoid version number conflicts with multiple developers.
- Migrations are run atomically inside a transaction.
- Supports creating and dropping databases (handy in development/test).
- Easily plugable to your existing workflows that uses GORM.

## Installation

To install Gin package, you need to install Go and set your Go workspace first.

1. You first need [Go](https://golang.org/) installed, then you can use the below Go command to install Migrator.

```sh
go get -u github.com/alob-mtc/migrator
```

2. Import it in your code:

```go
import "github.com/alob-mtc/migrator"
```

## Usage

### Register Model

```go
package main

import (
	"fmt"
	migrator "github.com/alob-mtc/migrator/lib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
  "time"
)

type User struct {
	ID        string `gorm:"primaryKey;"`
	Username  string `gorm:"index; not null"`
	Active    *bool  `gorm:"index; not null; default:false"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type Poduct struct {
	ID        string `gorm:"primaryKey;"`
	Name      string `gorm:"index; not null"`
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}


func main() {
	// Connecting to postgres
	dns := "postgres://test:test@localhost:54312/test_db"
	db, err := gorm.Open(postgres.Open(dns), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	// Established database connection

	// Register Model
	newMigrator := migrator.New(db, "migrations/sql/")
	newMigrator.RegisterModel(&User{}, &Poduct{})

}

```

### Creating Migrations

```go
//.....

func main() {

	//.....

	err = newMigrator.Run(db, "create", "add_username_column")
	if err != nil {
		log.Fatal(err)
	}

}

```

### Running Migrations

TODO:

### Rolling Back Migrations

TODO:

## Internals

TODO:

## Contributing

Migrator is written in Go, pull requests are welcome.
