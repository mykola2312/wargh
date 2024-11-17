package main

import (
	"log"
	"os"
	"wargh/db"
)

const DB_PATH = "wargh.db"

func main() {
	db.Init(&db.DBConfig{
		DBPath:         DB_PATH,
		MigrationsPath: "",
	})

	DB, err := db.Open()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	defer DB.Close()

	_, err = DB.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY AUTOINCREMENT, value TEXT NOT NULL);")
	log.Fatal(err)
}
