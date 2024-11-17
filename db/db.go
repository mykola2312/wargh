package db

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

type DBConfig struct {
	DBPath         string
	MigrationsPath string
}

var config *DBConfig = nil

func Init(_config *DBConfig) {
	config = _config
}

func checkConfig() {
	if config == nil {
		log.Fatal("DBConfig not initialized! Call db.Init!")
		os.Exit(1)
	}
}

func open(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	/* if database is empty. we must create
	'_migration` table and apply all releveant migrations
	*/
	_, err = db.Query("SELECT id FROM `_migration` LIMIT 1;")
	if err != nil {
		// create table
		_, err = db.Exec(`
		CREATE TABLE _migration (
			id		INTEGER PRIMARY KEY AUTOINCREMENT,
			seq		INTEGER	NOT NULL UNIQUE,
			name	TEXT	NOT NULL UNIQUE
		);`)
		// can't create migration table - fatal
		if err != nil {
			return nil, err
		}
	}
	return db, nil
}

func Open() (*sql.DB, error) {
	checkConfig()

	db, err := open(config.DBPath)
	if err != nil {
		return nil, err
	}

	return db, nil
}
