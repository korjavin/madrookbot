package main

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

// init func opens sqlite database and creates tables if they don't exist
func init() {
	var err error
	db, err = sql.Open("sqlite3", "./data.db")
	if err != nil {
		log.Fatal(err)
	}

	// Create tables if they don't exist
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS activity (username text PRIMARY KEY, groupname text not null, timestamp integer not null)")
	if err != nil {
		log.Fatal(err)
	}
}

// AddActivity updates timestamp of user's activity
func AddActivity(username string, group string, timestamp int64) error {
	_, err := db.Exec("INSERT OR REPLACE INTO activity (username, groupname, timestamp) VALUES (?, ?, ?)", username, group, timestamp)
	return err
}

func GetSilentMoreThan14Days(group string) (string, int64) {
	var username string
	var timestamp int64
	err := db.QueryRow("SELECT username,timestamp  FROM activity WHERE groupname = ? AND timestamp < ?", group, time.Now().Unix()-14*24*60*60).Scan(&username, &timestamp)
	if err != nil {
		return "", 0
	}
	return username, timestamp
}
