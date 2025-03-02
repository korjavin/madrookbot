package main

import (
	"database/sql"
	"log"

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
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS voices (userid int  PRIMARY KEY, voice text not null)")
	if err != nil {
		log.Fatal(err)
	}

}

func GetAllVoices() (map[int]string, error) {
	result := make(map[int]string)
	rows, err := db.Query("SELECT userid, voice FROM voices")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var userid int
		var voice string
		err = rows.Scan(&userid, &voice)
		if err != nil {
			return nil, err
		}
		result[userid] = voice
	}
	return result, nil
}

func SetVoice(userid int, voice string) error {
	_, err := db.Exec("INSERT OR REPLACE INTO voices (userid, voice) VALUES (?, ?)", userid, voice)
	return err
}
