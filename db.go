package main

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

// init func opens sqlite database
func init() {
	var err error
	dbPath := "./data/my.db" // Store in data directory for Docker volume mount
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}
}
