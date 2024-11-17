package db

import (
	"database/sql"
	"log"
	"os"
	"path"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/umpc/go-sortedmap"
	"github.com/umpc/go-sortedmap/asc"
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
	_, err = db.Query("SELECT `_seq` FROM `_migration` LIMIT 1;")
	if err != nil {
		// create table
		_, err = db.Exec(`
		CREATE TABLE _migration (
			_seq	INTEGER PRIMARY KEY UNIQUE
		);`)
		// can't create migration table - fatal
		if err != nil {
			return nil, err
		}
	}
	return db, nil
}

func applyMigrations(db *sql.DB) {
	entries, err := os.ReadDir(config.MigrationsPath)
	if err != nil {
		log.Fatal(err)
	}

	// load migrations from directory
	migrations := sortedmap.New(len(entries), asc.Int)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		split := strings.Split(name, "_")
		if len(split) < 2 {
			log.Printf("%s does not follow 000_name.sql convention\n", name)
			continue
		}

		seq, err := strconv.Atoi(split[0])
		if err != nil {
			log.Fatal(err)
		}

		migrations.Insert(name, seq)
	}

	if migrations.Len() == 0 {
		log.Fatal("no migrations!")
	}

	// detect last database migration
	var seq int = -1
	row := db.QueryRow("SELECT `_seq` FROM `_migration` ORDER BY `_seq` DESC LIMIT 1;")
	if row.Err() == nil {
		row.Scan(&seq)
	}

	// apply migrations starting from seq
	iter, err := migrations.BoundedIterCh(false, seq+1, nil)
	if err != nil {
		// no migrations to apply
		return
	}
	defer iter.Close()

	for m := range iter.Records() {
		path := path.Join(config.MigrationsPath, m.Key.(string))
		bytes, err := os.ReadFile(path)
		if err != nil {
			log.Fatal(err)
		}

		sql := string(bytes[:])
		_, err = db.Exec(sql)
		if err != nil {
			log.Fatal(err)
		}

		// successfuly applied migration, so bump seq and insert in table
		seq = m.Val.(int)
		_, err = db.Exec("INSERT INTO `_migration` (_seq) VALUES (?);", seq)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Applied migration %d\n", seq)
	}
}

func Open() (*sql.DB, error) {
	checkConfig()

	db, err := open(config.DBPath)
	if err != nil {
		return nil, err
	}
	applyMigrations(db)

	return db, nil
}
