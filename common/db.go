package common

import (
	"database/sql"
	"log"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

var (
	database *sql.DB
	once     sync.Once
)

// InitDB initializes the database connection
func InitDB(dataSourceName string) {
	once.Do(func() {
		var err error
		database, err = sql.Open("sqlite3", dataSourceName)
		if err != nil {
			log.Fatal(err)
		}
		_, err = database.Exec("PRAGMA foreign_keys = ON")
		if err != nil {
			log.Fatalf("Error enabling foreign keys: %v", err)
		}
		createTables()
	})
}

func GetDB() *sql.DB {
	return database
}

// CloseDB closes the database connection
func CloseDB() {
	if database != nil {
		err := database.Close()
		if err != nil {
			return
		}
	}
}

func createTables() {
	createTorrentTable := `
	CREATE TABLE IF NOT EXISTS torrent (
		id TEXT PRIMARY KEY,
		info_hash TEXT,
		name TEXT,
		folder TEXT,
		filename TEXT,
		size INTEGER,
		magnet TEXT,
		status TEXT,
		error TEXT,
	    watch_folder TEXT
	);`

	createFileTable := `
	CREATE TABLE IF NOT EXISTS file (
		id TEXT PRIMARY KEY,
		name TEXT,
		size INTEGER,
		path TEXT,
		torrent_id TEXT,
		FOREIGN KEY(torrent_id) REFERENCES torrent(id)
	);`

	_, err := database.Exec(createTorrentTable)
	if err != nil {
		log.Fatalf("Error creating torrent table: %v", err)
	}

	_, err = database.Exec(createFileTable)
	if err != nil {
		log.Fatalf("Error creating file table: %v", err)
	}
}
