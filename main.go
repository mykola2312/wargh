package main

import (
	"log"
	"os"
	"wargh/db"
)

func main() {
	db.Init(&db.DBConfig{
		DBPath:         "wargh.db",
		MigrationsPath: "migrations",
	})

	DB, err := db.Open()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	defer DB.Close()
}
